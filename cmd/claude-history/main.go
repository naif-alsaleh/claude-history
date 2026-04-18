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
		syncCmd(&dbPath),
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

func syncCmd(dbPath *string) *cobra.Command {
	var (
		sessionKey     string
		updateExisting bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync conversations from claude.ai",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionKey == "" {
				sessionKey = os.Getenv("CLAUDE_SESSION_KEY")
			}
			if sessionKey == "" {
				return fmt.Errorf("session key required: use --session-key flag or CLAUDE_SESSION_KEY env var\n\nTo get your session key:\n1. Open claude.ai in your browser\n2. Open DevTools (F12) → Application → Cookies\n3. Copy the value of the 'sessionKey' cookie")
			}

			if err := ensureDBDir(*dbPath); err != nil {
				return err
			}
			store, err := data.NewStore(*dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			client := data.NewClient(sessionKey)
			ctx := context.Background()

			stats, err := data.Sync(ctx, store, client, data.SyncOptions{
				UpdateExisting: updateExisting,
			}, func(format string, args ...any) {
				fmt.Printf(format+"\n", args...)
			})
			if err != nil {
				return err
			}

			fmt.Printf("\nSync complete: %d listed, %d new, %d updated, %d skipped, %d errors\n",
				stats.Listed, stats.New, stats.Updated, stats.Skipped, stats.Errors)
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionKey, "session-key", "", "claude.ai session key (or set CLAUDE_SESSION_KEY)")
	cmd.Flags().BoolVar(&updateExisting, "update", false, "re-fetch conversations that have been updated")
	return cmd
}

func searchCmd(dbPath *string) *cobra.Command {
	var (
		maxResults   int
		researchOnly bool
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search conversations (one-shot CLI mode). No query lists all by date.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := data.NewStore(*dbPath)
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()
			searcher := search.NewFuzzySearcher(store)
			searcher.SetResearchOnly(researchOnly)
			if err := searcher.Index(ctx); err != nil {
				return err
			}

			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			return cli.RunSearch(ctx, searcher, query, maxResults)
		},
	}

	cmd.Flags().IntVarP(&maxResults, "max", "n", 10, "maximum number of results")
	cmd.Flags().BoolVarP(&researchOnly, "research", "r", false, "only search research conversations")
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
