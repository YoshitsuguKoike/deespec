package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// IncompleteReason represents the reason for incomplete instruction
type IncompleteReason string

const (
	DepUnresolved IncompleteReason = "DEP_UNRESOLVED"
	DepCycle      IncompleteReason = "DEP_CYCLE"
	MetaMissing   IncompleteReason = "META_MISSING"
	PathInvalid   IncompleteReason = "PATH_INVALID"
	PromptError   IncompleteReason = "PROMPT_ERROR"
	TimeFormat    IncompleteReason = "TIME_FORMAT"
	JournalGuard  IncompleteReason = "JOURNAL_GUARD"
)

// FBDraft represents a feedback SBI draft for incomplete instruction
type FBDraft struct {
	TargetTaskID  string           `json:"target_task_id"`
	ReasonCode    IncompleteReason `json:"reason_code"`
	Title         string           `json:"title"`
	Summary       string           `json:"summary"`
	EvidencePaths []string         `json:"evidence_paths"`
	SuggestedFBID string           `json:"suggested_fb_id"`
	CreatedAt     time.Time        `json:"-"` // Will be formatted as RFC3339Nano
}

// FBDraftYAML represents the YAML structure for draft.yaml
type FBDraftYAML struct {
	ID         string   `yaml:"id"`
	Title      string   `yaml:"title"`
	Labels     []string `yaml:"labels"`
	Por        int      `yaml:"por"`
	Priority   int      `yaml:"priority"`
	RelatesTo  string   `yaml:"relates_to"`
	ReasonCode string   `yaml:"reason_code"`
	Details    string   `yaml:"details"`
}

// PickContext provides context for incomplete detection
type PickContext struct {
	JournalPath    string
	CompletedTasks map[string]bool
	AllTasks       []*Task
}

// DetectIncomplete checks for incomplete instructions in a task
func DetectIncomplete(t *Task, ctx *PickContext) ([]FBDraft, error) {
	var drafts []FBDraft

	// Check for unresolved dependencies
	dependsOn := []string{}
	if deps, ok := t.Meta["depends_on"].([]string); ok {
		dependsOn = deps
	} else if deps, ok := t.Meta["depends_on"].([]interface{}); ok {
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				dependsOn = append(dependsOn, depStr)
			}
		}
	}

	if len(dependsOn) > 0 {
		for _, depID := range dependsOn {
			if !ctx.CompletedTasks[depID] {
				draft := FBDraft{
					TargetTaskID: t.ID,
					ReasonCode:   DepUnresolved,
					Title:        fmt.Sprintf("【FB】%s の不完全指示修正", t.ID),
					Summary:      fmt.Sprintf("依存未解決: depends_on=[%s]（未完了）", depID),
					CreatedAt:    time.Now().UTC(),
				}
				drafts = append(drafts, draft)
			}
		}
	}

	// Check for cyclic dependencies
	if detectTaskCycle(t, ctx.AllTasks) {
		draft := FBDraft{
			TargetTaskID: t.ID,
			ReasonCode:   DepCycle,
			Title:        fmt.Sprintf("【FB】%s の不完全指示修正", t.ID),
			Summary:      fmt.Sprintf("循環依存検出: %s が依存グラフにサイクルを形成", t.ID),
			CreatedAt:    time.Now().UTC(),
		}
		drafts = append(drafts, draft)
	}

	// Check for missing meta fields
	if t.ID == "" || t.Title == "" {
		draft := FBDraft{
			TargetTaskID: t.ID,
			ReasonCode:   MetaMissing,
			Title:        fmt.Sprintf("【FB】%s の不完全指示修正", t.ID),
			Summary:      "メタ情報不備: 必須フィールド(id/title)が欠落",
			CreatedAt:    time.Now().UTC(),
		}
		drafts = append(drafts, draft)
	}

	// Check for invalid paths
	if containsInvalidPath(t.PromptPath) {
		draft := FBDraft{
			TargetTaskID: t.ID,
			ReasonCode:   PathInvalid,
			Title:        fmt.Sprintf("【FB】%s の不完全指示修正", t.ID),
			Summary:      fmt.Sprintf("パス異常: %s に絶対パスまたは親ディレクトリ参照が含まれる", t.PromptPath),
			CreatedAt:    time.Now().UTC(),
		}
		drafts = append(drafts, draft)
	}

	// Check for prompt errors (size, placeholders)
	if t.PromptPath != "" {
		if info, err := os.Stat(t.PromptPath); err == nil && info.Size() > 100*1024 { // 100KB limit
			draft := FBDraft{
				TargetTaskID: t.ID,
				ReasonCode:   PromptError,
				Title:        fmt.Sprintf("【FB】%s の不完全指示修正", t.ID),
				Summary:      fmt.Sprintf("プロンプトサイズ超過: %d bytes (制限: 100KB)", info.Size()),
				CreatedAt:    time.Now().UTC(),
			}
			drafts = append(drafts, draft)
		}
	}

	return drafts, nil
}

