package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

func main() {
	godotenv.Load()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" {
		log.Fatal("telegram bot token not set")
	}

	if telegramChatID == "" {
		log.Fatal("telegram chat ID not set")
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL("http://feeds.twit.tv/twit.xml")
	if err != nil {
		log.Fatal(err)
	}

	firstFive := feed.Items[:min(5, len(feed.Items))]

	if err := sendMessage(botToken, telegramChatID, formatItems(firstFive)); err != nil {
		log.Fatal(err)
	}
}

func formatItems(items []*gofeed.Item) string {
	var sb strings.Builder

	for _, item := range items {
		fmt.Fprintln(&sb, item.Title, item.Link)
	}

	return sb.String()
}

func sendMessage(botToken string, chatID string, text string) error {
	values := url.Values{}

	values.Set("chat_id", chatID)
	values.Set("text", text)
	values.Set("link_preview_options", `{"is_disabled": true}`)

	resp, err := http.PostForm(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken), values)

	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			return fmt.Errorf("telegram: request failed: %w", urlErr.Err)
		}
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}

	return nil
}
