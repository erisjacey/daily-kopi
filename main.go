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
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

func main() {
	// Load env & env variables
	godotenv.Load()
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("telegram bot token not set")
	}

	// Load config
	config, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	// Loop over recipients
	for _, recipient := range config.Recipients {
		if err := sendDigest(botToken, recipient, config.Topics, config.MaxItemsPerTopic); err != nil {
			slog.Error("send digest failed", "recipient", recipient.ChatID, "err", err)
			continue
		}
		slog.Info("send digest succeeded", "recipient", recipient.ChatID)
	}
}

func sendDigest(botToken string, recipient Recipient, topics []Topic, maxItemsPerTopic int) error {
	var filteredTopics []Topic

	// Find matching topics
	for _, topic := range topics {
		if slices.Contains(recipient.Topics, topic.Name) {
			filteredTopics = append(filteredTopics, topic)
		}
	}

	fp := gofeed.NewParser()
	var sb strings.Builder
	for _, topic := range filteredTopics {
		var topicItems []*gofeed.Item

		for _, feed := range topic.Feeds {
			parsedFeed, err := fp.ParseURL(feed.URL)
			if err != nil {
				slog.Warn("feed parse failed", "url", feed.URL, "err", err)
				continue
			}
			topicItems = append(topicItems, parsedFeed.Items...)
		}

		capped := topicItems[:min(maxItemsPerTopic, len(topicItems))]
		fmt.Fprintln(&sb, formatItems(capped, topic.Name))
	}

	return sendMessage(botToken, strconv.Itoa(recipient.ChatID), sb.String())
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
