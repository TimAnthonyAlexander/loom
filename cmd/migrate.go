package cmd

import (
	"fmt"
	"loom/paths"
	"loom/workspace"

	"github.com/spf13/cobra"
)

var (
	migrateBackup bool
	migrateForce  bool
	migrateClean  bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate existing .loom directories to new user-level storage",
	Long: `Migrate existing .loom directories from project workspaces to the new user-level storage structure.
This command will move all loom data from <workspace>/.loom/ to ~/.loom/projects/<project-hash>/`,
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Check migration status
		status, err := paths.CheckMigrationStatus(workspacePath)
		if err != nil {
			fmt.Printf("Error checking migration status: %v\n", err)
			return
		}

		fmt.Printf("Migration Status for: %s\n", workspacePath)
		fmt.Printf("  Has legacy .loom: %t\n", status.HasLegacyLoom)
		fmt.Printf("  Migration needed: %t\n", status.MigrationNeeded)
		fmt.Printf("  Migration complete: %t\n", status.MigrationComplete)

		if len(status.Issues) > 0 {
			fmt.Println("  Issues:")
			for _, issue := range status.Issues {
				fmt.Printf("    - %s\n", issue)
			}
		}

		if !status.HasLegacyLoom {
			fmt.Println("No legacy .loom directory found. Nothing to migrate.")
			return
		}

		if !status.MigrationNeeded {
			fmt.Println("Legacy .loom directory exists but is empty. Nothing to migrate.")
			if migrateClean {
				fmt.Println("Removing empty legacy .loom directory...")
				if err := paths.RemoveLegacyLoom(workspacePath, true); err != nil {
					fmt.Printf("Error removing legacy directory: %v\n", err)
				}
			}
			return
		}

		if status.MigrationComplete && !migrateForce {
			fmt.Println("Migration already completed. Use --force to migrate again.")
			if migrateClean {
				fmt.Println("Cleaning up legacy .loom directory...")
				if err := paths.RemoveLegacyLoom(workspacePath, false); err != nil {
					fmt.Printf("Error removing legacy directory: %v\n", err)
				}
			}
			return
		}

		// Perform migration
		fmt.Println("Starting migration...")
		if err := paths.MigrateWorkspace(workspacePath, migrateBackup); err != nil {
			fmt.Printf("Migration failed: %v\n", err)
			return
		}

		// Clean up legacy directory if requested
		if migrateClean {
			fmt.Println("Cleaning up legacy .loom directory...")
			if err := paths.RemoveLegacyLoom(workspacePath, false); err != nil {
				fmt.Printf("Error removing legacy directory: %v\n", err)
				return
			}
		}

		fmt.Println("Migration completed successfully!")
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check migration status for current workspace",
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		// Check migration status
		status, err := paths.CheckMigrationStatus(workspacePath)
		if err != nil {
			fmt.Printf("Error checking migration status: %v\n", err)
			return
		}

		fmt.Printf("Migration Status for: %s\n", workspacePath)
		fmt.Printf("  Has legacy .loom: %t\n", status.HasLegacyLoom)
		fmt.Printf("  Migration needed: %t\n", status.MigrationNeeded)
		fmt.Printf("  Migration complete: %t\n", status.MigrationComplete)

		if len(status.Issues) > 0 {
			fmt.Println("  Issues:")
			for _, issue := range status.Issues {
				fmt.Printf("    - %s\n", issue)
			}
		}

		if status.MigrationComplete {
			// Show new location
			projectPaths, err := paths.NewProjectPaths(workspacePath)
			if err == nil {
				fmt.Printf("  New location: %s\n", projectPaths.ProjectDir())
			}
		}
	},
}

var migrateCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove legacy .loom directory after successful migration",
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			return
		}

		if err := paths.RemoveLegacyLoom(workspacePath, migrateForce); err != nil {
			fmt.Printf("Error removing legacy directory: %v\n", err)
			return
		}

		fmt.Println("Legacy .loom directory removed successfully!")
	},
}

func init() {
	// Main migrate command flags
	migrateCmd.Flags().BoolVar(&migrateBackup, "backup", true, "Create backup of legacy .loom directory")
	migrateCmd.Flags().BoolVar(&migrateForce, "force", false, "Force migration even if already completed")
	migrateCmd.Flags().BoolVar(&migrateClean, "clean", false, "Remove legacy .loom directory after migration")

	// Clean command flags
	migrateCleanCmd.Flags().BoolVar(&migrateForce, "force", false, "Force removal without checking migration status")

	// Add subcommands
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateCleanCmd)

	// Add to root
	rootCmd.AddCommand(migrateCmd)
}
