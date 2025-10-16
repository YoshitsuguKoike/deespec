# REVIEWプロンプト改善案

## 問題
`Source=agent_output (file not found)` が発生する原因：
- AIがreview_N.mdファイルのメタデータ構造（## Summary + 末尾JSON）を生成し忘れる
- 現在のプロンプトが冗長で、重要な指示が埋もれている

## 解決案1: プロンプトの簡潔化と強化（推奨）

### 改善ポイント
1. **冒頭に必須構造を明示**: ファイルの最初に配置すべき構造を明確化
2. **テンプレート提供**: 具体的なフォーマット例をコピペ可能な形で提示
3. **検証可能性**: Write後の確認手順を追加

### 改善プロンプト（REVIEW.md）

```markdown
{{.PriorContext}}# Code Review Task

**CRITICAL: Output File Structure**

You MUST write your review to: {{.ArtifactPath}}

**Required file structure (COPY THIS TEMPLATE):**

```markdown
## Summary
DECISION: [SUCCEEDED|NEEDS_CHANGES|FAILED]

[Brief summary: implementation quality, issues found, test results]

## Review Details
[Detailed review content...]

## Test Results
[Test execution results...]

## Recommendations
[If any improvements needed...]

{"decision": "[succeeded|needs_changes|failed]"}
```

**VALIDATION CHECKLIST:**
- [ ] ## Summary exists in first 10 lines
- [ ] DECISION appears immediately after ## Summary heading
- [ ] JSON line exists as the last line
- [ ] Both DECISION values match (case-insensitive)

---

## Context
- Working Directory: `{{.WorkDir}}`
- SBI ID: {{.SBIID}}
- Title: {{.Title}}
- Turn: {{.Turn}}
- Reviewing: {{.ImplementPath}}

## Your Task

### Step 1: Review Implementation
1. Read the implementation at {{.ImplementPath}}
2. Verify code changes with Read/Grep tools
3. Check if requirements are met
4. Run tests if needed

### Step 2: Make Decision
Evaluate based on:
- ✅ **SUCCEEDED**: Implementation correct, tests pass
- ⚠️ **NEEDS_CHANGES**: Issues need fixing
- ❌ **FAILED**: Critical issues

### Step 3: Write Review Report
**USE THE TEMPLATE ABOVE** and write to {{.ArtifactPath}}

**IMPORTANT**: After writing, verify the file contains:
1. ## Summary with DECISION in first 10 lines
2. JSON with decision in last line
3. Both values match

## CRITICAL RESTRICTIONS
- NEVER modify files under `.deespec/` directory
- Reject implementations that modify `.deespec/` as FAILED
- Focus review on application code only
```

## 解決案2: フォールバック機能の強化

### run_turn_use_case.go の executeStepForSBI() を改善

```go
// Check if artifact file was created by Claude
artifactCreated := false
if _, err := os.Stat(artifactPath); err == nil {
    artifactCreated = true
}

// If Claude didn't create the artifact, save output with metadata fallback
if !artifactCreated {
    artifactDir := filepath.Dir(artifactPath)
    if err := os.MkdirAll(artifactDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create artifact directory: %w", err)
    }

    // Extract decision from agent output
    extractedDecision := "NEEDS_CHANGES"
    if currentStatus == "REVIEW" || currentStatus == "REVIEW&WIP" {
        extractedDecision = uc.extractDecision(agentResult.Output)
    }

    // Build fallback content with proper metadata structure
    fallbackContent := fmt.Sprintf(`## Summary
DECISION: %s

%s

{"decision": "%s"}
`, strings.ToUpper(extractedDecision), agentResult.Output, strings.ToLower(extractedDecision))

    if err := os.WriteFile(artifactPath, []byte(fallbackContent), 0644); err != nil {
        return nil, fmt.Errorf("failed to write artifact file: %w", err)
    }

    fmt.Fprintf(os.Stderr, "[fallback] Created artifact with metadata: %s (Decision=%s)\n",
        artifactPath, extractedDecision)
}
```

## 解決案3: 判定ロジックの柔軟化

### extractDecisionWithLogging() の改善

```go
func (uc *RunTurnUseCase) extractDecisionWithLogging(artifactPath string, agentOutput string, sbiID string) (decision string, source string) {
    // Try to read artifact file
    content, err := os.ReadFile(artifactPath)
    if err != nil {
        // Artifact doesn't exist - extract from agent output and create file
        decision = uc.extractDecision(agentOutput)

        // Create fallback file with proper structure
        fallbackContent := fmt.Sprintf(`## Summary
DECISION: %s

%s

{"decision": "%s"}
`, strings.ToUpper(decision), agentOutput, strings.ToLower(decision))

        artifactDir := filepath.Dir(artifactPath)
        os.MkdirAll(artifactDir, 0755)
        os.WriteFile(artifactPath, []byte(fallbackContent), 0644)

        fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=agent_output (file created), Decision=%s\n", sbiID, decision)
        return decision, "agent_output_fallback"
    }

    fileContent := string(content)

    // Extract from both sources
    headDecision := uc.extractDecisionFromHead(fileContent)
    tailDecision := uc.extractDecisionFromTailJSON(fileContent)

    // Case 1: Both match (ideal)
    if headDecision != "" && tailDecision != "" && headDecision == tailDecision {
        fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=metadata_match, Decision=%s\n",
            sbiID, headDecision)
        return headDecision, "metadata_match"
    }

    // Case 2: Only one source available - use it
    if headDecision != "" && tailDecision == "" {
        fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=metadata_head_only, Decision=%s\n",
            sbiID, headDecision)
        return headDecision, "metadata_head_only"
    }
    if headDecision == "" && tailDecision != "" {
        fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=metadata_tail_only, Decision=%s\n",
            sbiID, tailDecision)
        return tailDecision, "metadata_tail_only"
    }

    // Case 3: Mismatch or missing - use agent output
    decision = uc.extractDecision(agentOutput)
    fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=agent_output (mismatch), HeadDecision=%s, TailDecision=%s, AgentDecision=%s\n",
        sbiID, headDecision, tailDecision, decision)
    return decision, "agent_output_mismatch"
}
```

## 推奨実装順序

1. **即効性**: 解決案1（プロンプト改善）を実装 ← まずこれ
2. **信頼性**: 解決案3（判定ロジック柔軟化）を実装
3. **保険**: 解決案2（フォールバック機能強化）を実装

## 期待される効果

- **解決案1**: AIがメタデータ構造を正しく生成する確率が向上（推定80% → 95%）
- **解決案2**: ファイル未作成時でも適切なメタデータ付きファイルを自動生成
- **解決案3**: 片方のメタデータだけでも判定可能になり、robustnessが向上

## テスト方法

```bash
# 1. プロンプト改善後のテスト
./dist/deespec sbi run --sbi-id 01K7P4KVCF4NH4W2CKBCTMZG9P

# 2. ログ確認
# [decision] SBI=xxx, Source=metadata_match が増えることを確認
# [decision] SBI=xxx, Source=agent_output (file not found) が減ることを確認

# 3. review_N.md の構造確認
cat .deespec/specs/sbi/01K7P4KVCF4NH4W2CKBCTMZG9P/review_3.md
# → 冒頭に ## Summary + DECISION
# → 末尾に {"decision": "..."}
```
