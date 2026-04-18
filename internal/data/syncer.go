package data

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type SyncOptions struct {
	UpdateExisting bool
}

type SyncStats struct {
	Listed  int
	New     int
	Updated int
	Skipped int
	Errors  int
}

func Sync(ctx context.Context, store *Store, client *Client, opts SyncOptions, logf func(string, ...any)) (SyncStats, error) {
	logf("Fetching organization...")
	orgID, err := client.GetOrganizationID(ctx)
	if err != nil {
		return SyncStats{}, err
	}

	logf("Listing conversations...")
	convList, err := client.ListConversations(ctx, orgID)
	if err != nil {
		return SyncStats{}, err
	}

	stats := SyncStats{Listed: len(convList)}
	logf("Found %d conversations", len(convList))

	for i, item := range convList {
		if err := ctx.Err(); err != nil {
			return stats, err
		}

		dbUpdatedAt, exists, err := store.GetConversationUpdatedAt(ctx, item.UUID)
		if err != nil {
			return stats, fmt.Errorf("checking conversation %s: %w", item.UUID, err)
		}

		if exists {
			if !opts.UpdateExisting {
				stats.Skipped++
				continue
			}
			if dbUpdatedAt.Equal(item.UpdatedAt) {
				stats.Skipped++
				continue
			}
		}

		time.Sleep(500 * time.Millisecond)

		full, err := client.GetConversation(ctx, orgID, item.UUID)
		if err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				if apiErr.IsAuth() {
					return stats, fmt.Errorf("session key expired or invalid: %w", err)
				}
				if apiErr.IsRateLimit() {
					logf("  Rate limited, backing off 5s...")
					time.Sleep(5 * time.Second)
					full, err = client.GetConversation(ctx, orgID, item.UUID)
					if err != nil {
						logf("  [%d/%d] SKIP %s: %v", i+1, stats.Listed, item.Name, err)
						stats.Errors++
						continue
					}
				} else {
					logf("  [%d/%d] SKIP %s: %v", i+1, stats.Listed, item.Name, err)
					stats.Errors++
					continue
				}
			} else {
				logf("  [%d/%d] SKIP %s: %v", i+1, stats.Listed, item.Name, err)
				stats.Errors++
				continue
			}
		}

		conv := Conversation{
			UUID:       full.UUID,
			Name:       full.Name,
			Summary:    full.Summary,
			CreatedAt:  full.CreatedAt,
			UpdatedAt:  full.UpdatedAt,
			IsResearch: isResearch(full.ChatMessages),
		}

		if exists {
			if err := store.DeleteMessages(ctx, full.UUID); err != nil {
				return stats, fmt.Errorf("deleting old messages for %s: %w", full.UUID, err)
			}
		}

		if err := store.InsertConversation(ctx, conv); err != nil {
			return stats, fmt.Errorf("inserting conversation %s: %w", full.UUID, err)
		}

		for _, rm := range full.ChatMessages {
			msg := Message{
				UUID:           rm.UUID,
				ConversationID: full.UUID,
				Sender:         rm.Sender,
				Text:           rm.Text,
				CreatedAt:      rm.CreatedAt,
			}
			if err := store.InsertMessage(ctx, msg); err != nil {
				return stats, fmt.Errorf("inserting message %s: %w", rm.UUID, err)
			}
		}

		if exists {
			stats.Updated++
			logf("  [%d/%d] Updated: %s", i+1, stats.Listed, item.Name)
		} else {
			stats.New++
			logf("  [%d/%d] New: %s", i+1, stats.Listed, item.Name)
		}
	}

	return stats, nil
}
