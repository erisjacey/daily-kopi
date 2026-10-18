package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	StateFile        string      `toml:"state_file"`
	MaxItemsPerTopic int         `toml:"max_items_per_topic"`
	Topics           []Topic     `toml:"topics"`
	Recipients       []Recipient `toml:"recipients"`
}

type Topic struct {
	Name        string `toml:"name"`
	DisplayName string `toml:"display_name"`
	Feeds       []Feed `toml:"feeds"`
}

// Label returns DisplayName, or Name if it's unset.
func (t Topic) Label() string {
	if t.DisplayName == "" {
		return t.Name
	}
	return t.DisplayName
}

type Feed struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type Recipient struct {
	ChatID int      `toml:"chat_id"`
	Topics []string `toml:"topics"`
}

func loadConfig(path string) (Config, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	err = toml.Unmarshal(doc, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
