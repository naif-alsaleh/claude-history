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

func scoreBest(cw data.ConversationWithMessages, tokens []string) data.SearchResult {
	var bestScore float64
	var bestSnippet string

	// Score title match (weighted higher)
	titleLower := strings.ToLower(cw.Conversation.Name)
	titleScore := scoreText(titleLower, tokens) * 2.0
	if titleScore > bestScore {
		bestScore = titleScore
		bestSnippet = cw.Conversation.Name
	}

	// Score summary
	summaryLower := strings.ToLower(cw.Conversation.Summary)
	summaryScore := scoreText(summaryLower, tokens) * 1.5
	if summaryScore > bestScore {
		bestScore = summaryScore
		bestSnippet = extractSnippet(cw.Conversation.Summary, tokens, 150)
	}

	// Score messages
	for _, m := range cw.Messages {
		textLower := strings.ToLower(m.Text)
		msgScore := scoreText(textLower, tokens)
		if msgScore > bestScore {
			bestScore = msgScore
			bestSnippet = extractSnippet(m.Text, tokens, 150)
		}
	}

	// Research bonus
	if cw.Conversation.IsResearch {
		bestScore *= 1.1
	}

	return data.SearchResult{
		Conversation: cw.Conversation,
		Snippet:      bestSnippet,
		Score:        bestScore,
	}
}

func scoreText(text string, tokens []string) float64 {
	if text == "" {
		return 0
	}
	var matched int
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			matched++
		} else if fuzzyMatch(text, tok) {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	return float64(matched) / float64(len(tokens))
}

// fuzzyMatch checks if all characters of pattern appear in text in order.
func fuzzyMatch(text, pattern string) bool {
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
