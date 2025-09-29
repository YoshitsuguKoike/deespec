package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
	"github.com/YoshitsuguKoike/deespec/internal/workflow"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// DoctorJSON represents the JSON output structure for doctor command
type DoctorJSON struct {
	Runner           string             `json:"runner"`
	Active           bool               `json:"active"`
	WorkingDir       string             `json:"working_dir"`
	AgentBin         string             `json:"agent_bin"`
	StartIntervalSec int                `json:"start_interval_sec,omitempty"`
	Next             string             `json:"next,omitempty"`
	Errors           []string           `json:"errors"`
	Metrics          *DoctorMetricsJSON `json:"metrics,omitempty"`
	ConfigSource     string             `json:"config_source,omitempty"`
	SettingPath      string             `json:"setting_path,omitempty"`
}

// DoctorMetricsJSON represents transaction metrics for doctor --json
type DoctorMetricsJSON struct {
	CommitSuccess int64   `json:"commit_success"`
	CommitFailed  int64   `json:"commit_failed"`
	CASConflicts  int64   `json:"cas_conflicts"`
	RecoveryCount int64   `json:"recovery_count"`
	TotalCommits  int64   `json:"total_commits"`
	SuccessRate   float64 `json:"success_rate_percent"`
	LastUpdate    string  `json:"last_update"`
}

// DoctorValidationJSON represents the JSON output structure for --format=json
type DoctorValidationJSON struct {
	Steps   []DoctorStepJSON  `json:"steps"`
	Summary DoctorSummaryJSON `json:"summary"`
}

type DoctorStepJSON struct {
	ID     string            `json:"id"`
	Path   string            `json:"path"`
	Issues []DoctorIssueJSON `json:"issues"`
}

