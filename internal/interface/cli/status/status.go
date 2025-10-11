package status

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
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

// NewCommand creates the status command
func NewCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current workflow status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize container to access DB
			container, err := common.InitializeContainer()
			if err != nil {
				if jsonOutput {
					output := StatusOutput{
						Ts:    time.Now().UTC().Format(time.RFC3339Nano),
						Turn:  0,
						Step:  "idle",
						Ok:    false,
						Error: fmt.Sprintf("failed to initialize container: %v", err),
					}
					b, _ := json.Marshal(output)
					fmt.Println(string(b))
					os.Exit(1)
				}
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			// Query DB for currently executing SBI
			sbiRepo := container.GetSBIRepository()
			ctx := context.Background()

			// Find SBI in PICKED, IMPLEMENTING, or REVIEWING status
			filter := repository.SBIFilter{
				Statuses: []model.Status{
					model.StatusPicked,
					model.StatusImplementing,
					model.StatusReviewing,
				},
				Limit:  1,
				Offset: 0,
			}
			sbis, err := sbiRepo.List(ctx, filter)
			if err != nil {
				if jsonOutput {
					output := StatusOutput{
						Ts:    time.Now().UTC().Format(time.RFC3339Nano),
						Turn:  0,
						Step:  "idle",
						Ok:    false,
						Error: fmt.Sprintf("failed to query SBI: %v", err),
					}
					b, _ := json.Marshal(output)
					fmt.Println(string(b))
					os.Exit(1)
				}
				return fmt.Errorf("failed to query SBI: %w", err)
			}

			// Determine status
			var turn int
			var step string
			var updatedAt time.Time
			var sbiID string

			if len(sbis) > 0 {
				// Found an executing SBI
				sbi := sbis[0]
				if execState := sbi.ExecutionState(); execState != nil {
					turn = execState.CurrentTurn.Value()
				}
				step = sbi.CurrentStep().String()
				updatedAt = sbi.UpdatedAt().Value()
				sbiID = sbi.ID().String()
			} else {
				// No executing SBI, check for idle state
				step = "idle"
				turn = 0
				updatedAt = time.Now()
			}

			// Get the last journal error to determine ok status
			lastError, err := getLastJournalError()
			if err != nil {
				// If we can't read the journal, report the issue but continue
				lastError = fmt.Sprintf("journal read error: %v", err)
			}

			if jsonOutput {
				// JSON output mode
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
				if sbiID != "" {
					fmt.Printf("SBI     : %s\n", sbiID)
				}
				fmt.Printf("Current : %s\n", step)
				fmt.Printf("Turn    : %d\n", turn)
				fmt.Printf("Updated : %s\n", updatedAt.Format(time.RFC3339))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output status in JSON format")

	return cmd
}
