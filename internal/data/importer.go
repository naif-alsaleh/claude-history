package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type rawConversation struct {
	UUID         string       `json:"uuid"`
	Name         string       `json:"name"`
	Summary      string       `json:"summary"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	ChatMessages []rawMessage `json:"chat_messages"`
}

type rawMessage struct {
	UUID      string       `json:"uuid"`
	Text      string       `json:"text"`
	Sender    string       `json:"sender"`
	CreatedAt time.Time    `json:"created_at"`
	Content   []rawContent `json:"content"`
}

type rawContent struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

var researchTools = map[string]bool{
	"web_search":                   true,
	"web_fetch":                    true,
	"launch_extended_search_task":  true,
}

func isResearch(msgs []rawMessage) bool {
	for _, m := range msgs {
		for _, c := range m.Content {
			if c.Type == "tool_use" && researchTools[c.Name] {
				return true
			}
		}
	}
	return false
}

type ImportStats struct {
	Total   int
	New     int
	Skipped int
}

func Import(ctx context.Context, store *Store, jsonPath string) (ImportStats, error) {
	f, err := os.Open(jsonPath)
	if err != nil {
		return ImportStats{}, fmt.Errorf("opening export file: %w", err)
	}
	defer f.Close()

	var convs []rawConversation
	if err := json.NewDecoder(f).Decode(&convs); err != nil {
		return ImportStats{}, fmt.Errorf("decoding JSON: %w", err)
	}

	stats := ImportStats{Total: len(convs)}

	for _, raw := range convs {
		exists, err := store.ConversationExists(ctx, raw.UUID)
		if err != nil {
			return stats, err
		}
		if exists {
			stats.Skipped++
			continue
		}

		conv := Conversation{
			UUID:       raw.UUID,
			Name:       raw.Name,
			Summary:    raw.Summary,
			CreatedAt:  raw.CreatedAt,
			UpdatedAt:  raw.UpdatedAt,
			IsResearch: isResearch(raw.ChatMessages),
		}
		if err := store.InsertConversation(ctx, conv); err != nil {
			return stats, fmt.Errorf("inserting conversation %s: %w", raw.UUID, err)
		}

		for _, rm := range raw.ChatMessages {
			msg := Message{
				UUID:           rm.UUID,
				ConversationID: raw.UUID,
				Sender:         rm.Sender,
				Text:           rm.Text,
				CreatedAt:      rm.CreatedAt,
			}
			if err := store.InsertMessage(ctx, msg); err != nil {
				return stats, fmt.Errorf("inserting message %s: %w", rm.UUID, err)
			}
		}
		stats.New++
	}

	return stats, nil
}
