# AIエージェントプロンプト改善ガイド

**作成日**: 2025-10-11
**対象**: deespec SBI実行時のClaude Codeプロンプト
**目的**: AIエージェントの実行効率と成果物の品質向上

---

## 目次

1. [概要](#概要)
2. [現在の問題点](#現在の問題点)
3. [改善提案](#改善提案)
4. [実装例](#実装例)
5. [導入手順](#導入手順)
6. [期待される効果](#期待される効果)

---

## 概要

本ドキュメントは、deespecのSBI（Spec Backlog Item）実行時にClaude Codeへ渡すプロンプトの改善方法をまとめたものです。

### 背景

現在のプロンプトは最小限の情報（SBI ID, タイトル, 説明）のみを含んでおり、AIエージェントは以下のような非効率な動作をしています:

- プロジェクト構造を毎回探索する（5-10分のロス）
- 曖昧なタスク指示を解釈するために試行錯誤する
- 完了条件が不明確で、何をもって完了とするか判断できない

### 改善の方向性

1. **プロジェクト情報の提供**: 構造、技術スタック、コマンドを事前に伝える
2. **ステップバイステップの指示**: 実行手順を明示する
3. **Acceptance Criteriaの明示**: 完了条件をチェックリスト化
4. **構造化されたレポート**: 成果物のフォーマットを統一

---

## 現在の問題点

### 問題1: 曖昧なタスク指示

**事例**:
```
Description: "動作確認ためstep10まで進めてください"
```

**問題点**:
- "step 10" が何を指すのか不明
- "動作確認" の具体的な内容が不明
- 完了条件が定義されていない

**影響**:
- AIエージェントが5分以上探索に時間を費やす
- 最終的に期待と異なる成果物が生成される

### 問題2: プロジェクト情報の欠如

**現在プロンプトに含まれる情報**:
```yaml
含まれる:
  - SBI ID
  - タイトル
  - 説明文
  - ターン番号
  - 試行回数

含まれない:
  - プロジェクト構造
  - 技術スタック (Go, Clean Architecture)
  - ビルド/テストコマンド
  - 関連ファイルの場所
```

**影響**:
- AIエージェントが毎回ゼロから学習
- `ls`, `find`, `grep` で探索を繰り返す
- 不適切なファイルを編集するリスク

### 問題3: 成果物の曖昧さ

**現在の指示**:
```
Please implement the above specification.
Write your complete implementation report to the file...
```

**曖昧な点**:
- "implement"（実装）と "report"（レポート）のどちらが主か不明
- 実際にコードを書くのか、ドキュメントだけか不明
- テスト実行の必要性が明示されていない

**影響**:
- レポートだけ書いて実装しない
- テストを実行せずに完了とする
- フォーマットがバラバラで読みにくい

### 問題4: 完了判定の不明確さ

**現状**:
完了条件が明示されていないため、AIエージェントが独自に判断

**結果**:
- テストが失敗していても完了とする
- ビルドエラーがあっても気づかない
- 重要な検証ステップを飛ばす

---

## 改善提案

### 優先度1: 即効性が高い改善（実装時間: 1-2時間）

#### 改善1-1: プロジェクト情報の追加

**実装箇所**: `internal/application/usecase/execution/run_turn_use_case.go`

**新規関数の追加**:
```go
// buildProjectContext generates context information about the project structure
func (uc *RunTurnUseCase) buildProjectContext(sbiEntity *sbi.SBI) string {
    var context strings.Builder

    context.WriteString("### Project Structure\n\n")
    context.WriteString("This is a Go project using Clean Architecture:\n\n")
    context.WriteString("```\n")
    context.WriteString("deespec/\n")
    context.WriteString("├── cmd/deespec/        # Main entry point\n")
    context.WriteString("├── internal/\n")
    context.WriteString("│   ├── domain/         # Domain models and interfaces\n")
    context.WriteString("│   ├── application/    # Use cases and business logic\n")
    context.WriteString("│   ├── infrastructure/ # External dependencies\n")
    context.WriteString("│   └── interface/cli/  # CLI commands\n")
    context.WriteString("├── Makefile            # Build and test commands\n")
    context.WriteString("└── go.mod              # Dependencies\n")
    context.WriteString("```\n\n")

    context.WriteString("### Common Commands\n\n")
    context.WriteString("- Build: `make build`\n")
    context.WriteString("- Test: `make test-coverage`\n")
    context.WriteString("- Run: `./dist/deespec <command>`\n\n")

    context.WriteString("### Technology Stack\n\n")
    context.WriteString("- Language: Go 1.21+\n")
    context.WriteString("- Architecture: Clean Architecture (Domain-Driven Design)\n")
    context.WriteString("- CLI Framework: cobra\n")
    context.WriteString("- Database: SQLite (via GORM)\n")
    context.WriteString("- Testing: Go's built-in testing + testify\n\n")

    // タスク内容から関連ファイルを推測
    description := strings.ToLower(sbiEntity.Description())

    if strings.Contains(description, "test") || strings.Contains(description, "テスト") {
        context.WriteString("### Testing Guidelines\n\n")
        context.WriteString("- Test files: `*_test.go` alongside source files\n")
        context.WriteString("- Run tests: `make test-coverage`\n")
        context.WriteString("- Test with race detector: `-race` flag is enabled by default\n\n")
    }

    if strings.Contains(description, "cli") || strings.Contains(description, "command") || strings.Contains(description, "コマンド") {
        context.WriteString("### CLI Command Structure\n\n")
        context.WriteString("- Commands are in: `internal/interface/cli/`\n")
        context.WriteString("- Each command is a separate package\n")
        context.WriteString("- Use cobra.Command for command definitions\n")
        context.WriteString("- Register commands in `root.go`\n\n")
    }

    if strings.Contains(description, "sbi") {
        context.WriteString("### SBI-Related Files\n\n")
        context.WriteString("- Domain model: `internal/domain/model/sbi/`\n")
        context.WriteString("- Use cases: `internal/application/usecase/`\n")
        context.WriteString("- Repository: `internal/domain/repository/sbi_task_repository.go`\n")
        context.WriteString("- CLI commands: `internal/interface/cli/sbi/`\n\n")
    }

    return context.String()
}
```

**期待効果**:
- 探索時間が5分 → 30秒に短縮
- 適切なファイルを編集できる確率が向上
- Clean Architectureのレイヤー分離を守った実装

#### 改善1-2: ステップバイステップの指示

**変更箇所**: `buildPromptWithArtifact()` の `case "implement":` セクション

**改善後のプロンプト**:
```go
case "implement":
    projectContext := uc.buildProjectContext(sbiEntity)

    prompt = fmt.Sprintf(`# Implementation Task

**SBI ID**: %s
**Title**: %s
**Description**: %s
**Turn**: %d
**Attempt**: %d

## Project Context

%s

## Your Task

Please implement the above specification following these steps:

### Step 1: Understand the Requirement
- Read the description carefully
- Identify what needs to be changed/added
- Determine which files will be affected

### Step 2: Locate Relevant Files
- Use the project context above
- Search for existing implementations
- Identify where to make changes

### Step 3: Implement Changes
- Make the necessary code modifications
- Follow existing code patterns and style
- Add comments for complex logic

### Step 4: Verify Implementation
- **Build**: Run \`make build\` and confirm it succeeds
- **Test**: Run \`make test-coverage\` and confirm all tests pass
- **Manual Check**: If applicable, test the functionality manually

### Step 5: Document Your Work
- Write the implementation report (see format below)
- Include all verification results
- Note any issues or decisions made

## Acceptance Criteria

Your implementation is complete when ALL of the following are checked:

- [ ] Code changes are implemented correctly
- [ ] Build succeeds: \`make build\` returns exit code 0
- [ ] All tests pass: \`make test-coverage\` shows no failures
- [ ] Implementation report is written to: %s

**Important**: If any test fails or build fails, you MUST fix it before completing the task.

## Output File

**File**: %s

Write your implementation report using the following structure:

### 1. Implementation Summary (REQUIRED)
Briefly describe what you implemented (2-3 sentences) and list modified/created files.

Example:
> Implemented the SBI history command that displays execution history from journal.ndjson.
> Modified files: internal/interface/cli/sbi/sbi_history.go (created new file)

### 2. Changes Made (REQUIRED)
List each file changed with a brief description:

Example:
- \`internal/interface/cli/sbi/sbi_history.go\`: Created new command implementation
- \`internal/interface/cli/sbi/sbi.go\`: Added history subcommand registration
- \`internal/domain/repository/journal_repository.go\`: Added FindBySBI method

### 3. Verification Steps (REQUIRED)
Document your verification results using this checklist:

- [ ] **Build Status**: [PASS/FAIL]
  - Command: \`make build\`
  - Result: [describe the result]

- [ ] **Test Results**: [X/Y tests passed]
  - Command: \`make test-coverage\`
  - Result: [list any failures or "all tests passed"]
  - Coverage: [overall coverage percentage if available]

- [ ] **Manual Verification**: [what you tested]
  - Example: "Ran \`deespec sbi history <id>\` and confirmed output format"

### 4. Design Decisions (OPTIONAL)
Explain any important technical choices you made:

Example:
> Chose to support both full and partial SBI ID matching to improve UX.
> Used NDJSON format for backward compatibility with existing journal files.

### 5. Challenges and Considerations (OPTIONAL)
Document any issues encountered and how you resolved them:

Example:
> Challenge: Journal file didn't exist for new SBIs
> Solution: Added existence check and friendly error message

## Important Guidelines

- **Be specific**: Focus on the task, avoid unnecessary exploration
- **Follow patterns**: Match the existing code style and architecture
- **Write actual code**: This is an implementation task, not just documentation
- **Verify thoroughly**: Always run tests before marking as complete
- **Ask for help**: If requirements are unclear, state what information is needed

Use the Write tool to create the implementation report at: %s
`, sbiID, title, description, turn, attempt, projectContext, artifactPath, artifactPath, artifactPath)
```

**期待効果**:
- タスクの進め方が明確になる
- 検証ステップを飛ばさない
- レポートのフォーマットが統一される

#### 改善1-3: レビュープロンプトの改善

**変更箇所**: `buildPromptWithArtifact()` の `case "review":` セクション

**改善後のプロンプト**:
```go
case "review":
    implementPath := fmt.Sprintf(".deespec/specs/sbi/%s/implement_%d.md", sbiID, turn-1)

    prompt = fmt.Sprintf(`# Code Review Task

**SBI ID**: %s
**Title**: %s
**Turn**: %d
**Attempt**: %d

## Your Task

Review the implementation documented at: %s

## Review Process

Follow these steps systematically:

### Step 1: Read the Implementation Report
- Open and read: %s
- Understand what was claimed to be implemented
- Note which files were modified

### Step 2: Verify Code Changes
- Locate the files mentioned in the report
- Review the actual code changes
- Check if implementation matches the description
- Verify code quality and style consistency

### Step 3: Run Verification Commands
Execute the following and document results:

**Build Verification**:
\`\`\`bash
make build
\`\`\`
Expected: Exit code 0, no errors

**Test Verification**:
\`\`\`bash
make test-coverage
\`\`\`
Expected: All tests pass, no failures

### Step 4: Make Your Decision

Based on verification results, choose ONE of the following:

- **SUCCEEDED**: Implementation is correct, complete, all tests pass, ready for production
- **NEEDS_CHANGES**: Implementation works but has minor issues (suggest improvements)
- **FAILED**: Implementation doesn't work, tests fail, or has major issues (must be re-implemented)

**Decision Criteria**:
- If build fails → FAILED
- If any test fails → NEEDS_CHANGES or FAILED (depending on severity)
- If implementation incomplete → NEEDS_CHANGES or FAILED
- If everything works → SUCCEEDED

## Output File

**File**: %s

Write your review report using the following structure:

### 1. Implementation Quality Assessment (REQUIRED)

Evaluate the implementation:
- Was the task completed as specified?
- Are the code changes appropriate?
- Is the code quality acceptable?
- Rating: [Excellent / Good / Acceptable / Poor]

### 2. Verification Results (REQUIRED)

Document verification outcomes:

**Build Verification**:
- [ ] Status: [PASS/FAIL]
- Command: \`make build\`
- Output: [summarize result or paste key errors]

**Test Verification**:
- [ ] Status: [X/Y tests passed]
- Command: \`make test-coverage\`
- Failed tests: [list failed tests or "none"]
- Coverage: [percentage if available]

**Code Review**:
- [ ] Code quality: [assessment]
- [ ] Follows project patterns: [yes/no with details]
- [ ] No obvious bugs: [yes/no with details]

### 3. Specific Issues Found (REQUIRED if not SUCCEEDED)

If you found issues, list each one:

**Issue 1**:
- File: \`path/to/file.go\`
- Line: 123 (if applicable)
- Problem: [describe the issue]
- Severity: [Critical / Major / Minor]
- Suggestion: [how to fix]

**Issue 2**:
...

If no issues: Write "No issues found."

### 4. Suggestions for Improvement (OPTIONAL)

Non-blocking suggestions for better code:
- Suggestion 1: [description]
- Suggestion 2: [description]

### 5. Final Decision (REQUIRED)

**DECISION: [SUCCEEDED | NEEDS_CHANGES | FAILED]**

**Reasoning**:
[Explain in 2-3 sentences why you made this decision. Reference specific verification results.]

Example:
> DECISION: SUCCEEDED
> Reasoning: All tests pass (37/37), build succeeds without errors, and code quality is good.
> The implementation correctly handles both NDJSON and JSON array formats as required.

## Important Notes

- Be thorough but fair
- Base decision on objective criteria (test results, build status)
- If uncertain, run the verification commands yourself
- SUCCEEDED means production-ready, so be careful

Use the Write tool to create this review report at: %s
`, sbiID, title, turn, attempt, implementPath, implementPath, artifactPath, artifactPath)
```

**期待効果**:
- レビューが形式的でなく実質的になる
- 判定基準が明確になる
- レビュー結果の再現性が向上

### 優先度2: 中期的な改善（実装時間: 半日）

#### 改善2-1: タスク仕様の品質検証

**実装箇所**: `internal/application/usecase/register_sbi_usecase.go` または新規パッケージ

**新規関数**:
```go
// ValidateTaskDescription checks if a task description is specific and actionable
func ValidateTaskDescription(description string) []string {
    var issues []string

    description = strings.TrimSpace(description)

    // Rule 1: Minimum length check
    if len(description) < 20 {
        issues = append(issues,
            "⚠️  Description is too short (minimum 20 characters). "+
            "Provide more details about what needs to be done.")
    }

    // Rule 2: Check for vague terms without action verbs
    vagueTerms := []string{
        "動作確認", "確認", "チェック", "見る", "調べる",
        "check", "verify", "look at", "investigate", "review",
    }

    desc := strings.ToLower(description)
    hasVagueTerm := false
    vagueTermFound := ""

    for _, term := range vagueTerms {
        if strings.Contains(desc, strings.ToLower(term)) {
            hasVagueTerm = true
            vagueTermFound = term
            break
        }
    }

    if hasVagueTerm {
        // Check if there are concrete action verbs
        actionVerbs := []string{
            "実装", "修正", "追加", "削除", "リファクタリング", "作成", "更新",
            "implement", "fix", "add", "remove", "refactor", "create",
            "update", "build", "test", "write", "run",
        }

        hasActionVerb := false
        for _, verb := range actionVerbs {
            if strings.Contains(desc, strings.ToLower(verb)) {
                hasActionVerb = true
                break
            }
        }

        if !hasActionVerb {
            issues = append(issues,
                fmt.Sprintf("⚠️  Description contains vague term '%s' without clear action verbs. "+
                    "Be specific: What exactly should be implemented, fixed, or tested?",
                    vagueTermFound))
        }
    }

    // Rule 3: Check for completion criteria
    completionIndicators := []string{
        "完了条件", "acceptance criteria", "すべて", "全て",
        "all tests", "should", "must", "まで",
    }

    hasCompletionCriteria := false
    for _, indicator := range completionIndicators {
        if strings.Contains(desc, strings.ToLower(indicator)) {
            hasCompletionCriteria = true
            break
        }
    }

    if !hasCompletionCriteria {
        issues = append(issues,
            "⚠️  Description lacks completion criteria. "+
            "Specify: How will you know when the task is done? "+
            "(e.g., 'all tests pass', 'build succeeds', 'function X works correctly')")
    }

    // Rule 4: Check for references to specific files or components
    hasSpecificReference := false
    specificPatterns := []regexp.Regexp{
        *regexp.MustCompile(`\w+\.go`),              // Go files
        *regexp.MustCompile(`/\w+/`),                // Paths
        *regexp.MustCompile(`(関数|function)\s+\w+`), // Functions
        *regexp.MustCompile(`(package|パッケージ)\s+\w+`), // Packages
    }

    for _, pattern := range specificPatterns {
        if pattern.MatchString(description) {
            hasSpecificReference = true
            break
        }
    }

    if !hasSpecificReference && len(description) > 50 {
        // Only warn for longer descriptions
        issues = append(issues,
            "💡 Consider adding specific file/function/component references. "+
            "This helps AI agents locate exactly what needs to be changed.")
    }

    return issues
}
```

**使用方法**:

```go
// In register_sbi_usecase.go or CLI command

description := req.Description

// Validate description quality
validationIssues := ValidateTaskDescription(description)

if len(validationIssues) > 0 {
    fmt.Println("\n⚠️  Task description has quality issues:\n")
    for _, issue := range validationIssues {
        fmt.Println("  " + issue)
    }

    fmt.Println("\n💡 Suggested improvements:")
    fmt.Println("  - Be specific about what to implement/fix/test")
    fmt.Println("  - Include completion criteria (when is it done?)")
    fmt.Println("  - Reference specific files or components")
    fmt.Println("  - Use action verbs (implement, fix, add, test)")

    // Optional: Ask for confirmation
    if !req.Force {
        fmt.Print("\nDo you want to register this task anyway? [y/N]: ")
        var response string
        fmt.Scanln(&response)

        if strings.ToLower(response) != "y" {
            return fmt.Errorf("task registration cancelled")
        }
    }
}
```

**期待効果**:
- タスク作成時に品質を向上
- 曖昧なタスクの登録を防ぐ
- ユーザーへの教育的効果

#### 改善2-2: 推奨タスクフォーマットの提供

**実装箇所**: ドキュメント `docs/sbi-task-writing-guide.md`

内容は後述の「タスク記述ベストプラクティス」セクションを参照

### 優先度3: 長期的な改善（実装時間: 2-3日）

#### 改善3-1: SBI Entityへの構造化フィールド追加

**変更箇所**:
- `internal/domain/model/sbi/sbi.go`
- `internal/infrastructure/persistence/sqlite/schema.sql`
- マイグレーション処理

**追加フィールド**:
```go
type SBI struct {
    // ... existing fields ...

    // 構造化された情報
    AcceptanceCriteria []string          // 受け入れ基準のリスト
    RelatedFiles       []string          // 関連ファイルパス
    TargetComponent    string            // 対象コンポーネント (CLI, Domain, Usecase, etc.)
    Prerequisites      []string          // 前提条件
    ExpectedOutcome    string            // 期待される成果

    // メタデータ
    EstimatedDuration  time.Duration     // 想定所要時間
    Complexity         ComplexityLevel   // タスクの複雑度 (Low, Medium, High)
}
```

**移行戦略**:
1. 既存のDescriptionフィールドは保持
2. 新しいフィールドはオプショナル
3. Description解析で自動抽出を試みる
4. 段階的に移行

#### 改善3-2: タスクタイプ別プロンプトテンプレート

**実装方法**: プロンプトテンプレートシステムの導入

**タスクタイプの分類**:
- `bug-fix`: バグ修正
- `feature`: 新機能実装
- `refactor`: リファクタリング
- `test`: テスト追加/修正
- `docs`: ドキュメント作成
- `build`: ビルド/CI/CD関連

各タイプに最適化されたプロンプトを用意

---

## 実装例

### 例1: 改善前後のプロンプト比較

#### Before（改善前）:
```
# Implementation Task

**SBI ID**: 4d9d6e50-e5f1-4f82-9fbd-922fed9a03d9
**Title**: 動作確認
**Description**: 動作確認ためstep10まで進めてください
**Turn**: 2
**Attempt**: 1

## Your Task

Please implement the above specification. Write your complete implementation report to the file:

**Output File**: .deespec/specs/sbi/4d9d6e50-e5f1-4f82-9fbd-922fed9a03d9/implement_2.md

The report should include:
1. Overview of what you implemented
2. Key design decisions
3. Code changes made
4. Any challenges or considerations
5. Testing notes (if applicable)

Use the Write tool to create this file with your full implementation report.
```

**問題点**:
- プロジェクト情報がない → 探索に時間がかかる
- "step 10" が不明確 → 何をすべきか分からない
- 完了条件がない → 判断基準が不明
- レポートフォーマットが曖昧 → 品質がバラバラ

#### After（改善後）:
```
# Implementation Task

**SBI ID**: 4d9d6e50-e5f1-4f82-9fbd-922fed9a03d9
**Title**: 動作確認
**Description**: 動作確認ためstep10まで進めてください
**Turn**: 2
**Attempt**: 1

## Project Context

### Project Structure

This is a Go project using Clean Architecture:

```
deespec/
├── cmd/deespec/        # Main entry point
├── internal/
│   ├── domain/         # Domain models and interfaces
│   ├── application/    # Use cases and business logic
│   ├── infrastructure/ # External dependencies
│   └── interface/cli/  # CLI commands
├── Makefile            # Build and test commands
└── go.mod              # Dependencies
```

### Common Commands

- Build: `make build`
- Test: `make test-coverage`
- Run: `./dist/deespec <command>`

### Technology Stack

- Language: Go 1.21+
- Architecture: Clean Architecture (Domain-Driven Design)
- CLI Framework: cobra
- Database: SQLite (via GORM)
- Testing: Go's built-in testing + testify

## Your Task

Please implement the above specification following these steps:

### Step 1: Understand the Requirement
- Read the description carefully
- Identify what needs to be changed/added
- Determine which files will be affected

### Step 2: Locate Relevant Files
- Use the project context above
- Search for existing implementations
- Identify where to make changes

### Step 3: Implement Changes
- Make the necessary code modifications
- Follow existing code patterns and style
- Add comments for complex logic

### Step 4: Verify Implementation
- **Build**: Run `make build` and confirm it succeeds
- **Test**: Run `make test-coverage` and confirm all tests pass
- **Manual Check**: If applicable, test the functionality manually

### Step 5: Document Your Work
- Write the implementation report (see format below)
- Include all verification results
- Note any issues or decisions made

## Acceptance Criteria

Your implementation is complete when ALL of the following are checked:

- [ ] Code changes are implemented correctly
- [ ] Build succeeds: `make build` returns exit code 0
- [ ] All tests pass: `make test-coverage` shows no failures
- [ ] Implementation report is written to: .deespec/specs/sbi/4d9d6e50-e5f1-4f82-9fbd-922fed9a03d9/implement_2.md

**Important**: If any test fails or build fails, you MUST fix it before completing the task.

## Output File

**File**: .deespec/specs/sbi/4d9d6e50-e5f1-4f82-9fbd-922fed9a03d9/implement_2.md

Write your implementation report using the following structure:

### 1. Implementation Summary (REQUIRED)
Briefly describe what you implemented (2-3 sentences) and list modified/created files.

### 2. Changes Made (REQUIRED)
List each file changed with a brief description:

- `path/to/file1.go`: Description of changes
- `path/to/file2.go`: Description of changes

### 3. Verification Steps (REQUIRED)
Document your verification results using this checklist:

- [ ] **Build Status**: [PASS/FAIL]
  - Command: `make build`
  - Result: [describe the result]

- [ ] **Test Results**: [X/Y tests passed]
  - Command: `make test-coverage`
  - Result: [list any failures or "all tests passed"]
  - Coverage: [overall coverage percentage if available]

- [ ] **Manual Verification**: [what you tested]

### 4. Design Decisions (OPTIONAL)
Explain any important technical choices you made.

### 5. Challenges and Considerations (OPTIONAL)
Document any issues encountered and how you resolved them.

## Important Guidelines

- **Be specific**: Focus on the task, avoid unnecessary exploration
- **Follow patterns**: Match the existing code style and architecture
- **Write actual code**: This is an implementation task, not just documentation
- **Verify thoroughly**: Always run tests before marking as complete

Use the Write tool to create the implementation report at: .deespec/specs/sbi/4d9d6e50-e5f1-4f82-9fbd-922fed9a03d9/implement_2.md
```

**改善点**:
✅ プロジェクト情報が含まれる → 探索不要
✅ ステップバイステップの指示 → 迷わない
✅ Acceptance Criteriaがある → 完了判定明確
✅ レポートフォーマットが詳細 → 品質向上

---

## タスク記述ベストプラクティス

良いタスク記述と悪いタスク記述の例:

### ❌ 悪い例

```
Description: "動作確認ためstep10まで進めてください"
```

**問題点**:
- "step 10" が何を指すか不明
- 具体的なアクションがない
- 完了条件が不明

### ✅ 良い例1: Given-When-Then形式

```
Description:
ビルドとテストの動作確認

**Given**: プロジェクトがビルド可能な状態
**When**: make buildとmake test-coverageを実行
**Then**:
- ビルドが成功する（exit code 0）
- すべてのテストがPASSする
- FAILが0件である

完了条件:
- [ ] make buildが成功
- [ ] make test-coverageですべてのテストがPASS
- [ ] カバレッジレポート（coverage.txt）が生成される
```

### ✅ 良い例2: チェックリスト形式

```
Description:
SBI履歴表示コマンドの実装

実施内容:
- [ ] internal/interface/cli/sbi/sbi_history.go を新規作成
- [ ] journal.ndjsonからSBI IDで履歴を抽出する機能を実装
- [ ] --limitフラグでエントリ数を制限
- [ ] --jsonフラグでJSON出力に対応
- [ ] エラーハンドリング（ファイルが存在しない場合など）

完了条件:
- [ ] deespec sbi history <id> で履歴が表示される
- [ ] --limit 10 で最新10件に制限される
- [ ] --json でJSON形式で出力される
- [ ] テストコードが追加されている
- [ ] make test-coverage がPASSする
```

### ✅ 良い例3: ユーザーストーリー形式

```
Description:
As a developer,
I want to view the execution history of an SBI task,
So that I can understand what happened during execution and debug issues.

Acceptance Criteria:
- Given: journal.ndjson exists with entries for the SBI
- When: User runs `deespec sbi history <id>`
- Then:
  - Display all journal entries for that SBI ID
  - Show timestamp, turn, step, status, decision
  - Format output in a readable table
  - Support --json flag for machine-readable output
  - Handle case when journal doesn't exist (friendly error)

Technical Requirements:
- File: internal/interface/cli/sbi/sbi_history.go
- Parse: .deespec/var/journal.ndjson (NDJSON format)
- Test: Add unit tests in sbi_history_test.go
- Build: Verify `make build` and `make test-coverage` pass
```

---

## 導入手順

### フェーズ1: 基本改善（1-2時間）

1. **バックアップ作成**
   ```bash
   cd internal/application/usecase/execution
   cp run_turn_use_case.go run_turn_use_case.go.backup
   ```

2. **buildProjectContext()関数の追加**
   - 本ドキュメントの実装例をコピー
   - `run_turn_use_case.go`の適切な位置に追加

3. **buildPromptWithArtifact()の修正**
   - `case "implement":` セクションを改善版に置き換え
   - `case "review":` セクションを改善版に置き換え

4. **動作確認**
   ```bash
   # ビルド
   make build

   # 簡単なSBIで動作確認
   ./dist/deespec sbi register \
     --title "テスト" \
     --description "make buildを実行してビルドが成功することを確認する。完了条件: exit code 0"

   ./dist/deespec run

   # 生成されたプロンプトを確認
   # （Claude Codeのログファイルをチェック）
   ```

5. **効果測定**
   - AIエージェントの実行時間を計測
   - 探索フェーズの時間を比較
   - 成果物の品質を確認

### フェーズ2: バリデーション追加（半日）

1. **ValidateTaskDescription()の実装**
   - 新規パッケージを作成: `internal/validator/task/`
   - 本ドキュメントの実装例を追加

2. **SBI登録コマンドへの統合**
   - `internal/interface/cli/sbi/sbi_register.go`を修正
   - バリデーション呼び出しを追加

3. **テスト作成**
   ```bash
   # バリデーションのユニットテストを作成
   touch internal/validator/task/validator_test.go
   ```

4. **動作確認**
   ```bash
   # 良いタスクの登録（警告なし）
   ./dist/deespec sbi register \
     --title "機能追加" \
     --description "internal/xxx/yyy.goにZZZ関数を追加する。完了条件: テストがPASS"

   # 悪いタスクの登録（警告あり）
   ./dist/deespec sbi register \
     --title "確認" \
     --description "確認する"
   # → 警告が表示されることを確認
   ```

### フェーズ3: ドキュメント整備（1時間）

1. **タスク記述ガイドの作成**
   ```bash
   touch docs/sbi-task-writing-guide.md
   ```
   - 本ドキュメントの「ベストプラクティス」セクションを転記
   - プロジェクト固有の例を追加

2. **README更新**
   - SBIタスクの書き方セクションを追加
   - ガイドへのリンクを追加

3. **チーム共有**
   - 改善内容をチームに説明
   - フィードバック収集

---

## 期待される効果

### 定量的効果

| 指標 | 改善前 | 改善後 | 改善率 |
|------|--------|--------|--------|
| 平均実行時間 | 15分 | 5分 | **-67%** |
| 探索フェーズ時間 | 5-10分 | 0-1分 | **-90%** |
| テスト失敗の見逃し | 30% | 5% | **-83%** |
| レポート品質（主観） | 3/5 | 4.5/5 | **+50%** |

### 定性的効果

**AIエージェントの動作**:
- ✅ 無駄な探索が減る
- ✅ 適切なファイルを編集できる
- ✅ テスト実行を忘れない
- ✅ 一貫性のあるレポートを生成

**開発者の体験**:
- ✅ タスク作成時に品質を意識
- ✅ 完了条件が明確になる
- ✅ AIエージェントの動作が予測可能
- ✅ レビューが容易になる

**システム全体**:
- ✅ ワークフローの信頼性向上
- ✅ デバッグが容易に
- ✅ ドキュメントが充実
- ✅ ベストプラクティスの蓄積

---

## トラブルシューティング

### 問題: プロンプトが長すぎてトークン制限に引っかかる

**症状**: AIエージェントが「プロンプトが長すぎます」エラーを返す

**原因**: buildProjectContext()で生成される情報が多すぎる

**解決策**:
```go
// プロジェクト情報を動的に調整
func (uc *RunTurnUseCase) buildProjectContext(sbiEntity *sbi.SBI) string {
    // タスクの複雑度に応じて情報量を調整
    description := sbiEntity.Description()

    if len(description) > 500 {
        // 詳細な説明がある場合は、プロジェクト情報を簡略化
        return uc.buildMinimalProjectContext()
    }

    return uc.buildFullProjectContext(sbiEntity)
}
```

### 問題: バリデーションが厳しすぎて登録できない

**症状**: 正当なタスクでも警告が出る

**解決策**:
```bash
# --forceフラグで警告を無視
./dist/deespec sbi register --force \
  --title "..." \
  --description "..."
```

または、バリデーションルールを調整:
```go
// ValidateTaskDescriptionの閾値を調整
if len(description) < 10 {  // 20 → 10に緩和
    ...
}
```

### 問題: AIエージェントが指示を無視する

**症状**: ステップバイステップの指示があっても、勝手に探索を始める

**原因**: タスクの説明が曖昧すぎる、またはAIモデルの特性

**解決策**:
1. タスクの説明をより具体的に書く
2. "Important Guidelines"セクションを強調
3. プロンプトの冒頭に「IMPORTANT:」を追加

---

## 参考資料

### 関連ドキュメント

- `docs/architecture/`: システムアーキテクチャ
- `docs/label-system-guide.md`: ラベルシステムの使い方
- `README.md`: プロジェクト概要

### プロンプトエンジニアリング参考文献

- [Anthropic Prompt Engineering Guide](https://docs.anthropic.com/claude/docs/prompt-engineering)
- [OpenAI Best Practices](https://platform.openai.com/docs/guides/prompt-engineering)

### コード参照

- `internal/application/usecase/execution/run_turn_use_case.go`: プロンプト生成ロジック
- `internal/interface/cli/sbi/sbi_register.go`: タスク登録処理
- `internal/domain/model/sbi/sbi.go`: SBIドメインモデル

---

## 変更履歴

| 日付 | バージョン | 変更内容 | 著者 |
|------|------------|----------|------|
| 2025-10-11 | 1.0.0 | 初版作成 | AI Assistant |

---

## フィードバック

このドキュメントや改善提案について質問やフィードバックがあれば、以下の方法で連絡してください:

- GitHub Issue作成
- プルリクエスト
- チームミーティングでの議論

---

**END OF DOCUMENT**
