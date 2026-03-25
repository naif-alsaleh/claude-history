package data

import "time"

type Conversation struct {
	UUID       string
	Name       string
	Summary    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	IsResearch bool
}

type Message struct {
	UUID           string
	ConversationID string
	Sender         string
	Text           string
	CreatedAt      time.Time
}

type SearchResult struct {
	Conversation  Conversation
	Snippet       string
	MatchedTokens []string
	Score         float64
}
