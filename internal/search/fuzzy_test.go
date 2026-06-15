package search

import (
	"strings"
	"testing"

	"github.com/naif/claude-history/internal/data"
)

func score(text, query string, mode MatchMode) float64 {
	query = strings.ToLower(query)
	tokens := strings.Fields(query)
	return scoreText(strings.ToLower(text), strings.Join(tokens, " "), tokens, mode)
}

func TestScoreText_RequiresAllTokens(t *testing.T) {
	if s := score("a state machine design doc", "machine learning", MatchSubstring); s != 0 {
		t.Fatalf("expected 0 when a token is missing, got %v", s)
	}
	if s := score("the machine ran the learning loop", "machine learning", MatchSubstring); s <= 0 {
		t.Fatalf("expected match when all tokens present, got %v", s)
	}
}

func TestScoreText_PhraseRanksHigher(t *testing.T) {
	phrase := score("fine-tuning a machine learning model", "machine learning", MatchSubstring)
	scattered := score("the machine ran the learning loop", "machine learning", MatchSubstring)
	if phrase <= scattered {
		t.Fatalf("expected exact phrase (%v) to outscore scattered tokens (%v)", phrase, scattered)
	}
}

func TestScoreText_SingleToken(t *testing.T) {
	if s := score("a machine here", "machine", MatchSubstring); s != 1.0 {
		t.Fatalf("expected single exact token to score 1.0, got %v", s)
	}
	if s := score("nothing relevant", "machine", MatchSubstring); s != 0 {
		t.Fatalf("expected 0 for absent single token, got %v", s)
	}
}

func TestScoreBest_ExcludesPartialMatch(t *testing.T) {
	cw := data.ConversationWithMessages{
		Conversation: data.Conversation{Name: "state machine notes"},
		Messages:     []data.Message{{Text: "discussing the machine in detail"}},
	}
	tokens := []string{"machine", "learning"}
	r := scoreBest(cw, "machine learning", tokens, MatchSubstring)
	if r.Score != 0 {
		t.Fatalf("expected no match when 'learning' is absent, got score %v", r.Score)
	}
}

func TestScoreBest_PhraseSnippet(t *testing.T) {
	cw := data.ConversationWithMessages{
		Conversation: data.Conversation{Name: "notes"},
		Messages:     []data.Message{{Text: "intro then a machine learning model appears later"}},
	}
	r := scoreBest(cw, "machine learning", []string{"machine", "learning"}, MatchSubstring)
	if r.Score <= 0 {
		t.Fatalf("expected a match, got %v", r.Score)
	}
	if !strings.Contains(strings.ToLower(r.Snippet), "machine learning") {
		t.Fatalf("expected snippet to contain the phrase, got %q", r.Snippet)
	}
}
