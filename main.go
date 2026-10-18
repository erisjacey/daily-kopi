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

type Article struct {
	Title  string
	Link   string
	Source string
}

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

	state, err := loadState(config.StateFile)
	if err != nil {
		log.Fatalf("loading state: %v", err)
	}

	for _, recipient := range config.Recipients {
		sent := state[recipient.ChatID]
		if sent == nil {
			sent = make(map[string]time.Time)
		}

		digest, included := buildDigest(articles, recipient, config.MaxItemsPerTopic, sent)
		if digest == "" {
			slog.Info("empty message, skip send", "recipient", recipient.ChatID)
			continue
		}

		if err := sendMessage(botToken, strconv.Itoa(recipient.ChatID), digest); err != nil {
			slog.Error("send digest failed", "recipient", recipient.ChatID, "err", err)
			continue
		}

		slog.Info("send digest succeeded", "recipient", recipient.ChatID)

		// Only mark after successful send, so failed sends are retried next run
		now := time.Now()
		for link := range included {
			sent[link] = now
		}

		state[recipient.ChatID] = sent
	}

	pruneState(state)
	err = saveState(config.StateFile, state)
	if err != nil {
		log.Fatalf("saving state: %v", err)
	}
}

// fetchArticles returns recent items keyed by topic, skipping feeds that fail.
// Items are interleaved round-robin across each topic's feeds.
func fetchArticles(topics []Topic) map[string][]Article {
	articles := make(map[string][]Article)
	fp := gofeed.NewParser()

	for _, topic := range topics {
		var feedItems [][]Article

		for _, feed := range topic.Feeds {
			parsedFeed, err := fp.ParseURL(feed.URL)
			if err != nil {
				slog.Warn("feed parse failed", "url", feed.URL, "err", err)
				continue
			}

			feedItems = append(feedItems, toArticles(filterRecentItems(parsedFeed.Items), feed.Name))
		}

		articles[topic.Name] = interleave(feedItems)
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

func toArticles(items []*gofeed.Item, feedName string) []Article {
	articles := make([]Article, 0, len(items))
	for _, item := range items {
		article := Article{Title: item.Title, Link: item.Link, Source: feedName}
		articles = append(articles, article)
	}
	return articles
}

func interleave(feeds [][]Article) []Article {
	var items []Article

	longestLength := 0
	for _, feed := range feeds {
		longestLength = max(longestLength, len(feed))
	}

	for i := range longestLength {
		for _, feed := range feeds {
			if i < len(feed) {
				items = append(items, feed[i])
			}
		}
	}

	return items
}

// buildDigest returns the digest text and the URLs it included.
func buildDigest(articles map[string][]Article, recipient Recipient, maxItemsPerTopic int,
	sent map[string]time.Time) (string, map[string]struct{}) {
	var sb strings.Builder
	included := make(map[string]struct{})

	// Iterate the recipient's list, not the map, to keep their topic order
	for _, topic := range recipient.Topics {
		items := articles[topic]
		var uniqueItems []Article

		for _, item := range items {
			if len(uniqueItems) >= maxItemsPerTopic {
				break
			}

			if _, exists := sent[item.Link]; exists {
				continue
			}

			if _, exists := included[item.Link]; exists {
				continue
			}

			uniqueItems = append(uniqueItems, item)

			// Only mark once item makes the cut, so items dropped by cap can still appear under a later topic
			included[item.Link] = struct{}{}
		}

		if len(uniqueItems) == 0 {
			slog.Warn("no items for topic", "topic", topic)
			continue
		}

		fmt.Fprintln(&sb, formatItems(uniqueItems, topic))
	}

	return sb.String(), included
}

func formatItems(items []Article, topicName string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "<b>%s</b>\n", html.EscapeString(topicName))
	for _, item := range items {
		fmt.Fprintf(&sb, "• <a href=\"%s\">%s</a> <i>— %s</i>\n", html.EscapeString(item.Link),
			html.EscapeString(item.Title), html.EscapeString(item.Source))
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
