package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// LabelIndex represents the label to SBI mapping
type LabelIndex struct {
	Labels map[string][]string `json:"labels"`
}

// newLabelCmd creates the label command group
func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels for SBI specifications",
		Long: `Manage labels for SBI specifications to categorize and enhance AI context.

Labels help organize SBIs and provide additional context to AI prompts,
enabling more specialized and accurate implementations.`,
		Example: `  # Set labels for current SBI (replaces all)
  deespec label set transaction validation critical

  # Add labels to current SBI (appends)
  deespec label add performance

  # List all labels in the project
  deespec label list

  # Search SBIs by label
  deespec label search transaction

  # Delete specific labels from current SBI
  deespec label delete validation

  # Clear all labels from current SBI
  deespec label clear`,
	}

	// Add subcommands
	cmd.AddCommand(newLabelSetCmd())
	cmd.AddCommand(newLabelAddCmd())
	cmd.AddCommand(newLabelListCmd())
	cmd.AddCommand(newLabelSearchCmd())
	cmd.AddCommand(newLabelDeleteCmd())
	cmd.AddCommand(newLabelClearCmd())

	return cmd
}

// newLabelSetCmd creates the label set command
func newLabelSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <label1> [label2] ...",
		Short: "Set labels for current SBI (replaces all existing labels)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setLabels(args, false)
		},
	}
}

// newLabelAddCmd creates the label add command
func newLabelAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <label1> [label2] ...",
		Short: "Add labels to current SBI (appends to existing)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setLabels(args, true)
		},
	}
}

// newLabelListCmd creates the label list command
func newLabelListCmd() *cobra.Command {
	var showCount bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all labels in the project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listLabels(showCount)
		},
	}
	cmd.Flags().BoolVarP(&showCount, "count", "c", false, "Show SBI count for each label")
	return cmd
}

// newLabelSearchCmd creates the label search command
func newLabelSearchCmd() *cobra.Command {
	var matchAll bool
	cmd := &cobra.Command{
		Use:   "search <label1> [label2] ...",
		Short: "Search SBIs by label",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return searchByLabels(args, matchAll)
		},
	}
	cmd.Flags().BoolVarP(&matchAll, "all", "a", false, "Match all labels (AND operation)")
	return cmd
}

// newLabelDeleteCmd creates the label delete command
func newLabelDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <label1> [label2] ...",
		Short: "Delete specific labels from current SBI",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteLabels(args)
		},
	}
}

// newLabelClearCmd creates the label clear command
func newLabelClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear all labels from current SBI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return clearLabels()
		},
	}
}

// Helper functions (implementations would go here)

// LoadState loads the current state from the default location
func LoadState() (*State, error) {
	return loadState(".deespec/var/state.json")
}

// SaveState saves the state to the default location
func SaveState(st *State) error {
	// Load current state to get version for CAS
	current, err := loadState(".deespec/var/state.json")
	if err != nil {
		// If file doesn't exist, start with version 0
		if os.IsNotExist(err) {
			return saveStateCAS(".deespec/var/state.json", st, 0)
		}
		return err
	}
	return saveStateCAS(".deespec/var/state.json", st, current.Version)
}

func setLabels(labels []string, appendMode bool) error {
	// Load current state to get WIP SBI
	st, err := LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if st.WIP == "" {
		return fmt.Errorf("no WIP SBI currently selected")
	}

	// Load meta.yml for the WIP SBI
	metaPath := filepath.Join(".deespec", st.WIP, "meta.yml")
	meta, err := loadSBIMeta(metaPath)
	if err != nil {
		return fmt.Errorf("failed to load SBI meta: %w", err)
	}

	// Update labels
	if appendMode && meta.Labels != nil {
		// Append to existing, avoiding duplicates
		existingMap := make(map[string]bool)
		for _, label := range meta.Labels {
			existingMap[label] = true
		}
		for _, label := range labels {
			if !existingMap[label] {
				meta.Labels = append(meta.Labels, label)
			}
		}
	} else {
		// Replace all labels
		meta.Labels = labels
	}

	// Save updated meta.yml
	if err := saveSBIMeta(metaPath, meta); err != nil {
		return fmt.Errorf("failed to save SBI meta: %w", err)
	}

	// Update label index
	if err := updateLabelIndex(); err != nil {
		return fmt.Errorf("failed to update label index: %w", err)
	}

	fmt.Printf("Labels updated for %s: %s\n", st.WIP, strings.Join(meta.Labels, ", "))
	return nil
}