type DoctorIssueJSON struct {
	Type    string `json:"type"` // "ok", "warn", "error"
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type DoctorSummaryJSON struct {
	Steps int `json:"steps"`
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
}

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool
	var format string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment & configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runDoctorJSON()
			}
			if format == "json" {
				return runDoctorValidationJSON()
			}
			paths := app.GetPathsWithConfig(globalConfig)
			exitCode := 0 // Track errors for exit code
			// Use globalConfig if available
			if globalConfig != nil {
				fmt.Println("AgentBin:", globalConfig.AgentBin())
				fmt.Println("SpecsSBI:", paths.SpecsSBI)
				fmt.Println("Timeout:", time.Duration(globalConfig.TimeoutSec())*time.Second)
			} else {
				fmt.Println("AgentBin: claude")
				fmt.Println("SpecsSBI:", paths.SpecsSBI)
				fmt.Println("Timeout: 60s")
			}
			fmt.Println("DeespecHome:", paths.Home)

			agentBin := "claude"
			if globalConfig != nil {
				agentBin = globalConfig.AgentBin()
			}
			if _, err := exec.LookPath(agentBin); err != nil {
				fmt.Printf("WARN: %s not found in PATH\n", agentBin)
			} else {
				fmt.Printf("OK: %s found\n", agentBin)
			}
			if err := os.MkdirAll(paths.SpecsSBI, 0o755); err != nil {
				return fmt.Errorf("specs/sbi dir error: %w", err)
			}
			probeFile := filepath.Join(paths.Var, ".probe")
			if f, err := os.Create(probeFile); err != nil {
				return fmt.Errorf("write check failed: %w", err)
			} else {
				f.Close()
				os.Remove(probeFile) // Clean up probe file
			}
			fmt.Println("OK: write permission in var dir")

			// Check workflow.yaml exists and validate
			if _, err := os.Stat(paths.Workflow); err != nil {
				fmt.Printf("WARN: workflow.yaml not found at %s\n", paths.Workflow)
			} else {
				// Try to load and validate workflow
				ctx := context.Background()
				wfPath := paths.Workflow
				// Check for config override
				if globalConfig != nil && globalConfig.Workflow() != "" {
					wfPath = globalConfig.Workflow()
				}
				wf, err := workflow.LoadWorkflow(ctx, wfPath)
				if err != nil {
					fmt.Printf("ERROR: workflow validation failed: %v\n", err)
				} else {
					// Display with supported agents and placeholders
					// Format agents list
					agentsList := fmt.Sprintf("%q", workflow.AllowedAgents[0])
					for i := 1; i < len(workflow.AllowedAgents); i++ {
						agentsList += fmt.Sprintf(",%q", workflow.AllowedAgents[i])
					}
					// Format placeholders list
					placeholdersList := fmt.Sprintf("%q", workflow.Allowed[0])
					for i := 1; i < len(workflow.Allowed); i++ {
						placeholdersList += fmt.Sprintf(",%q", workflow.Allowed[i])
					}
					// Get prompt size limit
					sizeLimit := wf.Constraints.MaxPromptKB
					if sizeLimit <= 0 {
						sizeLimit = workflow.DefaultMaxPromptKB
					}
					fmt.Printf("OK: workflow.yaml found and valid (prompt_path only; agents=[%s]; placeholders=[%s]; prompt_size_limit=%dKB)\n", agentsList, placeholdersList, sizeLimit)

					// Check for decision.regex on review step
					for _, step := range wf.Steps {
						if step.ID == "review" && step.CompiledDecision != nil {
							pattern := ""
							if step.Decision != nil {
								pattern = step.Decision.Regex
							}
							if pattern == "" {
								pattern = workflow.DefaultDecisionRegex
							}
							fmt.Printf("OK: decision.regex compiled for review (pattern='%s')\n", pattern)
							break
						}
					}

					// Check if prompt files exist, are readable, and pass validation (SBI-DR-001, SBI-DR-002)
					promptErrors := 0
					promptWarnings := 0
					promptOK := 0

					// Get max prompt size limit
					maxPromptKB := wf.Constraints.MaxPromptKB
					if maxPromptKB <= 0 {
						maxPromptKB = workflow.DefaultMaxPromptKB
					}

					for _, step := range wf.Steps {
						// Check existence
						fileInfo, err := os.Stat(step.ResolvedPromptPath)
						if err != nil {
							if os.IsNotExist(err) {
								Error("prompt_path not found: %s\n", step.ResolvedPromptPath)
							} else {
								Error("prompt_path not accessible: %s (%v)\n", step.ResolvedPromptPath, err)
							}
							promptErrors++
							continue
						}

						// Check it's a regular file
						if !fileInfo.Mode().IsRegular() {
							Error("prompt_path not a regular file: %s\n", step.ResolvedPromptPath)
							promptErrors++
							continue
						}

						// Check size (SBI-DR-002)
						fileSizeKB := (fileInfo.Size() + 1023) / 1024
						if fileSizeKB > int64(maxPromptKB) {
							Error("prompt_path (%s) exceeds max_prompt_kb=%d (found %d)\n", step.ID, maxPromptKB, fileSizeKB)
							promptErrors++
							continue
						}

						// Read file content for UTF-8 and format checks
						content, err := os.ReadFile(step.ResolvedPromptPath)
						if err != nil {
							Error("prompt_path not readable: %s (%v)\n", step.ResolvedPromptPath, err)
							promptErrors++
							continue
						}

						// Check UTF-8 validity (SBI-DR-002)
						if !utf8.Valid(content) {
							Error("prompt_path (%s) invalid UTF-8 encoding\n", step.ID)
							promptErrors++
							continue
						}

						// Check for BOM (SBI-DR-002)
						if len(content) >= 3 && bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
							fmt.Printf("WARN: prompt_path (%s) contains UTF-8 BOM\n", step.ID)
							promptWarnings++
						}

						// Check for CRLF (SBI-DR-002)
						if bytes.Contains(content, []byte("\r")) {
							fmt.Printf("WARN: prompt_path (%s) contains CRLF; prefer LF\n", step.ID)
							promptWarnings++
						}

						// Check for undefined/unknown placeholders (SBI-DR-003)
						placeholderErrors, placeholderWarnings := validatePlaceholders(string(content), step.ID)
						for _, err := range placeholderErrors {
							fmt.Fprintln(os.Stderr, err)
							promptErrors++
						}
						for _, warn := range placeholderWarnings {
							fmt.Println(warn)
							promptWarnings++
						}

						// Report OK for this step's prompt with details
						if len(placeholderErrors) == 0 {
							fmt.Printf("OK: prompt_path (%s) size=%dKB utf8=valid lf=ok placeholders=valid\n", step.ID, fileSizeKB)
							promptOK++
						}
					}

					// Print summary (SBI-DR-002)
					totalSteps := len(wf.Steps)
					fmt.Printf("SUMMARY: steps=%d ok=%d warn=%d error=%d\n", totalSteps, promptOK, promptWarnings, promptErrors)

					// Set exit code based on errors
					if promptErrors > 0 {
						exitCode = 1
					}
				}
			}

			// Check state.json exists and validate schema
			stateInfo := ""
			if err := checkStateJSON(paths.State); err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("INFO: state.json not found at %s (run 'deespec init' first)\n", paths.State)
				} else if strings.Contains(err.Error(), "WARN:") {
					fmt.Printf("%v\n", err)
				} else {
					fmt.Printf("ERROR: state.json validation failed: %v\n", err)
				}
			} else {
				fmt.Printf("OK: state.json found and valid at %s\n", paths.State)
				// Load state for summary display
				if data, err := os.ReadFile(paths.State); err == nil {
					var state map[string]interface{}
					if json.Unmarshal(data, &state) == nil {
						step := state["step"]
						turn := state["turn"]
						stateInfo = fmt.Sprintf("State: step=%v turn=%v", step, turn)
					}
				}
			}

			// Check health.json schema
			if err := checkHealthJSON(paths.Health); err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("INFO: health.json not found at %s\n", paths.Health)
				} else if strings.Contains(err.Error(), "WARN:") {
					fmt.Printf("%v\n", err)
				} else {
					fmt.Printf("ERROR: health.json validation failed: %v\n", err)
				}
			} else {
				fmt.Printf("OK: health.json found and valid")
			}

			// Check journal (INFO if not exists, not ERROR)
			if _, err := os.Stat(paths.Journal); err != nil {
				fmt.Printf("INFO: journal.ndjson not found (first run not executed yet)")
			} else {
				// Validate NDJSON format
				if err := checkJournalNDJSON(paths.Journal); err != nil {
					fmt.Printf("WARN: journal.ndjson format issue: %v\n", err)
				} else {
					fmt.Printf("OK: journal.ndjson found and valid at %s\n", paths.Journal)
				}
			}

			// Check review_policy.yaml exists
			policyPath := filepath.Join(paths.Policies, "review_policy.yaml")
			if _, err := os.Stat(policyPath); err != nil {
				fmt.Printf("INFO: review_policy.yaml not found at %s\n", policyPath)
			} else {
				fmt.Printf("OK: review_policy.yaml found at %s\n", policyPath)
			}

			// Check specs directories
			if err := checkWritable(paths.SpecsSBI); err != nil {
				fmt.Printf("WARN: specs/sbi directory not writable: %v\n", err)
			} else {
				fmt.Printf("OK: specs/sbi directory is writable")
			}

			if err := checkWritable(paths.SpecsPBI); err != nil {
				fmt.Printf("WARN: specs/pbi directory not writable: %v\n", err)
			} else {
				fmt.Printf("OK: specs/pbi directory is writable")
			}

			// Check optional templates (SBI-INIT-006 finalization)
			templatesDir := filepath.Join(paths.Home, "templates")
			if _, err := os.Stat(filepath.Join(templatesDir, "spec_feedback.yaml")); err != nil {
				fmt.Printf("INFO: spec_feedback.yaml not found (will use built-in template)")
			} else {
				fmt.Printf("OK: spec_feedback.yaml template found")
			}

			// Check takeover template (SBI-INIT-007 addition)
			if _, err := os.Stat(filepath.Join(templatesDir, "spec_takeover.yaml")); err != nil {
				fmt.Printf("INFO: spec_takeover.yaml not found (will use built-in template)")
			} else {
				fmt.Printf("OK: spec_takeover.yaml template found")
			}

			// Check SBI meta template schema
			if err := checkSBIMetaTemplate(filepath.Join(templatesDir, "spec_sbi_meta.yaml")); err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("INFO: spec_sbi_meta.yaml not found (will use built-in template)")
				} else {
					fmt.Printf("WARN: spec_sbi_meta.yaml template issue: %v\n", err)
				}
			} else {
				fmt.Printf("OK: spec_sbi_meta.yaml template schema valid")
			}

			// Check .gitignore for deespec block
			checkGitignore()

			// Check for scheduler (launchd/systemd)
			checkScheduler()

			// Print summary information
			fmt.Println("\n--- Summary ---")
			if stateInfo != "" {
				fmt.Println(stateInfo)
			}
			fmt.Println("Logic: ok = (last journal.error == \"\")")

			// Exit with appropriate code
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&format, "format", "", "Output format (json for CI integration)")
	return cmd
}

