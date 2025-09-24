package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current workflow status",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := loadState("state.json")
			if err != nil {
				return fmt.Errorf("read state: %w", err)
			}
			fmt.Printf("Current : %s\n", st.Current)
			fmt.Printf("Turn    : %d\n", st.Turn)
			fmt.Printf("Updated : %s\n", st.Meta.UpdatedAt)
			return nil
		},
	}
}