// detectTaskCycle checks if a task creates a cycle in dependency graph
func detectTaskCycle(task *Task, allTasks []*Task) bool {
	// Build task map for quick lookup
	taskMap := make(map[string]*Task)
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	// Use DFS to detect cycle starting from this task
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	return hasCycleDFS(task.ID, taskMap, visited, recStack)
}

// hasCycleDFS performs depth-first search to detect cycles
func hasCycleDFS(taskID string, taskMap map[string]*Task, visited, recStack map[string]bool) bool {
	visited[taskID] = true
	recStack[taskID] = true

	task, exists := taskMap[taskID]
	if !exists {
		recStack[taskID] = false
		return false
	}

	// Extract depends_on from meta
	dependsOn := []string{}
	if deps, ok := task.Meta["depends_on"].([]string); ok {
		dependsOn = deps
	} else if deps, ok := task.Meta["depends_on"].([]interface{}); ok {
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				dependsOn = append(dependsOn, depStr)
			}
		}
	}

	for _, depID := range dependsOn {
		if !visited[depID] {
			if hasCycleDFS(depID, taskMap, visited, recStack) {
				return true
			}
		} else if recStack[depID] {
			return true
		}
	}

	recStack[taskID] = false
	return false
}

// containsInvalidPath checks for absolute paths or parent directory references
func containsInvalidPath(path string) bool {
	if path == "" {
		return false
	}

	// Check for absolute paths
	if filepath.IsAbs(path) {
		return true
	}

	// Check for parent directory references
	if strings.Contains(path, "..") {
		return true
	}

	// Check for Windows-style paths
	if strings.Contains(path, "\\") || strings.Contains(path, "C:") {
		return true
	}

	return false
}

// PersistFBDraft saves the FB draft to artifacts
func PersistFBDraft(d FBDraft, artifactsDir string) (string, error) {
	// Create fb_sbi directory structure
	fbDir := filepath.Join(artifactsDir, "fb_sbi", d.TargetTaskID)
	if err := os.MkdirAll(fbDir, 0755); err != nil {
		return "", fmt.Errorf("create fb_sbi dir: %w", err)
	}

	// Write context.md
	contextPath := filepath.Join(fbDir, "context.md")
	contextContent := fmt.Sprintf(`# 不完全指示検出レポート

## 対象タスク
- ID: %s
- 理由コード: %s

## 状況説明
%s

## 検出時刻
%s

## 再現手順
1. タスク %s のピック/再開を試行
2. 不完全指示の条件 (%s) を検出
3. 本ドラフトを自動生成

## 推奨対応
- 依存関係の見直し
- メタ情報の修正
- パス/プロンプトの調整
`, d.TargetTaskID, d.ReasonCode, d.Summary,
		d.CreatedAt.Format(time.RFC3339Nano), d.TargetTaskID, d.ReasonCode)

	if err := os.WriteFile(contextPath, []byte(contextContent), 0644); err != nil {
		return "", fmt.Errorf("write context.md: %w", err)
	}

	// Write evidence.txt
	evidencePath := filepath.Join(fbDir, "evidence.txt")
	evidenceContent := fmt.Sprintf(`Task ID: %s
Reason Code: %s
Summary: %s
Created At: %s
`, d.TargetTaskID, d.ReasonCode, d.Summary, d.CreatedAt.Format(time.RFC3339Nano))

	if err := os.WriteFile(evidencePath, []byte(evidenceContent), 0644); err != nil {
		return "", fmt.Errorf("write evidence.txt: %w", err)
	}

	// Write draft.yaml
	draftPath := filepath.Join(fbDir, "draft.yaml")
	draftYAML := FBDraftYAML{
		ID:         "", // Will be assigned on registration
		Title:      d.Title,
		Labels:     []string{"feedback", "pick", "sbi-fb"},
		Por:        1,
		Priority:   1,
		RelatesTo:  d.TargetTaskID,
		ReasonCode: string(d.ReasonCode),
		Details: fmt.Sprintf(`- 理由: %s
- 対象タスク: %s
- 検出時刻: %s
- 再現手順:
  1. タスクのピック/再開を試行
  2. 不完全指示を検出
  3. 本ドラフトを生成
- 期待: 依存の整理 or 優先度変更 or 分解`,
			d.Summary, d.TargetTaskID, d.CreatedAt.Format(time.RFC3339Nano)),
	}

	yamlData, err := yaml.Marshal(&draftYAML)
	if err != nil {
		return "", fmt.Errorf("marshal draft.yaml: %w", err)
	}

	if err := os.WriteFile(draftPath, yamlData, 0644); err != nil {
		return "", fmt.Errorf("write draft.yaml: %w", err)
	}

	// Store evidence paths
	d.EvidencePaths = []string{contextPath, evidencePath, draftPath}

	return draftPath, nil
}

