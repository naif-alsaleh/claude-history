package search

import (
	"context"

	"github.com/naif/claude-history/internal/data"
)

type OrderMode int

const (
	OrderDate OrderMode = iota
	OrderScore
	OrderName
)

var orderModes = []OrderMode{OrderDate, OrderScore, OrderName}

func (o OrderMode) String() string {
	switch o {
	case OrderDate:
		return "date"
	case OrderScore:
		return "score"
	case OrderName:
		return "name"
	default:
		return "date"
	}
}

func (o OrderMode) Next() OrderMode {
	return orderModes[(int(o)+1)%len(orderModes)]
}

type MatchMode int

const (
	MatchFuzzy MatchMode = iota
	MatchSubstring
)

var matchModes = []MatchMode{MatchFuzzy, MatchSubstring}

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
