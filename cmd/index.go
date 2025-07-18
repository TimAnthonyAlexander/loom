package cmd

import (
	"fmt"
	"loom/config"
	"loom/indexer"
	"loom/workspace"
	"time"

	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Rebuild the workspace file index",
	Long:  `Force a complete rebuild of the workspace file index`,
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Load configuration
		cfg, err := config.LoadConfig(workspacePath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Ensure .loom directory exists
		err = workspace.EnsureLoomDir(workspacePath)
		if err != nil {
			fmt.Printf("Error creating .loom directory: %v\n", err)
			return
		}

		fmt.Printf("Rebuilding index for workspace: %s\n", workspacePath)
		start := time.Now()

		// Create and build index
		idx := indexer.NewIndex(workspacePath, cfg.MaxFileSize)
		err = idx.BuildIndex()
		if err != nil {
			fmt.Printf("Error building index: %v\n", err)
			return
		}

		// Save to cache
		err = idx.SaveToCache()
		if err != nil {
			fmt.Printf("Error saving index cache: %v\n", err)
			return
		}

		duration := time.Since(start)
		stats := idx.GetStats()

		fmt.Printf("Index rebuilt successfully in %v\n", duration)
		fmt.Printf("Indexed %d files (%.2f MB)\n",
			stats.TotalFiles,
			float64(stats.TotalSize)/1024/1024)

		// Show language breakdown
		if len(stats.LanguageBreakdown) > 0 {
			fmt.Println("\nLanguage breakdown:")
			for lang, count := range stats.LanguageBreakdown {
				if count > 0 {
					fmt.Printf("  %s: %d files (%.1f%%)\n",
						lang, count, stats.LanguagePercent[lang])
				}
			}
		}
	},
}
