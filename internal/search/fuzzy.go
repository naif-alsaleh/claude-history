package search

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/naif/claude-history/internal/data"
)

type FuzzySearcher struct {
	store *data.Store
	convs []data.ConversationWithMessages
}

func NewFuzzySearcher(store *data.Store) *FuzzySearcher {
	return &FuzzySearcher{store: store}
}

func (f *FuzzySearcher) Index(ctx context.Context) error {
	convs, err := f.store.AllConversationsWithMessages(ctx)
	if err != nil {
		return err
	}
	f.convs = convs
	return nil
}

func (f *FuzzySearcher) Search(_ context.Context, query string, maxResults int) ([]data.SearchResult, error) {
	query = strings.ToLower(query)
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return nil, nil
	}

	var results []data.SearchResult

	for _, cw := range f.convs {
		best := scoreBest(cw, tokens)
		if best.Score > 0 {
			results = append(results, best)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}
	return results, nil
}

type match struct {
	score   float64
	snippet string
	source  string // "title", "summary", "message"
}

func scoreBest(cw data.ConversationWithMessages, tokens []string) data.SearchResult {
	var best match

	titleLower := strings.ToLower(cw.Conversation.Name)
	if s := scoreText(titleLower, tokens); s > 0 {
		m := match{score: s * 3.0, snippet: cw.Conversation.Name, source: "title"}
		if m.score > best.score {
			best = m
		}
	}

	summaryLower := strings.ToLower(cw.Conversation.Summary)
	if s := scoreText(summaryLower, tokens); s > 0 {
		m := match{score: s * 1.5, snippet: extractSnippet(cw.Conversation.Summary, tokens, 150), source: "summary"}
		if m.score > best.score {
			best = m
		}
	}

	for _, msg := range cw.Messages {
		textLower := strings.ToLower(msg.Text)
		if s := scoreText(textLower, tokens); s > 0 {
			m := match{score: s, snippet: extractSnippet(msg.Text, tokens, 150), source: "message"}
			if m.score > best.score {
				best = m
			}
		}
	}

	if best.score == 0 {
		return data.SearchResult{}
	}

	if cw.Conversation.IsResearch {
		best.score *= 1.1
	}

	// When the best match is the title, try to find a content snippet to show instead.
	if best.source == "title" {
		if snippet := findContentSnippet(cw, tokens); snippet != "" {
			best.snippet = snippet
		}
	}

	return data.SearchResult{
		Conversation:  cw.Conversation,
		Snippet:       best.snippet,
		MatchedTokens: tokens,
		Score:         best.score,
	}
}

func findContentSnippet(cw data.ConversationWithMessages, tokens []string) string {
	// Prefer summary
	if hasSubstringMatch(strings.ToLower(cw.Conversation.Summary), tokens) {
		return extractSnippet(cw.Conversation.Summary, tokens, 150)
	}
	for _, msg := range cw.Messages {
		if hasSubstringMatch(strings.ToLower(msg.Text), tokens) {
			return extractSnippet(msg.Text, tokens, 150)
		}
	}
	return ""
}

func hasSubstringMatch(text string, tokens []string) bool {
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			return true
		}
	}
	return false
}

// scoreText returns a score for how well text matches the tokens.
// Exact substring matches score much higher than fuzzy character-sequence matches.
func scoreText(text string, tokens []string) float64 {
	if text == "" {
		return 0
	}
	var total float64
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			total += 1.0
		} else if fuzzyMatch(text, tok) {
			total += 0.2
		}
	}
	if total == 0 {
		return 0
	}
	return total / float64(len(tokens))
}

// fuzzyMatch checks if all characters of pattern appear in text in order.
func fuzzyMatch(text, pattern string) bool {
	if len(pattern) < 3 {
		return false
	}
	ti := 0
	for pi := 0; pi < len(pattern); {
		if ti >= len(text) {
			return false
		}
		pr, ps := utf8.DecodeRuneInString(pattern[pi:])
		tr, ts := utf8.DecodeRuneInString(text[ti:])
		if pr == tr {
			pi += ps
		}
		ti += ts
	}
	return true
}

func extractSnippet(text string, tokens []string, maxLen int) string {
	lower := strings.ToLower(text)
	bestIdx := -1
	for _, tok := range tokens {
		idx := strings.Index(lower, strings.ToLower(tok))
		if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
		}
	}
	if bestIdx < 0 {
		bestIdx = 0
	}

	start := bestIdx - maxLen/3
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.Join(strings.Fields(snippet), " ")

	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(text) {
		suffix = "..."
	}
	return prefix + snippet + suffix
}
