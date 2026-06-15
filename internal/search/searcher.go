package search

import (
	"context"

	"github.com/naif/claude-history/internal/data"
)

type OrderMode int

const (
	OrderScore OrderMode = iota
	OrderRecent
	OrderCreated
	OrderName
)

var orderModes = []OrderMode{OrderScore, OrderRecent, OrderCreated, OrderName}

func (o OrderMode) String() string {
	switch o {
	case OrderScore:
		return "score"
	case OrderRecent:
		return "recent"
	case OrderCreated:
		return "created"
	case OrderName:
		return "name"
	default:
		return "score"
	}
}

func (o OrderMode) Next() OrderMode {
	return orderModes[(int(o)+1)%len(orderModes)]
}

type MatchMode int

const (
	MatchSubstring MatchMode = iota
	MatchFuzzy
)

var matchModes = []MatchMode{MatchSubstring, MatchFuzzy}

func (m MatchMode) String() string {
	switch m {
	case MatchFuzzy:
		return "fuzzy"
	case MatchSubstring:
		return "substring"
	default:
		return "fuzzy"
	}
}

func (m MatchMode) Next() MatchMode {
	return matchModes[(int(m)+1)%len(matchModes)]
}

type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]data.SearchResult, error)
	Index(ctx context.Context) error
	SetResearchOnly(bool)
	ResearchOnly() bool
	SetOrderMode(OrderMode)
	OrderMode() OrderMode
	SetMatchMode(MatchMode)
	MatchMode() MatchMode
}
