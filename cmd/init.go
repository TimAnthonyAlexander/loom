package cmd

import (
	"fmt"
	"loom/config"
	"loom/workspace"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize loom configuration for current directory",
	Long:  `Initialize loom configuration for current directory`,
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Create local config with default values
		cfg := config.DefaultConfig()
		err = config.SaveLocalConfig(workspacePath, cfg)
		if err != nil {
			fmt.Printf("Error saving local config: %v\n", err)
			return
		}

		fmt.Printf("Initialized loom for %s\n", workspacePath)
		fmt.Println("Created project-specific configuration with default settings")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
