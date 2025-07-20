package cmd

import (
	"fmt"
	"loom/config"
	"loom/indexer"
	"loom/llm"
	"loom/workspace"
	"strings"

	"github.com/spf13/cobra"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage project-specific rules",
	Long:  `Add, remove, or list project-specific rules that will be included in AI prompts`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var addRuleCmd = &cobra.Command{
	Use:   "add [rule-text]",
	Short: "Add a new project-specific rule",
	Long:  `Add a new project-specific rule that will be included in AI prompts for this project`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Load configuration to get maxFileSize
		cfg, err := config.LoadConfig(workspacePath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create a simple index for the prompt enhancer
		idx := indexer.NewIndex(workspacePath, cfg.MaxFileSize)
		enhancer := llm.NewPromptEnhancer(workspacePath, idx)

		// Join all args as the rule text
		ruleText := strings.Join(args, " ")

		// Get optional description from flag
		description, _ := cmd.Flags().GetString("description")

		err = enhancer.AddProjectRule(ruleText, description)
		if err != nil {
			fmt.Printf("Error adding rule: %v\n", err)
			return
		}

		fmt.Printf("Rule added successfully: %s\n", ruleText)
		if description != "" {
			fmt.Printf("Description: %s\n", description)
		}
	},
}

var removeRuleCmd = &cobra.Command{
	Use:   "remove [rule-id]",
	Short: "Remove a project-specific rule",
	Long:  `Remove a project-specific rule by its ID`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Load configuration to get maxFileSize
		cfg, err := config.LoadConfig(workspacePath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create a simple index for the prompt enhancer
		idx := indexer.NewIndex(workspacePath, cfg.MaxFileSize)
		enhancer := llm.NewPromptEnhancer(workspacePath, idx)

		ruleID := args[0]

		err = enhancer.RemoveProjectRule(ruleID)
		if err != nil {
			fmt.Printf("Error removing rule: %v\n", err)
			return
		}

		fmt.Printf("Rule removed successfully: %s\n", ruleID)
	},
}

var listRulesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all project-specific rules",
	Long:  `List all project-specific rules for this project`,
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Load configuration to get maxFileSize
		cfg, err := config.LoadConfig(workspacePath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}

		// Create a simple index for the prompt enhancer
		idx := indexer.NewIndex(workspacePath, cfg.MaxFileSize)
		enhancer := llm.NewPromptEnhancer(workspacePath, idx)

		rules, err := enhancer.ListProjectRules()
		if err != nil {
			fmt.Printf("Error listing rules: %v\n", err)
			return
		}

		if len(rules.Rules) == 0 {
			fmt.Println("No project-specific rules found.")
			fmt.Println("Add rules with: loom rules add \"your rule text\"")
			return
		}

		fmt.Printf("Project-specific rules (%d):\n\n", len(rules.Rules))
		for _, rule := range rules.Rules {
			fmt.Printf("ID: %s\n", rule.ID)
			fmt.Printf("Text: %s\n", rule.Text)
			if rule.Description != "" {
				fmt.Printf("Description: %s\n", rule.Description)
			}
			fmt.Printf("Created: %s\n", rule.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
	},
}

func init() {
	// Add the --description flag to the add command
	addRuleCmd.Flags().StringP("description", "d", "", "Optional description for the rule")

	// Add subcommands to rules command
	rulesCmd.AddCommand(addRuleCmd)
	rulesCmd.AddCommand(removeRuleCmd)
	rulesCmd.AddCommand(listRulesCmd)

	// Add rules command to root
	rootCmd.AddCommand(rulesCmd)
}
