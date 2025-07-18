package cmd

import (
	"fmt"
	"loom/config"
	"loom/workspace"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .loom folder and config in current directory",
	Long:  `Initialize .loom folder and config in current directory`,
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Ensure .loom directory exists
		err = workspace.EnsureLoomDir(workspacePath)
		if err != nil {
			fmt.Printf("Error creating .loom directory: %v\n", err)
			return
		}

		// Create local config with default values
		cfg := config.DefaultConfig()
		err = config.SaveLocalConfig(workspacePath, cfg)
		if err != nil {
			fmt.Printf("Error saving local config: %v\n", err)
			return
		}

		fmt.Printf("Initialized loom in %s\n", workspacePath)
		fmt.Println("Created .loom/config.json with default settings")
	},
}
