package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"
)

// chat ID → URL → sent at
type State map[int]map[string]time.Time

func saveState(path string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state %s: %w", path, err)
	}

	err = os.WriteFile(path, data, 0600)
	if err != nil {
		return fmt.Errorf("saving state %s: %w", path, err)
	}

	return nil
}

func loadState(path string) (State, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		// Missing file means an empty state
		if errors.Is(err, fs.ErrNotExist) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("reading state %s: %w", path, err)
	}

	var state State
	err = json.Unmarshal(doc, &state)
	if err != nil {
		return State{}, fmt.Errorf("parsing state %s: %w", path, err)
	}

	return state, nil
}

const retention = 30 * 24 * time.Hour

func pruneState(state State) {
	cutoff := time.Now().Add(-retention)

	for chatID, sent := range state {
		for link, sentAt := range sent {
			if sentAt.Before(cutoff) {
				delete(sent, link)
			}
		}

		if len(sent) == 0 {
			delete(state, chatID)
			slog.Info("deleted state for recipient", "recipient", chatID)
		}
	}
}
