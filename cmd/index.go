package cmd

import (
	"fmt"
	"loom/config"
	"loom/indexer"
	"loom/workspace"
	"sort"
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
			type langPair struct {
				name    string
				count   int
				percent float64
			}

			var langs []langPair
			for lang, count := range stats.LanguageBreakdown {
				if count > 0 {
					langs = append(langs, langPair{lang, count, stats.LanguagePercent[lang]})
				}
			}

			sort.Slice(langs, func(i, j int) bool {
				if langs[i].count != langs[j].count {
					return langs[i].count > langs[j].count
				}
				return langs[i].name < langs[j].name
			})

			for _, lang := range langs {
				fmt.Printf("  %s: %d files (%.1f%%)\n", lang.name, lang.count, lang.percent)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}
