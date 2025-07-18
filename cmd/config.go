package cmd

import (
	"fmt"
	"loom/config"
	"loom/workspace"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage loom configuration",
	Long:  `Get and set configuration values for loom`,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]

		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		cfg, err := config.LoadConfig(workspacePath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		value, err := cfg.Get(key)
		if err != nil {
			fmt.Printf("Error getting config value: %v\n", err)
			return
		}

		fmt.Printf("%s = %v\n", key, value)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		cfg, err := config.LoadConfig(workspacePath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		err = cfg.Set(key, value)
		if err != nil {
			fmt.Printf("Error setting config value: %v\n", err)
			return
		}

		err = config.SaveLocalConfig(workspacePath, cfg)
		if err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Printf("Set %s = %s\n", key, value)
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
