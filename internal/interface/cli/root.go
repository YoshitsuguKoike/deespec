package cli

import "github.com/spf13/cobra"

func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deespec",
		Short: "DeeSpec CLI",
		RunE:  func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newDoctorIntegratedCmd())
	cmd.AddCommand(newJournalCmd())
	cmd.AddCommand(newStateCmd())
	cmd.AddCommand(newHealthCmd())
	cmd.AddCommand(workflowCmd)
	cmd.AddCommand(NewSBICommand())
	return cmd
}
