package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/naif/claude-history/internal/cli"
	"github.com/naif/claude-history/internal/data"
	"github.com/naif/claude-history/internal/search"
	"github.com/naif/claude-history/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func defaultDBPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "claude-history", "conversations.duckdb")
}

func rootCmd() *cobra.Command {
	var dbPath string

	root := &cobra.Command{
		Use:   "claude-history",
		Short: "Search and browse Claude conversation history",
	}

	root.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "path to DuckDB database")

	root.AddCommand(
		importCmd(&dbPath),
		searchCmd(&dbPath),
		tuiCmd(&dbPath),
	)

	return root
}

func ensureDBDir(dbPath string) error {
	return os.MkdirAll(filepath.Dir(dbPath), 0o755)
}

func importCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "import [path-to-conversations.json]",
		Short: "Import conversations from Claude export JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureDBDir(*dbPath); err != nil {
				return err
			}
			store, err := data.NewStore(*dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()
			stats, err := data.Import(ctx, store, args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Import complete: %d total, %d new, %d skipped\n", stats.Total, stats.New, stats.Skipped)
			return nil
		},
	}
}

func searchCmd(dbPath *string) *cobra.Command {
	var maxResults int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search conversations (one-shot CLI mode)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := data.NewStore(*dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()
			searcher := search.NewFuzzySearcher(store)
			if err := searcher.Index(ctx); err != nil {
				return err
			}

			query := args[0]
			return cli.RunSearch(ctx, searcher, query, maxResults)
		},
	}

	cmd.Flags().IntVarP(&maxResults, "max", "n", 10, "maximum number of results")
	return cmd
}

func tuiCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Interactive TUI for browsing and searching conversations",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := data.NewStore(*dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()
			searcher := search.NewFuzzySearcher(store)
			if err := searcher.Index(ctx); err != nil {
				return err
			}

			return tui.Run(searcher)
		},
	}
}
