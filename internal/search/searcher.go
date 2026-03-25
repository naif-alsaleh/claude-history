package search

import (
	"context"

	"github.com/naif/claude-history/internal/data"
)

type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]data.SearchResult, error)
	Index(ctx context.Context) error
	SetResearchOnly(bool)
	ResearchOnly() bool
}
