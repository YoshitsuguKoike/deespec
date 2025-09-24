package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type StatusOutput struct {
	Ts    string `json:"ts"`
	Turn  int    `json:"turn"`
	Step  string `json:"step"`
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

func newStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current workflow status",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := loadState("state.json")
			if err != nil {
				if jsonOutput {
					output := StatusOutput{
						Ts:    time.Now().Format(time.RFC3339Nano),
						Turn:  0,
						Step:  "unknown",
						Ok:    false,
						Error: fmt.Sprintf("read state: %v", err),
					}
					b, _ := json.Marshal(output)
					fmt.Println(string(b))
					os.Exit(1)
				}
				return fmt.Errorf("read state: %w", err)
			}

			if jsonOutput {
				// JSON output mode
				turn := st.Turn
				step := st.Current
				if step == "" {
					step = "unknown"
				}

				output := StatusOutput{
					Ts:    time.Now().Format(time.RFC3339Nano),
					Turn:  turn,
					Step:  step,
					Ok:    true,
					Error: "",
				}

				b, err := json.Marshal(output)
				if err != nil {
					return fmt.Errorf("marshal json: %w", err)
				}
				fmt.Println(string(b))
			} else {
				// Normal text output
				fmt.Printf("Current : %s\n", st.Current)
				fmt.Printf("Turn    : %d\n", st.Turn)
				fmt.Printf("Updated : %s\n", st.Meta.UpdatedAt)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output status in JSON format")

	return cmd
}