func listLabels(showCount bool) error {
	index, err := loadLabelIndex()
	if err != nil {
		return fmt.Errorf("failed to load label index: %w", err)
	}

	if len(index.Labels) == 0 {
		fmt.Println("No labels found in the project")
		return nil
	}

	// Sort labels alphabetically
	var labels []string
	for label := range index.Labels {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	fmt.Println("Project Labels:")
	for _, label := range labels {
		if showCount {
			fmt.Printf("  %s (%d SBIs)\n", label, len(index.Labels[label]))
		} else {
			fmt.Printf("  %s\n", label)
		}
	}

	return nil
}

func searchByLabels(searchLabels []string, matchAll bool) error {
	index, err := loadLabelIndex()
	if err != nil {
		return fmt.Errorf("failed to load label index: %w", err)
	}

	// Find matching SBIs
	sbiMatches := make(map[string]int)
	for _, label := range searchLabels {
		if sbis, ok := index.Labels[label]; ok {
			for _, sbi := range sbis {
				sbiMatches[sbi]++
			}
		}
	}

	// Filter based on match criteria
	var results []string
	for sbi, count := range sbiMatches {
		if matchAll && count == len(searchLabels) {
			results = append(results, sbi)
		} else if !matchAll && count > 0 {
			results = append(results, sbi)
		}
	}

	if len(results) == 0 {
		fmt.Println("No SBIs found with specified labels")
		return nil
	}

	sort.Strings(results)
	fmt.Printf("Found %d SBIs:\n", len(results))
	for _, sbi := range results {
		// Load and display SBI description
		metaPath := filepath.Join(".deespec", sbi, "meta.yml")
		if meta, err := loadSBIMeta(metaPath); err == nil {
			fmt.Printf("  %s: %s\n", sbi, meta.Description)
			if len(meta.Labels) > 0 {
				fmt.Printf("    Labels: %s\n", strings.Join(meta.Labels, ", "))
			}
		} else {
			fmt.Printf("  %s\n", sbi)
		}
	}

	return nil
}

func deleteLabels(labels []string) error {
	st, err := LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if st.WIP == "" {
		return fmt.Errorf("no WIP SBI currently selected")
	}

	metaPath := filepath.Join(".deespec", st.WIP, "meta.yml")
	meta, err := loadSBIMeta(metaPath)
	if err != nil {
		return fmt.Errorf("failed to load SBI meta: %w", err)
	}

	// Remove specified labels
	labelMap := make(map[string]bool)
	for _, label := range labels {
		labelMap[label] = true
	}

	var newLabels []string
	for _, label := range meta.Labels {
		if !labelMap[label] {
			newLabels = append(newLabels, label)
		}
	}

	meta.Labels = newLabels

	if err := saveSBIMeta(metaPath, meta); err != nil {
		return fmt.Errorf("failed to save SBI meta: %w", err)
	}

	if err := updateLabelIndex(); err != nil {
		return fmt.Errorf("failed to update label index: %w", err)
	}

	fmt.Printf("Labels removed from %s\n", st.WIP)
	return nil
}

func clearLabels() error {
	st, err := LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if st.WIP == "" {
		return fmt.Errorf("no WIP SBI currently selected")
	}

	metaPath := filepath.Join(".deespec", st.WIP, "meta.yml")
	meta, err := loadSBIMeta(metaPath)
	if err != nil {
		return fmt.Errorf("failed to load SBI meta: %w", err)
	}

	meta.Labels = nil

	if err := saveSBIMeta(metaPath, meta); err != nil {
		return fmt.Errorf("failed to save SBI meta: %w", err)
	}

	if err := updateLabelIndex(); err != nil {
		return fmt.Errorf("failed to update label index: %w", err)
	}

	fmt.Printf("All labels cleared from %s\n", st.WIP)
	return nil
}

// SBIMeta represents the structure of meta.yml files
type SBIMeta struct {
	ID          string   `yaml:"id"`
	Description string   `yaml:"description"`
	Status      string   `yaml:"status"`
	Labels      []string `yaml:"labels,omitempty"`
}

func loadSBIMeta(path string) (*SBIMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta SBIMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func saveSBIMeta(path string, meta *SBIMeta) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func loadLabelIndex() (*LabelIndex, error) {
	indexPath := filepath.Join(".deespec", "var", "labels.json")

	// Return empty index if file doesn't exist
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return &LabelIndex{Labels: make(map[string][]string)}, nil
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var index LabelIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	if index.Labels == nil {
		index.Labels = make(map[string][]string)
	}

	return &index, nil
}

func saveLabelIndex(index *LabelIndex) error {
	indexPath := filepath.Join(".deespec", "var", "labels.json")

	// Ensure var directory exists
	varDir := filepath.Dir(indexPath)
	if err := os.MkdirAll(varDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}

func updateLabelIndex() error {
	index := &LabelIndex{Labels: make(map[string][]string)}

	// Walk through all SBI directories
	sbiDir := ".deespec"
	entries, err := os.ReadDir(sbiDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "var" {
			continue
		}

		// Check for meta.yml or meta.yaml
		metaPath := filepath.Join(sbiDir, entry.Name(), "meta.yml")
		if _, err := os.Stat(metaPath); os.IsNotExist(err) {
			metaPath = filepath.Join(sbiDir, entry.Name(), "meta.yaml")
			if _, err := os.Stat(metaPath); os.IsNotExist(err) {
				continue
			}
		}

		meta, err := loadSBIMeta(metaPath)
		if err != nil {
			continue
		}

		// Add to index
		for _, label := range meta.Labels {
			if index.Labels[label] == nil {
				index.Labels[label] = []string{}
			}
			index.Labels[label] = append(index.Labels[label], entry.Name())
		}
	}

	return saveLabelIndex(index)
}
