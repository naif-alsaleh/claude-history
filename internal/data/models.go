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

// DateLabel renders the start date, plus the last-updated date when it differs.
func (c Conversation) DateLabel() string {
	created := c.CreatedAt.Format(time.DateOnly)
	updated := c.UpdatedAt.Format(time.DateOnly)
	if created == updated {
		return created
	}
	return "created " + created + " · updated " + updated
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