func checkGitignore() {
	gitignorePath := ".gitignore"
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content := string(data)
		if strings.Contains(content, "# >>> deespec v1") {
			fmt.Println("INFO: .gitignore deespec block present (v1)")
		} else {
			fmt.Println("INFO: .gitignore deespec block not found (recommended)")
		}
	} else if os.IsNotExist(err) {
		fmt.Println("INFO: .gitignore not found (will be created by 'deespec init')")
	} else {
		fmt.Printf("WARN: Cannot read .gitignore: %v\n", err)
	}
}

func checkScheduler() {
	switch runtime.GOOS {
	case "darwin":
		// Check launchd on macOS
		plistPath := os.ExpandEnv("$HOME/Library/LaunchAgents/com.deespec.runner.plist")
		if _, err := os.Stat(plistPath); err == nil {
			// Check if loaded
			cmd := exec.Command("launchctl", "list")
			output, _ := cmd.Output()
			if contains(string(output), "com.deespec.runner") {
				fmt.Println("OK: launchd service loaded (com.deespec.runner)")
			} else {
				fmt.Println("INFO: launchd plist exists but not loaded")
				fmt.Printf("  Run: launchctl load %s\n", plistPath)
			}
		} else {
			fmt.Println("INFO: launchd not configured")
			fmt.Println("  See: https://github.com/YoshitsuguKoike/deespec#5-min-loop")
		}
	case "linux":
		// Check systemd on Linux
		servicePath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.service")
		timerPath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.timer")

		if _, err := os.Stat(servicePath); err == nil {
			// Check timer status
			cmd := exec.Command("systemctl", "--user", "is-active", "deespec.timer")
			output, _ := cmd.Output()
			status := string(output)
			if status == "active\n" {
				fmt.Println("OK: systemd timer active (deespec.timer)")
			} else if _, err := os.Stat(timerPath); err == nil {
				fmt.Println("INFO: systemd timer exists but not active")
				fmt.Println("  Run: systemctl --user enable --now deespec.timer")
			} else {
				fmt.Println("INFO: systemd service exists but timer not configured")
			}
		} else {
			fmt.Println("INFO: systemd not configured")
			fmt.Println("  See: https://github.com/YoshitsuguKoike/deespec#5-min-loop")
		}
	default:
		fmt.Printf("INFO: Scheduler check not available for %s\n", runtime.GOOS)
	}
}

