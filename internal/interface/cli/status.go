package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/spf13/cobra"
)

type StatusOutput struct {
	Ts    string `json:"ts"`
	Turn  int    `json:"turn"`
	Step  string `json:"step"`
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

// getLastJournalError reads the last line of journal.ndjson and returns the error field
func getLastJournalError() (string, error) {
	file, err := os.Open("journal.ndjson")
	if err != nil {
		// If journal doesn't exist, assume no error
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	var lastLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	if lastLine == "" {
		// Empty journal, no error
		return "", nil
	}

	// Parse the last journal entry
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		return "", err
	}

	// Get the error field
	if errorStr, ok := entry["error"].(string); ok {
		return errorStr, nil
	}

	return "", nil
}

func newStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current workflow status",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := app.GetPathsWithConfig(globalConfig)
			st, err := loadState(paths.State)
			if err != nil {
				if jsonOutput {
					output := StatusOutput{
						Ts:    time.Now().UTC().Format(time.RFC3339Nano),
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

				// Get the last journal error to determine ok status
				lastError, err := getLastJournalError()
				if err != nil {
					// If we can't read the journal, report the issue but continue
					lastError = fmt.Sprintf("journal read error: %v", err)
				}

				output := StatusOutput{
					Ts:    time.Now().UTC().Format(time.RFC3339Nano),
					Turn:  turn,
					Step:  step,
					Ok:    lastError == "",
					Error: lastError,
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
