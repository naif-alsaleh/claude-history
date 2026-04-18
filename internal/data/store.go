package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening duckdb: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			uuid        TEXT PRIMARY KEY,
			name        TEXT NOT NULL DEFAULT '',
			summary     TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMP NOT NULL,
			updated_at  TIMESTAMP NOT NULL,
			is_research BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE IF NOT EXISTS messages (
			uuid            TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES conversations(uuid),
			sender          TEXT NOT NULL,
			text            TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
	`)
	return err
}

func (s *Store) ConversationExists(ctx context.Context, uuid string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) > 0 FROM conversations WHERE uuid = ?", uuid).Scan(&exists)
	return exists, err
}

func (s *Store) InsertConversation(ctx context.Context, c Conversation) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations (uuid, name, summary, created_at, updated_at, is_research)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (uuid) DO UPDATE SET
			name = EXCLUDED.name,
			summary = EXCLUDED.summary,
			updated_at = EXCLUDED.updated_at,
			is_research = EXCLUDED.is_research`,
		c.UUID, c.Name, c.Summary, c.CreatedAt, c.UpdatedAt, c.IsResearch,
	)
	return err
}

func (s *Store) InsertMessage(ctx context.Context, m Message) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (uuid, conversation_id, sender, text, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (uuid) DO NOTHING`,
		m.UUID, m.ConversationID, m.Sender, m.Text, m.CreatedAt,
	)
	return err
}

func (s *Store) GetConversationUpdatedAt(ctx context.Context, uuid string) (time.Time, bool, error) {
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, "SELECT updated_at FROM conversations WHERE uuid = ?", uuid).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return updatedAt, true, nil
}

func (s *Store) DeleteMessages(ctx context.Context, conversationID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM messages WHERE conversation_id = ?", conversationID)
	return err
}

func (s *Store) ConversationCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM conversations").Scan(&count)
	return count, err
}

func (s *Store) AllConversationsWithMessages(ctx context.Context) ([]ConversationWithMessages, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.uuid, c.name, c.summary, c.created_at, c.updated_at, c.is_research,
		       m.uuid, m.sender, m.text, m.created_at
		FROM conversations c
		JOIN messages m ON m.conversation_id = c.uuid
		ORDER BY c.updated_at DESC, m.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	convMap := make(map[string]*ConversationWithMessages)
	var order []string

	for rows.Next() {
		var (
			conv Conversation
			msg  Message
			cAt  time.Time
			uAt  time.Time
			mAt  time.Time
		)
		if err := rows.Scan(
			&conv.UUID, &conv.Name, &conv.Summary, &cAt, &uAt, &conv.IsResearch,
			&msg.UUID, &msg.Sender, &msg.Text, &mAt,
		); err != nil {
			return nil, err
		}
		conv.CreatedAt = cAt
		conv.UpdatedAt = uAt
		msg.CreatedAt = mAt

		if _, ok := convMap[conv.UUID]; !ok {
			convMap[conv.UUID] = &ConversationWithMessages{Conversation: conv}
			order = append(order, conv.UUID)
		}
		convMap[conv.UUID].Messages = append(convMap[conv.UUID].Messages, msg)
	}

	result := make([]ConversationWithMessages, 0, len(order))
	for _, id := range order {
		result = append(result, *convMap[id])
	}
	return result, rows.Err()
}

type ConversationWithMessages struct {
	Conversation Conversation
	Messages     []Message
}