func checkWritable(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			// Try to create the directory
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("cannot create directory: %w", err)
			}
		} else {
			return fmt.Errorf("cannot access directory: %w", err)
		}
	}

	// Test write permission
	testFile := filepath.Join(dir, ".write_test")
	if f, err := os.Create(testFile); err != nil {
		return fmt.Errorf("not writable: %w", err)
	} else {
		f.Close()
		os.Remove(testFile)
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// validatePlaceholders checks for undefined or unknown placeholders in prompt content (SBI-DR-003)
func validatePlaceholders(content string, stepID string) (errors []string, warnings []string) {
	// Allowed placeholders from workflow.Allowed
	allowed := map[string]bool{
		"turn":         true,
		"task_id":      true,
		"project_name": true,
		"language":     true,
	}

	// Remove code blocks and inline code to avoid false positives
	processedContent := removeCodeBlocks(content)

	// Find all placeholders using regex
	doubleRe := regexp.MustCompile(`\{\{([^{}]+)\}\}`)

	// Check for mustache-style templates (warning only) and remove them
	doubleMatches := doubleRe.FindAllStringSubmatchIndex(processedContent, -1)
	for _, match := range doubleMatches {
		line := countLines(content[:match[0]]) + 1
		warnings = append(warnings, fmt.Sprintf("WARN: prompt_path (%s) contains non-standard {{%s}} at line %d",
			stepID, processedContent[match[2]:match[3]], line))
	}

	// Remove double brace placeholders before checking single braces
	processedContent = doubleRe.ReplaceAllString(processedContent, "")

	// Now check single brace placeholders
	placeholderRe := regexp.MustCompile(`\{([^{}]*)\}`)
	matches := placeholderRe.FindAllStringSubmatchIndex(processedContent, -1)
	for _, match := range matches {
		placeholder := strings.TrimSpace(processedContent[match[2]:match[3]])
		line := countLines(content[:match[0]]) + 1

		// Check for empty placeholder
		if placeholder == "" {
			errors = append(errors, fmt.Sprintf("ERROR: prompt_path (%s) contains empty placeholder {} at line %d",
				stepID, line))
			continue
		}

		// Check if placeholder is in allowed list
		if !allowed[placeholder] {
			// Check if it's a valid identifier (alphanumeric and underscore)
			if isValidIdentifier(placeholder) {
				errors = append(errors, fmt.Sprintf("ERROR: prompt_path (%s) unknown placeholder {%s} at line %d",
					stepID, placeholder, line))
			} else {
				errors = append(errors, fmt.Sprintf("ERROR: prompt_path (%s) invalid placeholder {%s} at line %d",
					stepID, placeholder, line))
			}
		}
	}

	return errors, warnings
}

// validatePlaceholdersJSON checks for undefined or unknown placeholders and returns JSON format issues
func validatePlaceholdersJSON(content string, stepID string) (errors []DoctorIssueJSON, warnings []DoctorIssueJSON) {
	// Allowed placeholders from workflow.Allowed
	allowed := map[string]bool{
		"turn":         true,
		"task_id":      true,
		"project_name": true,
		"language":     true,
	}

	// Remove code blocks and inline code to avoid false positives
	processedContent := removeCodeBlocks(content)

	// Find all placeholders using regex
	doubleRe := regexp.MustCompile(`\{\{([^{}]+)\}\}`)

	// Check for mustache-style templates (warning only) and remove them
	doubleMatches := doubleRe.FindAllStringSubmatchIndex(processedContent, -1)
	for _, match := range doubleMatches {
		line := countLines(content[:match[0]]) + 1
		warnings = append(warnings, DoctorIssueJSON{
			Type:    "warn",
			Line:    line,
			Message: fmt.Sprintf("contains non-standard {{%s}}", processedContent[match[2]:match[3]]),
		})
	}

	// Remove double brace placeholders before checking single braces
	processedContent = doubleRe.ReplaceAllString(processedContent, "")

	// Now check single brace placeholders
	placeholderRe := regexp.MustCompile(`\{([^{}]*)\}`)
	matches := placeholderRe.FindAllStringSubmatchIndex(processedContent, -1)
	for _, match := range matches {
		placeholder := strings.TrimSpace(processedContent[match[2]:match[3]])
		line := countLines(content[:match[0]]) + 1

		// Check for empty placeholder
		if placeholder == "" {
			errors = append(errors, DoctorIssueJSON{
				Type:    "error",
				Line:    line,
				Message: "empty placeholder {}",
			})
			continue
		}

		// Check if placeholder is in allowed list
		if !allowed[placeholder] {
			// Check if it's a valid identifier (alphanumeric and underscore)
			if isValidIdentifier(placeholder) {
				errors = append(errors, DoctorIssueJSON{
					Type:    "error",
					Line:    line,
					Message: fmt.Sprintf("unknown placeholder {%s}", placeholder),
				})
			} else {
				errors = append(errors, DoctorIssueJSON{
					Type:    "error",
					Line:    line,
					Message: fmt.Sprintf("invalid placeholder {%s}", placeholder),
				})
			}
		}
	}

	return errors, warnings
}

// removeCodeBlocks removes fenced code blocks and inline code to prevent false positives
func removeCodeBlocks(content string) string {
	// Remove fenced code blocks (```...```)
	fencedRe := regexp.MustCompile("(?s)```[^`]*```")
	content = fencedRe.ReplaceAllString(content, "")

	// Remove inline code (`...`)
	inlineRe := regexp.MustCompile("`[^`]+`")
	content = inlineRe.ReplaceAllString(content, "")

	// Remove escaped braces \{ and \}
	content = strings.ReplaceAll(content, `\{`, "")
	content = strings.ReplaceAll(content, `\}`, "")

	return content
}

// countLines counts the number of lines (newlines) in a string
func countLines(s string) int {
	return strings.Count(s, "")
}

// isValidIdentifier checks if a string is a valid identifier (alphanumeric and underscore)
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func runDoctorJSON() error {
	paths := app.GetPathsWithConfig(globalConfig)
	agentBin := "claude"
	if globalConfig != nil {
		agentBin = globalConfig.AgentBin()
	}
	result := DoctorJSON{
		Runner:       "none",
		Active:       false,
		WorkingDir:   "",
		AgentBin:     agentBin,
		Errors:       []string{},
		ConfigSource: "",
		SettingPath:  "",
	}

	// Add config source information if available
	if globalConfig != nil {
		result.ConfigSource = globalConfig.ConfigSource()
		result.SettingPath = globalConfig.SettingPath()
	}

	// Load transaction metrics
	metricsPath := filepath.Join(paths.Var, "metrics.json")
	if metrics, err := txn.LoadMetrics(metricsPath); err == nil {
		snapshot := metrics.GetSnapshot()
		result.Metrics = &DoctorMetricsJSON{
			CommitSuccess: snapshot.CommitSuccess,
			CommitFailed:  snapshot.CommitFailed,
			CASConflicts:  snapshot.CASConflicts,
			RecoveryCount: snapshot.RecoveryCount,
			TotalCommits:  metrics.GetTotalCommits(),
			SuccessRate:   metrics.GetSuccessRate(),
			LastUpdate:    snapshot.LastUpdate,
		}
	}

	// Check working directory
	if wd, err := os.Getwd(); err == nil {
		result.WorkingDir = wd
		// Check write permission in var directory
		if err := os.MkdirAll(paths.Var, 0755); err == nil {
			probeFile := filepath.Join(paths.Var, ".probe")
			if f, err := os.Create(probeFile); err != nil {
				result.Errors = append(result.Errors, "var_dir not writable")
			} else {
				f.Close()
				os.Remove(probeFile)
			}
		} else {
			result.Errors = append(result.Errors, "cannot create var directory")
		}
	}

	// Check agent binary
	if _, err := exec.LookPath(agentBin); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("agent_bin '%s' not found", agentBin))
	}

	// Check scheduler based on OS
	switch runtime.GOOS {
	case "darwin":
		checkLaunchdJSON(&result)
	case "linux":
		checkSystemdJSON(&result)
	}

	// Determine exit code
	exitCode := 0
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			if strings.Contains(err, "not writable") || strings.Contains(err, "not found") {
				exitCode = 1
				break
			}
		}
		if exitCode == 0 {
			exitCode = 2
		}
	} else if !result.Active {
		exitCode = 2
	}

	// Output JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	if err := enc.Encode(result); err != nil {
		return err
	}

	os.Exit(exitCode)
	return nil
}

