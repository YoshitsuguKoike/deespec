package pbi

import (
	"github.com/spf13/cobra"
)

// NewPBICommand creates a new pbi command
func NewPBICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pbi",
		Short: "Manage Product Backlog Items",
		Long:  "Commands for managing Product Backlog Items (PBIs)",
	}

	// Add subcommands
	cmd.AddCommand(NewRegisterCommand())
	cmd.AddCommand(NewShowCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewEditCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewDecomposeCommand())
	cmd.AddCommand(NewSBICommand())
	cmd.AddCommand(NewRegisterSBIsCommand())

	return cmd
}
