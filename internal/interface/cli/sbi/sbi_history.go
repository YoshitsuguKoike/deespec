package sbi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// sbiHistoryFlags holds the flags for sbi history command
type sbiHistoryFlags struct {
	limit   int  // Limit number of history entries
	jsonOut bool // Output in JSON format
}

// JournalEntry represents a journal entry from journal.ndjson
type JournalEntry struct {
	Timestamp  string                 `json:"timestamp"`
	SbiID      string                 `json:"sbi_id"`
	Turn       int                    `json:"turn"`
	Step       string                 `json:"step"`
	Status     string                 `json:"status"`
	Decision   string                 `json:"decision,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// NewSBIHistoryCommand creates the sbi history command
func NewSBIHistoryCommand() *cobra.Command {
	flags := &sbiHistoryFlags{}

	cmd := &cobra.Command{
		Use:   "history <id>",
		Short: "Show execution history of an SBI",
		Long: `Display the execution history of an SBI task from journal.ndjson.

Shows all turns, steps, decisions, and errors that occurred during execution.

Examples:
  # Show full history
  deespec sbi history 010b1f9c-2cbf-40e6-90d8-ecba5b62d335

  # Show last 10 entries
  deespec sbi history 010b1f9c --limit 10

  # Show in JSON format
  deespec sbi history 010b1f9c --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSBIHistory(cmd.Context(), args[0], flags)
		},
	}

	// Define flags
	cmd.Flags().IntVar(&flags.limit, "limit", 50, "Maximum number of history entries to show")
	cmd.Flags().BoolVar(&flags.jsonOut, "json", false, "Output in JSON format")

	return cmd
}

// runSBIHistory executes the sbi history command
func runSBIHistory(ctx context.Context, sbiID string, flags *sbiHistoryFlags) error {
	// Get journal path
	journalPath := ".deespec/var/journal.ndjson"
	if common.GetGlobalConfig() != nil {
		// Use config if available
		journalPath = ".deespec/var/journal.ndjson" // Default path
	}

	// Read journal file
	file, err := os.Open(journalPath)
	if err != nil {
		// Check if file doesn't exist
		if os.IsNotExist(err) {
			// Try to get SBI info to provide better error message
			container, containerErr := common.InitializeContainer()
			if containerErr == nil {
				defer container.Close()
				sbiRepo := container.GetSBIRepository()

				// Try to find SBI by ID (convert string to repository.SBIID)
				sbi, findErr := sbiRepo.Find(ctx, repository.SBIID(sbiID))
				if findErr == nil && sbi != nil {
					status := string(sbi.Status())
					step := string(sbi.CurrentStep())

					// If task hasn't been executed yet
					if status == "PENDING" || step == "PICK" {
						fmt.Printf("No execution history found for SBI: %s\n", sbiID)
						fmt.Printf("\nℹ️  This task has not been executed yet (Status: %s, Step: %s)\n", status, step)
						fmt.Printf("   History will be available after the task starts executing.\n")
						fmt.Printf("\nTo execute this task, run:\n")
						fmt.Printf("   deespec run\n")
						return nil
					}
				}
			}

			// Generic message if we can't get SBI info
			return fmt.Errorf("journal file not found: %s\n\nℹ️  Journal entries are created when SBI tasks are executed.\n   If the task hasn't run yet, there will be no history.\n   Run 'deespec sbi list' to check task status", journalPath)
		}

		// Other errors
		return fmt.Errorf("failed to open journal: %w (path: %s)", err, journalPath)
	}
	defer file.Close()

	// Parse journal entries
	// Support both JSON array format and NDJSON format
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return fmt.Errorf("failed to read journal file: %w", err)
	}

	var allEntries []JournalEntry

	// Try to parse as JSON array first (new format)
	if err := json.Unmarshal(data, &allEntries); err != nil {
		// If that fails, try NDJSON format (old format)
		allEntries = []JournalEntry{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var entry JournalEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				// Skip malformed lines
				continue
			}
			allEntries = append(allEntries, entry)
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read journal: %w", err)
		}
	}

	// Filter by SBI ID (support both full and partial ID match)
	var entries []JournalEntry
	for _, entry := range allEntries {
		if entry.SbiID == sbiID || strings.HasPrefix(entry.SbiID, sbiID) {
			entries = append(entries, entry)
		}
	}

	// Apply limit (show most recent entries)
	if len(entries) > flags.limit {
		entries = entries[len(entries)-flags.limit:]
	}

	// Output results
	if flags.jsonOut {
		return outputJSONHistory(entries)
	}

	return outputTableHistory(entries, sbiID)
}

// outputTableHistory outputs history in table format
func outputTableHistory(entries []JournalEntry, sbiID string) error {
	if len(entries) == 0 {
		fmt.Printf("No history found for SBI: %s\n", sbiID)
		fmt.Printf("\nNote: Journal entries are written during SBI execution.\n")
		fmt.Printf("If this SBI hasn't been executed yet, there will be no history.\n")
		return nil
	}

	fmt.Printf("Execution History for SBI: %s\n", sbiID)
	fmt.Printf("==============================================\n\n")

	for i, entry := range entries {
		// Parse timestamp
		timestamp := entry.Timestamp
		if t, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
			timestamp = t.Format("2006-01-02 15:04:05")
		}

		fmt.Printf("[%d] %s\n", i+1, timestamp)
		fmt.Printf("    Turn: %d, Step: %s, Status: %s\n", entry.Turn, entry.Step, entry.Status)

		if entry.Decision != "" {
			fmt.Printf("    Decision: %s\n", entry.Decision)
		}
		if entry.Error != "" {
			fmt.Printf("    Error: %s\n", entry.Error)
		}
		fmt.Printf("\n")
	}

	fmt.Printf("Total: %d history entries\n", len(entries))

	return nil
}

// outputJSONHistory outputs history in JSON format
func outputJSONHistory(entries []JournalEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
