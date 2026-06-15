package search

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/naif/claude-history/internal/data"
)

const (
	fuzzyScoreThreshold = 0.3
	phraseBoost         = 1.0
)

type FuzzySearcher struct {
	store        *data.Store
	convs        []data.ConversationWithMessages
	researchOnly bool
	orderMode    OrderMode
	matchMode    MatchMode
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

func (f *FuzzySearcher) SetResearchOnly(v bool) { f.researchOnly = v }
func (f *FuzzySearcher) ResearchOnly() bool      { return f.researchOnly }
func (f *FuzzySearcher) SetOrderMode(m OrderMode) { f.orderMode = m }
func (f *FuzzySearcher) OrderMode() OrderMode     { return f.orderMode }
func (f *FuzzySearcher) SetMatchMode(m MatchMode)  { f.matchMode = m }
func (f *FuzzySearcher) MatchMode() MatchMode      { return f.matchMode }

func (f *FuzzySearcher) Search(_ context.Context, query string, maxResults int) ([]data.SearchResult, error) {
	query = strings.ToLower(query)
	tokens := strings.Fields(query)
	phrase := strings.Join(tokens, " ")

	var results []data.SearchResult

	if len(tokens) == 0 {
		for _, cw := range f.convs {
			if f.researchOnly && !cw.Conversation.IsResearch {
				continue
			}
			results = append(results, data.SearchResult{
				Conversation: cw.Conversation,
			})
		}
	} else {
		minScore := 0.0
		if f.matchMode == MatchFuzzy {
			minScore = fuzzyScoreThreshold
		}
		for _, cw := range f.convs {
			if f.researchOnly && !cw.Conversation.IsResearch {
				continue
			}
			best := scoreBest(cw, phrase, tokens, f.matchMode)
			if best.Score > minScore {
				results = append(results, best)
			}
		}
	}

	f.sortResults(results, len(tokens) > 0)

	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}
	return results, nil
}

func (f *FuzzySearcher) sortResults(results []data.SearchResult, hasQuery bool) {
	switch f.orderMode {
	case OrderRecent:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Conversation.UpdatedAt.After(results[j].Conversation.UpdatedAt)
		})
	case OrderCreated:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Conversation.CreatedAt.After(results[j].Conversation.CreatedAt)
		})
	case OrderName:
		sort.Slice(results, func(i, j int) bool {
			return strings.ToLower(results[i].Conversation.Name) < strings.ToLower(results[j].Conversation.Name)
		})
	default: // OrderScore — relevance when querying, most-recent otherwise
		if hasQuery {
			sort.Slice(results, func(i, j int) bool {
				return results[i].Score > results[j].Score
			})
		} else {
			sort.Slice(results, func(i, j int) bool {
				return results[i].Conversation.UpdatedAt.After(results[j].Conversation.UpdatedAt)
			})
		}
	}
}

type match struct {
	score   float64
	snippet string
	source  string // "title", "summary", "message"
}

func scoreBest(cw data.ConversationWithMessages, phrase string, tokens []string, mode MatchMode) data.SearchResult {
	var best match

	titleLower := strings.ToLower(cw.Conversation.Name)
	if s := scoreText(titleLower, phrase, tokens, mode); s > 0 {
		m := match{score: s * 3.0, snippet: cw.Conversation.Name, source: "title"}
		if m.score > best.score {
			best = m
		}
	}

	summaryLower := strings.ToLower(cw.Conversation.Summary)
	if s := scoreText(summaryLower, phrase, tokens, mode); s > 0 {
		m := match{score: s * 1.5, snippet: extractSnippet(cw.Conversation.Summary, phrase, tokens, 150), source: "summary"}
		if m.score > best.score {
			best = m
		}
	}

	for _, msg := range cw.Messages {
		textLower := strings.ToLower(msg.Text)
		if s := scoreText(textLower, phrase, tokens, mode); s > 0 {
			m := match{score: s, snippet: extractSnippet(msg.Text, phrase, tokens, 150), source: "message"}
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
		if snippet := findContentSnippet(cw, phrase, tokens); snippet != "" {
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

func findContentSnippet(cw data.ConversationWithMessages, phrase string, tokens []string) string {
	// Prefer summary
	if hasSubstringMatch(strings.ToLower(cw.Conversation.Summary), tokens) {
		return extractSnippet(cw.Conversation.Summary, phrase, tokens, 150)
	}
	for _, msg := range cw.Messages {
		if hasSubstringMatch(strings.ToLower(msg.Text), tokens) {
			return extractSnippet(msg.Text, phrase, tokens, 150)
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

// scoreText returns a score for how well text matches the query. Every token
// must be present (AND); a missing token yields no match. An exact consecutive
// phrase match earns a large boost so it ranks above scattered token matches.
func scoreText(text, phrase string, tokens []string, mode MatchMode) float64 {
	if text == "" {
		return 0
	}
	var matched, fuzzy int
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			matched++
		} else if mode == MatchFuzzy && fuzzyMatch(text, tok) {
			fuzzy++
		}
	}
	if matched+fuzzy < len(tokens) {
		return 0
	}
	score := (float64(matched) + 0.2*float64(fuzzy)) / float64(len(tokens))
	if len(tokens) > 1 && strings.Contains(text, phrase) {
		score += phraseBoost
	}
	return score
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

func extractSnippet(text, phrase string, tokens []string, maxLen int) string {
	lower := strings.ToLower(text)
	bestIdx := -1
	if len(tokens) > 1 {
		bestIdx = strings.Index(lower, phrase)
	}
	if bestIdx < 0 {
		for _, tok := range tokens {
			idx := strings.Index(lower, strings.ToLower(tok))
			if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
				bestIdx = idx
			}
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