func runDoctorValidationJSON() error {
	paths := app.GetPathsWithConfig(globalConfig)

	// Check workflow.yaml exists and validate
	if _, err := os.Stat(paths.Workflow); err != nil {
		// If workflow not found, return error
		result := DoctorValidationJSON{
			Steps:   []DoctorStepJSON{},
			Summary: DoctorSummaryJSON{Steps: 0, OK: 0, Warn: 0, Error: 1},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR: failed to encode JSON response:", err)
			os.Exit(1)
		}
		os.Exit(1)
		return nil
	}

	// Load and validate workflow
	ctx := context.Background()
	wfPath := paths.Workflow
	if globalConfig != nil && globalConfig.Workflow() != "" {
		wfPath = globalConfig.Workflow()
	}

	wf, err := workflow.LoadWorkflow(ctx, wfPath)
	if err != nil {
		// Workflow loading failed
		result := DoctorValidationJSON{
			Steps:   []DoctorStepJSON{},
			Summary: DoctorSummaryJSON{Steps: 0, OK: 0, Warn: 0, Error: 1},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR: failed to encode JSON response:", err)
			os.Exit(1)
		}
		os.Exit(1)
		return nil
	}

	// Process each step's prompt validation
	var steps []DoctorStepJSON
	totalWarnings := 0
	totalErrors := 0
	totalOK := 0

	// Get max prompt size limit
	maxPromptKB := wf.Constraints.MaxPromptKB
	if maxPromptKB <= 0 {
		maxPromptKB = workflow.DefaultMaxPromptKB
	}

	for _, step := range wf.Steps {
		stepResult := DoctorStepJSON{
			ID:     step.ID,
			Path:   step.ResolvedPromptPath,
			Issues: []DoctorIssueJSON{},
		}

		stepHasError := false

		// Check existence
		fileInfo, err := os.Stat(step.ResolvedPromptPath)
		if err != nil {
			if os.IsNotExist(err) {
				stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
					Type:    "error",
					Message: "prompt_path not found",
				})
			} else {
				stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
					Type:    "error",
					Message: fmt.Sprintf("prompt_path not accessible (%v)", err),
				})
			}
			stepHasError = true
			totalErrors++
		} else if !fileInfo.Mode().IsRegular() {
			stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
				Type:    "error",
				Message: "prompt_path not a regular file",
			})
			stepHasError = true
			totalErrors++
		} else {
			// File exists and is regular, continue validation

			// Check size
			fileSizeKB := (fileInfo.Size() + 1023) / 1024
			if fileSizeKB > int64(maxPromptKB) {
				stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
					Type:    "error",
					Message: fmt.Sprintf("exceeds max_prompt_kb=%d (found %d)", maxPromptKB, fileSizeKB),
				})
				stepHasError = true
				totalErrors++
			}

			// Read file content for UTF-8 and format checks
			content, err := os.ReadFile(step.ResolvedPromptPath)
			if err != nil {
				stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
					Type:    "error",
					Message: fmt.Sprintf("not readable (%v)", err),
				})
				stepHasError = true
				totalErrors++
			} else {
				// Check UTF-8 validity
				if !utf8.Valid(content) {
					stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
						Type:    "error",
						Message: "invalid UTF-8 encoding",
					})
					stepHasError = true
					totalErrors++
				}

				// Check for BOM (warning)
				if len(content) >= 3 && bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
					stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
						Type:    "warn",
						Message: "contains UTF-8 BOM",
					})
					totalWarnings++
				}

				// Check for CRLF (warning)
				if bytes.Contains(content, []byte("\r")) {
					stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
						Type:    "warn",
						Message: "contains CRLF",
					})
					totalWarnings++
				}

				// Check placeholders
				placeholderErrors, placeholderWarnings := validatePlaceholdersJSON(string(content), step.ID)
				for _, issue := range placeholderErrors {
					stepResult.Issues = append(stepResult.Issues, issue)
					stepHasError = true
					totalErrors++
				}
				for _, issue := range placeholderWarnings {
					stepResult.Issues = append(stepResult.Issues, issue)
					totalWarnings++
				}
			}
		}

		// Add OK status if no errors for this step
		if !stepHasError {
			stepResult.Issues = append(stepResult.Issues, DoctorIssueJSON{
				Type:    "ok",
				Message: fmt.Sprintf("size=%dKB utf8=valid lf=ok placeholders=valid", (fileInfo.Size()+1023)/1024),
			})
			totalOK++
		}

		steps = append(steps, stepResult)
	}

	// Create result
	result := DoctorValidationJSON{
		Steps: steps,
		Summary: DoctorSummaryJSON{
			Steps: len(wf.Steps),
			OK:    totalOK,
			Warn:  totalWarnings,
			Error: totalErrors,
		},
	}

	// Output JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return err
	}

	// Exit with appropriate code
	if totalErrors > 0 {
		os.Exit(1)
	}
	return nil
}