// RecordFBDraftInJournal adds FB draft record to journal
func RecordFBDraftInJournal(d FBDraft, journalPath string, turn int) error {
	artifact := map[string]interface{}{
		"type":            "fb_sbi_draft",
		"target_task_id":  d.TargetTaskID,
		"reason_code":     string(d.ReasonCode),
		"title":           d.Title,
		"summary":         d.Summary,
		"evidence_paths":  d.EvidencePaths,
		"suggested_fb_id": d.SuggestedFBID,
		"created_at":      d.CreatedAt.Format(time.RFC3339Nano),
	}

	record := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       "plan", // FB drafts are recorded during planning
		"decision":   "",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts":  []interface{}{artifact},
	}

	// Use atomic append
	file, err := os.OpenFile(journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open journal: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal journal record: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write journal record: %w", err)
	}

	return nil
}

// HandleAutoFBRegistration checks for FB drafts and automatically registers them
func HandleAutoFBRegistration(journalPath string, turn int) error {
	// Check for recent fb_sbi_draft entries in journal
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return nil // No journal yet
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Look for fb_sbi_draft artifacts that haven't been registered
		if artifacts, ok := entry["artifacts"].([]interface{}); ok {
			for _, artifact := range artifacts {
				if artifactMap, ok := artifact.(map[string]interface{}); ok {
					if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "fb_sbi_draft" {
						// Check if already registered
						if !isAlreadyRegistered(artifactMap["target_task_id"].(string), journalPath) {
							// Register the draft
							if err := registerFBDraft(artifactMap, journalPath, turn); err != nil {
								fmt.Fprintf(os.Stderr, "WARN: Failed to auto-register FB draft: %v\n", err)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// isAlreadyRegistered checks if an FB draft was already registered
func isAlreadyRegistered(targetTaskID string, journalPath string) bool {
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Look for fb_sbi_registered artifacts
		if artifacts, ok := entry["artifacts"].([]interface{}); ok {
			for _, artifact := range artifacts {
				if artifactMap, ok := artifact.(map[string]interface{}); ok {
					if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "fb_sbi_registered" {
						if targetID, ok := artifactMap["target_task_id"].(string); ok && targetID == targetTaskID {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// registerFBDraft simulates registration of an FB draft
func registerFBDraft(draftInfo map[string]interface{}, journalPath string, turn int) error {
	targetTaskID, _ := draftInfo["target_task_id"].(string)

	fmt.Fprintf(os.Stderr, "INFO: Auto-registering FB draft for %s\n", targetTaskID)

	// In a real implementation, this would call:
	// cat artifacts/fb_sbi/<target-id>/draft.yaml | deespec sbi register --stdin
	// For now, we simulate the registration

	// Record registration in journal
	registeredArtifact := map[string]interface{}{
		"type":           "fb_sbi_registered",
		"target_task_id": targetTaskID,
		"fb_id":          fmt.Sprintf("SBI-FB-%03d", turn), // Simulated ID
		"registered_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}

	record := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       "plan",
		"decision":   "",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts":  []interface{}{registeredArtifact},
	}

	// Append to journal
	file, err := os.OpenFile(journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open journal: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal journal record: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write journal record: %w", err)
	}

	fmt.Fprintf(os.Stderr, "INFO: FB draft registered as %s\n", registeredArtifact["fb_id"])
	return nil
}
