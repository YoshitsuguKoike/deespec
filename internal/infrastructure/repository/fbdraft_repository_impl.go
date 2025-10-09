package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"gopkg.in/yaml.v3"
)

// FBDraftRepositoryImpl implements FBDraftRepository for file-based storage
type FBDraftRepositoryImpl struct{}

// NewFBDraftRepositoryImpl creates a new file-based FBDraft repository
func NewFBDraftRepositoryImpl() repository.FBDraftRepository {
	return &FBDraftRepositoryImpl{}
}

// PersistDraft saves the FB draft to SBI directory
func (r *FBDraftRepositoryImpl) PersistDraft(ctx context.Context, draft dto.FBDraft, sbiDir string) (string, error) {
	// Ensure SBI directory exists
	if err := os.MkdirAll(sbiDir, 0755); err != nil {
		return "", fmt.Errorf("create SBI dir: %w", err)
	}

	// Parse CreatedAt
	createdAt, err := time.Parse(time.RFC3339Nano, draft.CreatedAt)
	if err != nil {
		createdAt = time.Now().UTC()
	}

	// Write fb_context.md
	contextPath := filepath.Join(sbiDir, "fb_context.md")
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
`, draft.TargetTaskID, draft.ReasonCode, draft.Summary,
		createdAt.Format(time.RFC3339Nano), draft.TargetTaskID, draft.ReasonCode)

	if err := os.WriteFile(contextPath, []byte(contextContent), 0644); err != nil {
		return "", fmt.Errorf("write context.md: %w", err)
	}

	// Write fb_evidence.txt
	evidencePath := filepath.Join(sbiDir, "fb_evidence.txt")
	evidenceContent := fmt.Sprintf(`Task ID: %s
Reason Code: %s
Summary: %s
Created At: %s
`, draft.TargetTaskID, draft.ReasonCode, draft.Summary, createdAt.Format(time.RFC3339Nano))

	if err := os.WriteFile(evidencePath, []byte(evidenceContent), 0644); err != nil {
		return "", fmt.Errorf("write evidence.txt: %w", err)
	}

	// Write fb_draft.yaml
	draftPath := filepath.Join(sbiDir, "fb_draft.yaml")
	draftYAML := map[string]interface{}{
		"id":          "", // Will be assigned on registration
		"title":       draft.Title,
		"labels":      []string{"feedback", "pick", "sbi-fb"},
		"por":         1,
		"priority":    1,
		"relates_to":  draft.TargetTaskID,
		"reason_code": string(draft.ReasonCode),
		"details": fmt.Sprintf(`- 理由: %s
- 対象タスク: %s
- 検出時刻: %s
- 再現手順:
  1. タスクのピック/再開を試行
  2. 不完全指示を検出
  3. 本ドラフトを生成
- 期待: 依存の整理 or 優先度変更 or 分解`,
			draft.Summary, draft.TargetTaskID, createdAt.Format(time.RFC3339Nano)),
	}

	yamlData, err := yaml.Marshal(&draftYAML)
	if err != nil {
		return "", fmt.Errorf("marshal draft.yaml: %w", err)
	}

	if err := os.WriteFile(draftPath, yamlData, 0644); err != nil {
		return "", fmt.Errorf("write draft.yaml: %w", err)
	}

	return draftPath, nil
}

// RecordDraftInJournal adds FB draft record to journal
func (r *FBDraftRepositoryImpl) RecordDraftInJournal(ctx context.Context, draft dto.FBDraft, journalPath string, turn int) error {
	artifact := map[string]interface{}{
		"type":            "fb_sbi_draft",
		"target_task_id":  draft.TargetTaskID,
		"reason_code":     string(draft.ReasonCode),
		"title":           draft.Title,
		"summary":         draft.Summary,
		"evidence_paths":  draft.EvidencePaths,
		"suggested_fb_id": draft.SuggestedFBID,
		"created_at":      draft.CreatedAt,
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

// IsAlreadyRegistered checks if an FB draft was already registered
func (r *FBDraftRepositoryImpl) IsAlreadyRegistered(ctx context.Context, targetTaskID string, journalPath string) (bool, error) {
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return false, nil // No journal yet
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
							return true, nil
						}
					}
				}
			}
		}
	}

	return false, nil
}