func checkLaunchdJSON(result *DoctorJSON) {
	plistPath := os.ExpandEnv("$HOME/Library/LaunchAgents/com.deespec.runner.plist")

	if _, err := os.Stat(plistPath); err == nil {
		result.Runner = "launchd"

		// Check if loaded
		cmd := exec.Command("launchctl", "list")
		output, _ := cmd.Output()
		if contains(string(output), "com.deespec.runner") {
			result.Active = true
		}

		// Extract StartInterval from plist
		cmd = exec.Command("plutil", "-p", plistPath)
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "")
			for _, line := range lines {
				if strings.Contains(line, "StartInterval") {
					parts := strings.Split(line, "=>")
					if len(parts) == 2 {
						if val, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
							result.StartIntervalSec = val
						}
					}
				}
				if strings.Contains(line, "WorkingDirectory") {
					// Extract value after => and remove quotes
					parts := strings.Split(line, "=>")
					if len(parts) == 2 {
						val := strings.TrimSpace(parts[1])
						val = strings.Trim(val, "\"")
						result.WorkingDir = val
					}
				}
			}
		}
	}
}

func checkSystemdJSON(result *DoctorJSON) {
	servicePath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.service")
	timerPath := os.ExpandEnv("$HOME/.config/systemd/user/deespec.timer")

	if _, err := os.Stat(servicePath); err == nil {
		result.Runner = "systemd"

		// Check timer status
		cmd := exec.Command("systemctl", "--user", "is-active", "deespec.timer")
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) == "active" {
			result.Active = true
		}

		// Try to get timer info
		if _, err := os.Stat(timerPath); err == nil {
			// Default to 5 minutes for systemd timer
			result.StartIntervalSec = 300
		}
	}
}

