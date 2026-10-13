package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

func main() {
	godotenv.Load()
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("telegram bot token not set")
	}

	config, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	articles := fetchArticles(config.Topics)

	for _, recipient := range config.Recipients {
		digest := buildDigest(articles, recipient, config.MaxItemsPerTopic)
		if err := sendMessage(botToken, strconv.Itoa(recipient.ChatID), digest); err != nil {
			slog.Error("send digest failed", "recipient", recipient.ChatID, "err", err)
			continue
		}
		slog.Info("send digest succeeded", "recipient", recipient.ChatID)
	}
}

// Fetch articles, skipping feeds that fail. If all feeds for a topic fail,
// it has no corresponding entry in the map.
func fetchArticles(topics []Topic) map[string][]*gofeed.Item {
	articles := make(map[string][]*gofeed.Item)
	fp := gofeed.NewParser()

	for _, topic := range topics {
		for _, feed := range topic.Feeds {
			parsedFeed, err := fp.ParseURL(feed.URL)
			if err != nil {
				slog.Warn("feed parse failed", "url", feed.URL, "err", err)
				continue
			}

			articles[topic.Name] = append(articles[topic.Name], filterRecentItems(parsedFeed.Items)...)
		}
	}

	return articles
}

func filterRecentItems(items []*gofeed.Item) []*gofeed.Item {
	var filtered []*gofeed.Item
	for _, item := range items {
		// nil PublishedParsed is considered "probably recent"
		if item.PublishedParsed == nil || time.Since(*item.PublishedParsed).Abs() < 24*time.Hour {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func buildDigest(articles map[string][]*gofeed.Item, recipient Recipient, maxItemsPerTopic int) string {
	var sb strings.Builder

	// Iterate the recipient's list, not the map, to keep their topic order
	for _, topic := range recipient.Topics {
		items := articles[topic]
		if len(items) == 0 {
			slog.Warn("no items for topic", "topic", topic)
			continue
		}

		capped := items[:min(maxItemsPerTopic, len(items))]
		fmt.Fprintln(&sb, formatItems(capped, topic))
	}

	return sb.String()
}

func formatItems(items []*gofeed.Item, topicName string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "<b>%s</b>\n", html.EscapeString(topicName))
	for _, item := range items {
		fmt.Fprintf(&sb, "<a href=\"%s\">%s</a>\n", html.EscapeString(item.Link), html.EscapeString(item.Title))
	}

	return sb.String()
}

func sendMessage(botToken string, chatID string, text string) error {
	type telegramResponse struct {
		Description string `json:"description"`
	}

	values := url.Values{}

	values.Set("chat_id", chatID)
	values.Set("text", text)
	values.Set("link_preview_options", `{"is_disabled": true}`)
	values.Set("parse_mode", "HTML")

	resp, err := http.PostForm(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken), values)

	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			return fmt.Errorf("telegram: request failed: %w", urlErr.Err)
		}
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		var telegramResp telegramResponse
		json.Unmarshal(body, &telegramResp)
		return fmt.Errorf("telegram: unexpected status %d: %s", resp.StatusCode, telegramResp.Description)
	}

	return nil
}
