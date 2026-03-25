package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/naif/claude-history/internal/data"
	"github.com/naif/claude-history/internal/search"
)

func RunSearch(ctx context.Context, searcher search.Searcher, query string, maxResults int) error {
	results, err := searcher.Search(ctx, query, maxResults)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	for i, r := range results {
		printResult(i+1, r)
	}
	return nil
}

func printResult(idx int, r data.SearchResult) {
	research := ""
	if r.Conversation.IsResearch {
		research = " [Research]"
	}

	fmt.Printf("\n%d. %s%s\n", idx, r.Conversation.Name, research)
	fmt.Printf("   %s\n", r.Conversation.CreatedAt.Format(time.DateOnly))
	fmt.Printf("   https://claude.ai/chat/%s\n", r.Conversation.UUID)
	if r.Snippet != "" {
		lines := wrapText(r.Snippet, 80)
		for _, line := range lines {
			fmt.Printf("   %s\n", line)
		}
	}
}

func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		if len(current)+1+len(w) > width {
			lines = append(lines, current)
			current = w
		} else {
			current += " " + w
		}
	}
	lines = append(lines, current)
	return lines
}