// checkStateJSON validates the state.json schema
func checkStateJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	// Check required fields
	if _, ok := state["version"].(float64); !ok {
		return fmt.Errorf("missing or invalid 'version' field (must be number)")
	}

	// Check for step field (v1 uses step, not current)
	if _, hasStep := state["step"].(string); !hasStep {
		if _, hasCurrent := state["current"]; hasCurrent {
			return fmt.Errorf(".deespec/var/state.json uses old 'current' field - should use 'step' (e.g., \"plan\")")
		}
		return fmt.Errorf("missing 'step' field (e.g., \"plan\")")
	}

	// Validate step value (WARN level for invalid values)
	step := state["step"].(string)
	validSteps := map[string]bool{
		"plan": true, "implement": true, "test": true, "review": true, "done": true,
	}
	if !validSteps[step] {
		// Return a special error type to indicate warning level
		return fmt.Errorf("WARN: .deespec/var/state.json has invalid step '%s' (expected: plan|implement|test|review|done)", step)
	}

	if _, ok := state["turn"].(float64); !ok {
		return fmt.Errorf("missing or invalid 'turn' field (must be number)")
	}

	return nil
}

// checkHealthJSON validates the health.json schema
func checkHealthJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var health map[string]interface{}
	if err := json.Unmarshal(data, &health); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	// Check required fields
	requiredFields := []string{"ts", "turn", "step", "ok", "error"}
	for _, field := range requiredFields {
		if _, ok := health[field]; !ok {
			return fmt.Errorf("missing required field '%s'", field)
		}
	}

	// Validate types
	tsStr, ok := health["ts"].(string)
	if !ok {
		return fmt.Errorf("'ts' must be a string")
	}

	// Check timestamp format (should be RFC3339-ish with Z ending)
	if !strings.HasSuffix(tsStr, "Z") {
		return fmt.Errorf("WARN: health.ts should be in UTC (RFC3339Nano format ending with 'Z')")
	}
	// Check for nanosecond precision
	if !strings.Contains(tsStr, ".") && !strings.Contains(tsStr, "T") {
		return fmt.Errorf("WARN: health.ts should use RFC3339Nano precision")
	}

	if _, ok := health["turn"].(float64); !ok {
		return fmt.Errorf("'turn' must be a number")
	}
	if _, ok := health["step"].(string); !ok {
		return fmt.Errorf("'step' must be a string")
	}
	if _, ok := health["ok"].(bool); !ok {
		return fmt.Errorf("'ok' must be a boolean")
	}
	if _, ok := health["error"].(string); !ok {
		return fmt.Errorf("'error' must be a string")
	}

	return nil
}

// checkJournalNDJSON validates NDJSON format
func checkJournalNDJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil // Empty journal is valid
	}

	lines := strings.Split(string(data), "")
	for i, line := range lines {
		if line == "" {
			continue // Skip empty lines
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("invalid JSON at line %d: %w", i+1, err)
		}
	}

	return nil
}

// checkSBIMetaTemplate validates the SBI meta.yaml template schema
func checkSBIMetaTemplate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var meta map[string]interface{}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("invalid YAML format: %w", err)
	}

	// Check required fields for SBI meta template
	requiredFields := []string{"id", "title", "priority", "status", "pbi_id"}
	for _, field := range requiredFields {
		if _, ok := meta[field]; !ok {
			return fmt.Errorf("missing required field '%s'", field)
		}
	}

	return nil
}
