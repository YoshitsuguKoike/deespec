# PBI登録方法の設計：ファイルベース vs コマンド引数

> この文書は `docs/pbi_how_to_work.md` から分離されました。
> PBI登録方法の設計について詳細に解説しています。

---

## 目次

1. [設計における根本的な問い](#1-設計における根本的な問い)
2. [結論：ファイルベースが主、コマンド引数は補助](#2-結論ファイルベースが主コマンド引数は補助)
3. [実用性の観点](#3-実用性の観点)
4. [既存ツールのベストプラクティス](#4-既存ツールのベストプラクティス)
5. [deespecの哲学との整合性](#5-deespecの哲学との整合性)
6. [推奨設計：ハイブリッドアプローチ](#6-推奨設計ハイブリッドアプローチ)
7. [実装優先度](#7-実装優先度)
8. [ファイルフォーマット詳細](#8-ファイルフォーマット詳細)
9. [コマンドライン引数（簡易版）の仕様](#9-コマンドライン引数簡易版の仕様)
10. [まとめ：設計の原則](#10-まとめ設計の原則)
11. [実装ガイドライン](#11-実装ガイドライン)

---

## 1. 設計における根本的な問い

PBIのような**長い・構造化されたデータ**をどう入力するか？

この問いに対する答えは、deespecの設計哲学とユーザビリティの両方に影響します。

---

## 2. 結論：**ファイルベースが主、コマンド引数は補助**

### なぜファイルベースなのか？

**3つのレベルで分析**：

1. **実用性の観点**
2. **既存ツールのベストプラクティス**
3. **deespecの哲学との整合性**

---

## 3. 実用性の観点

### ❌ コマンド引数で直接投入する問題点

```bash
# 現実的に不可能
deespec pbi register \
  --id PBI-001 \
  --title "テストカバレッジ50%達成" \
  --description "CIの要件である50%カバレッジを達成するため、
  ドメインモデル、Repository、Application層のテストを追加する。

  現状の課題：
  - Application Usecase層が完全に未テスト（2,907行、0%）
  - ドメインモデル層も大部分が未テスト

  解決策：
  Phase 1: ドメインモデルテスト（1-2日）
  Phase 2: Repository層テスト（2-3日）
  Phase 3: Usecase層部分テスト（3-5日）" \
  --acceptance-criteria "go test -cover ./... で50%以上" \
  --acceptance-criteria "CIが成功する" \
  --story-points 8 \
  --labels testing,ci-fix,technical-debt
```

**問題点**:
- 🔴 **長すぎて読めない・編集できない**
- 🔴 **改行やフォーマットが崩れる**
- 🔴 **シェルのエスケープ問題** (引用符、$変数、バックスラッシュ)
- 🔴 **履歴に残るが管理しにくい** (bashの履歴は検索困難)
- 🔴 **バージョン管理できない** (Gitで差分が見られない)

### ✅ ファイルベースの利点

```bash
# シンプルで明確
deespec pbi register -f .deespec/specs/pbi/PBI-001.yaml
```

```yaml
# .deespec/specs/pbi/PBI-001.yaml
id: PBI-001
title: "テストカバレッジ50%達成"
description: |
  CIの要件である50%カバレッジを達成するため、
  ドメインモデル、Repository、Application層のテストを追加する。

  ## 現状の課題
  - Application Usecase層が完全に未テスト（2,907行、0%）
  - ドメインモデル層も大部分が未テスト

  ## 解決策
  - Phase 1: ドメインモデルテスト（1-2日）
  - Phase 2: Repository層テスト（2-3日）
  - Phase 3: Usecase層部分テスト（3-5日）

acceptance_criteria:
  - condition: "go test -cover ./..."
    expected: ">= 50.0%"
  - condition: "CI status"
    expected: "passing"
  - condition: "Domain model coverage"
    expected: ">= 90%"

estimated_story_points: 8
priority: 1
labels: [testing, ci-fix, technical-debt]
```

**利点**:
- ✅ **読みやすい、編集しやすい**
- ✅ **フォーマットが保たれる** (改行、インデント)
- ✅ **IDEのサポート** (シンタックスハイライト、補完、バリデーション)
- ✅ **Gitで差分管理** (変更履歴が追跡可能)
- ✅ **コードレビュー可能** (PRで議論できる)
- ✅ **テンプレート化** (コピペで新規作成)
- ✅ **ドキュメントとしても機能** (そのまま仕様書)

---

## 4. 既存ツールのベストプラクティス

### 主要な開発ツールはすべて「ファイルベース」

| ツール | ファイル形式 | コマンド例 | 理由 |
|--------|-------------|-----------|------|
| **Kubernetes** | YAML | `kubectl apply -f deployment.yaml` | 複雑な設定をバージョン管理 |
| **Docker** | Dockerfile | `docker build -f Dockerfile` | 再現可能なビルド |
| **Terraform** | .tf | `terraform apply` | Infrastructure as Code |
| **GitHub Actions** | .yml | `.github/workflows/ci.yml` | CI/CD定義の共有 |
| **Make** | Makefile | `make build` | ビルド手順の標準化 |
| **npm** | package.json | `npm install` | 依存関係の宣言的管理 |
| **Ansible** | .yaml | `ansible-playbook site.yml` | 構成管理の自動化 |

**共通パターン：Infrastructure as Code (IaC)**

```
設定ファイル (宣言的) → ツール → 実行結果
     ↓                      ↓
  Git管理可能          再現可能
  レビュー可能          自動化可能
```

**なぜファイルベース？**
- 📝 **宣言的管理**: "何をしたいか"を記述
- 🔄 **バージョン管理**: Gitで履歴追跡
- 👥 **チーム共有**: 誰でも同じ状態を再現
- 🔍 **レビュー・監査**: 変更が可視化
- 🤖 **自動化**: CIで検証・デプロイ

### 反例：コマンド引数が主のツール

| ツール | 理由 |
|--------|------|
| **ls, grep, find** | 単発の検索・表示操作 |
| **git add, commit** | 小さな単位の操作 |
| **curl** | 一時的なAPIテスト |

→ これらは**状態を保存しない**操作

**PBIは状態を保存すべき** → ファイルベースが適切

---

## 5. deespecの哲学との整合性

### deespecは「ドキュメントファースト」

```
.deespec/
├── specs/           ← すべての仕様をファイルとして保存
│   ├── epic/
│   │   └── EPIC-001.yaml
│   ├── pbi/
│   │   ├── PBI-001.yaml    ← PBIもここに
│   │   └── PBI-002.yaml
│   └── sbi/
│       ├── SBI-001/
│       │   ├── spec.yaml
│       │   ├── implement_1.md
│       │   └── review_1.md
│       └── SBI-002/
├── var/
│   ├── journal.ndjson      ← 実行履歴
│   └── health.json
├── prompts/
│   ├── IMPLEMENT.md
│   ├── REVIEW.md
│   └── DONE.md
└── docs/                   ← 計画・設計ドキュメント
    ├── architecture.md
    └── test-coverage-improvement-plan.md
```

**deespecの設計思想**:
1. すべての**仕様**は `.deespec/specs/` に保存
2. すべての**実行履歴**は `journal.ndjson` に記録
3. すべての**計画**は `docs/` に文書化
4. すべての**プロンプト**は `.deespec/prompts/` にテンプレート化

→ **PBIもファイルとして保存するのが自然**

### 既存のSBI registerコマンドとの一貫性

```bash
# SBI register（既存）
deespec sbi register -f spec.yaml
deespec sbi register -f spec.json
deespec sbi register --stdin < spec.yaml
```

→ **PBIも同じパターンで統一**

```bash
# PBI register（提案）
deespec pbi register -f pbi.yaml      # YAMLファイル
deespec pbi register -f pbi.json      # JSONファイル
deespec pbi register --stdin < pbi.yaml  # stdin経由
```

---

## 6. 推奨設計：ハイブリッドアプローチ

### 主要：ファイルベース（完全な機能）

```bash
# 1. エディタでYAMLファイルを作成
vim .deespec/specs/pbi/PBI-001.yaml

# 2. 登録
deespec pbi register -f .deespec/specs/pbi/PBI-001.yaml

# または stdin経由
cat pbi-template.yaml | deespec pbi register -f -
```

**用途**:
- 詳細なPBI定義
- 複雑な受け入れ基準
- 長い説明文
- チームでの共有・レビュー

### 補助：コマンド引数（簡易版のみ）

```bash
# 簡単なPBIならコマンドで素早く登録
deespec pbi register \
  --id PBI-002 \
  --title "ログ出力改善" \
  --description "ログフォーマットを統一する" \
  --story-points 2

# 内部的には .deespec/specs/pbi/PBI-002.yaml を自動生成
# → あとでエディタで詳細を追加できる
```

**用途**:
- 素早いメモ・アイデアの記録
- プロトタイピング
- デモ・テスト
- 自動化スクリプト

**制限**:
- 単純な構造のみ
- 詳細は後でファイル編集で追加

### 便利：対話モード（将来実装）

```bash
# ウィザード形式で入力（初心者向け）
deespec pbi register --interactive

# 対話的に入力を促す
# > PBI ID: PBI-003
# > Title: API認証機能追加
# > Description (Ctrl+D to finish):
#   OAuth 2.0をサポートする
#   既存の認証と共存させる
#   ^D
# > Story Points: 5
# > Labels (comma-separated): feature,api
# > Edit in $EDITOR? [y/N]: y

# yを選択するとエディタが起動
# 保存すると .deespec/specs/pbi/PBI-003.yaml として保存
```

### 最も自然：エディタ統合（将来実装）

```bash
# テンプレートを生成してエディタで開く
deespec pbi init PBI-004
# → .deespec/specs/pbi/PBI-004.yaml を生成し $EDITOR で開く
# → 保存すると自動的に登録される

# 既存PBIの編集
deespec pbi edit PBI-001
# → $EDITORで開く
# → 保存すると変更を検出して更新

# ファイル監視モード（自動sync）
deespec pbi watch
# → .deespec/specs/pbi/ を監視
# → ファイルが変更されると自動的に反映
```

---

## 7. 実装優先度

### Phase 1: 基本機能（最優先）

```bash
# ✅ Priority 1
deespec pbi register -f <file>     # ファイルからの登録
deespec pbi show <id>              # 既存PBIの表示
deespec pbi list                   # 一覧表示
deespec pbi list --status IMPLEMENTING  # フィルタ
```

**理由**: 最も重要な機能。これだけで実用可能。

### Phase 2: 利便性向上（次のステップ）

```bash
# ✅ Priority 2
deespec pbi register --title "..." --description "..."  # 簡易登録
deespec pbi init PBI-005           # テンプレート生成
deespec pbi register --interactive # 対話モード
```

**理由**: UXの向上。初心者にも優しく。

### Phase 3: 高度な機能（将来）

```bash
# ✅ Priority 3
deespec pbi edit PBI-001           # エディタで編集
deespec pbi watch                  # ファイル監視
deespec pbi template --output custom-template.yaml  # カスタムテンプレート
```

**理由**: パワーユーザー向け。自動化。

---

## 8. ファイルフォーマット詳細

### 推奨フォーマット：YAML（人間が読みやすい）

```yaml
# .deespec/specs/pbi/PBI-001.yaml
version: "1.0"  # スキーマバージョン
type: pbi

# === 基本情報 ===
id: PBI-001
title: "テストカバレッジ50%達成"
description: |
  複数行の詳細な説明。
  Markdownも使える。

  ## 背景
  CIが失敗している。

  ## 目標
  カバレッジ50%達成。

# === メタデータ ===
status: PENDING
estimated_story_points: 8
priority: 1  # 0=通常, 1=高, 2=緊急
labels:
  - testing
  - ci-fix
  - technical-debt
assigned_agent: claude-code

# === 階層構造 ===
parent_epic: null  # EPICのIDまたはnull
child_sbis:
  - TEST-COV-SBI-001
  - TEST-COV-SBI-002
  - TEST-COV-SBI-003

# === 受け入れ基準 ===
acceptance_criteria:
  - condition: "Coverage"
    expected: ">= 50%"
    measurement_command: "go test -cover ./..."
  - condition: "CI"
    expected: "passing"
    verification: "GitHub Actions must be green"

# === 関連ドキュメント ===
related_documents:
  - docs/test-coverage-improvement-plan.md
  - docs/architecture.md

# === カスタムフィールド（拡張可能） ===
custom:
  related_issues:
    github: "#123"
    jira: "PROJ-456"
  risk_level: medium
  dependencies:
    - "Infrastructure setup"
```

### 代替：JSON（機械が読みやすい）

```json
{
  "version": "1.0",
  "type": "pbi",
  "id": "PBI-001",
  "title": "テストカバレッジ50%達成",
  "description": "CIの要件である50%カバレッジを達成...",
  "status": "PENDING",
  "estimated_story_points": 8,
  "labels": ["testing", "ci-fix"]
}
```

**YAMLとJSONの使い分け**:
- **YAML**: 人間が手で書く・読む（推奨）
- **JSON**: 機械生成、API連携、自動化

---

## 9. コマンドライン引数（簡易版）の仕様

```bash
# 必須項目のみで登録（他はデフォルト値）
deespec pbi register \
  --title "ログ出力改善" \
  --description "ログフォーマットを統一"

# 自動生成される内容:
# - id: PBI-XXX (自動採番)
# - status: PENDING
# - priority: 0 (通常)
# - estimated_story_points: 0 (未見積もり)
# - created_at: 現在時刻

# オプション追加
deespec pbi register \
  --title "ログ出力改善" \
  --description "ログフォーマットを統一" \
  --story-points 2 \
  --priority 1 \
  --labels testing,logging \
  --output .deespec/specs/pbi/CUSTOM-001.yaml

# 生成されたファイルを確認・編集
cat .deespec/specs/pbi/CUSTOM-001.yaml
# → 必要に応じてエディタで詳細を追加
```

---

## 10. まとめ：設計の原則

### ✅ 推奨：ファイルベースを主軸に

**理由**:
1. ✅ 長い内容（説明、受け入れ基準、関連ドキュメント）を扱える
2. ✅ Gitでバージョン管理できる（変更履歴、差分、ブランチ）
3. ✅ コードレビューできる（PRでチームで議論）
4. ✅ IDEのサポート（シンタックスハイライト、補完、検証）
5. ✅ 既存ツールのベストプラクティスに準拠（Kubernetes, Docker, Terraform等）
6. ✅ deespecの「ドキュメントファースト」哲学に合致
7. ✅ 既存のSBI registerとの一貫性

### ✅ 補助：コマンド引数（限定的）

**用途**:
- 素早いメモ・アイデアの記録
- プロトタイピング・デモ
- 自動化スクリプトでの生成

**制限**:
- 単純な構造のみ
- 内部的にYAMLファイルを生成
- 詳細は後でファイル編集

### ✅ 将来：対話モード & エディタ統合

**UX向上**:
- 初心者にも優しい段階的入力
- エディタで長文を快適に編集
- ファイル監視で自動sync

---

## 11. 実装ガイドライン

```go
// internal/interface/cli/pbi/register.go

func newRegisterCmd() *cobra.Command {
    var (
        file        string  // -f, --file
        title       string  // --title
        description string  // --description
        storyPoints int     // --story-points
        interactive bool    // --interactive
    )

    cmd := &cobra.Command{
        Use:   "register",
        Short: "Register a new PBI",
        Long: `Register a new PBI from a YAML/JSON file or command-line arguments.

Recommended: Use a YAML file for detailed PBIs
Quick mode: Use --title for simple PBIs`,
        RunE: func(cmd *cobra.Command, args []string) error {
            // Priority 1: File-based
            if file != "" {
                return registerFromFile(file)
            }

            // Priority 2: Interactive mode
            if interactive {
                return registerInteractive()
            }

            // Priority 3: Command-line arguments (simple mode)
            if title != "" {
                return registerFromArgs(title, description, storyPoints)
            }

            // No input provided
            return cmd.Help()
        },
    }

    cmd.Flags().StringVarP(&file, "file", "f", "", "PBI YAML/JSON file")
    cmd.Flags().StringVar(&title, "title", "", "PBI title (simple mode)")
    cmd.Flags().StringVar(&description, "description", "", "PBI description")
    cmd.Flags().IntVar(&storyPoints, "story-points", 0, "Estimated story points")
    cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

    return cmd
}
```

---

**Infrastructure as Code の原則**:

Kubernetes, Docker, Terraform など、成功しているツールはすべて「ファイルベース」を採用しています。

コマンドライン引数は**一時的な操作**用、永続的な設定は**ファイル**に保存。

deespecも同じ原則に従うことで、既存のエコシステム（Git, CI/CD、IDE等）と自然に統合できます。
