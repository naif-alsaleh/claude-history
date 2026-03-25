package search

import (
	"context"

	"github.com/naif/claude-history/internal/data"
)

type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]data.SearchResult, error)
	// Index loads or refreshes searchable data from the store.
	Index(ctx context.Context) error
}
