package sbi

import "github.com/spf13/cobra"

// NewSBICommand creates the sbi command with its subcommands
func NewSBICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sbi",
		Short: "SBI (Spec Backlog Item) management commands",
		Long:  "Manage SBI specifications including registration, validation, and processing",
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewSBIRegisterCommand())
	cmd.AddCommand(NewSBIRunCommand())
	cmd.AddCommand(NewSBIExtractCommand())
	cmd.AddCommand(NewSBIListCommand())
	cmd.AddCommand(NewSBIShowCommand())
	cmd.AddCommand(NewSBIResetCommand())
	cmd.AddCommand(NewSBIHistoryCommand())

	return cmd
}
