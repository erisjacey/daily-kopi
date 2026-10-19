package main

import (
	"errors"
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

// validate reports every problem in the config, joined.
func (c Config) validate() error {
	var allErrors []error

	topicNames := make(map[string]struct{})
	for _, topic := range c.Topics {
		topicNames[topic.Name] = struct{}{}
	}

	// Check if recipient topics exist
	for _, recipient := range c.Recipients {
		for _, name := range recipient.Topics {
			if _, exists := topicNames[name]; !exists {
				err := fmt.Errorf("recipient %d: unknown topic %q", recipient.ChatID, name)
				allErrors = append(allErrors, err)
			}
		}
	}

	// Check for invalid max_items_per_topic
	if c.MaxItemsPerTopic < 0 {
		err := fmt.Errorf("max_items_per_topic must be positive, got %d", c.MaxItemsPerTopic)
		allErrors = append(allErrors, err)
	}

	return errors.Join(allErrors...)
}

type Topic struct {
	Name        string `toml:"name"`
	DisplayName string `toml:"display_name"`
	Feeds       []Feed `toml:"feeds"`
}

// label returns DisplayName, or Name if it's unset.
func (t Topic) label() string {
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

	// Set config defaults if unset
	if cfg.StateFile == "" {
		cfg.StateFile = "state.json"
	}
	if cfg.MaxItemsPerTopic == 0 {
		cfg.MaxItemsPerTopic = 5
	}

	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}

	return cfg, nil
}
