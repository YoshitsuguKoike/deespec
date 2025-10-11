# PBI登録フローの設計：ユーザー体験

> この文書はユーザー体験とワークフローにフォーカスしています。
> 技術的な実装提案については [pbi_how_to_work_03.md](./pbi_how_to_work_03.md) を参照してください。

---

## はじめに：設計の目標

### 目標
1. **deespecを意識させない**: ユーザーは`.deespec/`ディレクトリの存在を意識しない
2. **MCPライクな体験**: タスクの委譲と自動実行
3. **シームレスな統合**: Claude Codeとの自然な対話でPBIが登録される

### 原則："Invisible Infrastructure"

最高のツールは存在を意識させない。
- Gitを使うとき、`.git/`ディレクトリを意識しない
- npmを使うとき、`node_modules/`を意識しない
- **deespecも同様に、`.deespec/`を意識させるべきではない**

---

## 1. 理想のユーザー体験フロー

### Phase 1: 相談・計画

```markdown
User: "テストカバレッジが低くてCIが失敗してるんだけど、どうしたらいい？"

Claude Code:
「現状を分析しますね」
- カバレッジ測定を実行
- 問題箇所を特定
- docs/test-coverage-plan.md を作成
- 複数のオプションを提示

Claude Code:
"""
計画を作成しました（docs/test-coverage-plan.md）

3つのアプローチがあります：
1. 全体的にテストを追加（時間: 10日、リスク: 低）
2. 段階的にテスト追加（時間: 6日、リスク: 中）✅ 推奨
3. リファクタリング後にテスト（時間: 15日、リスク: 高）

どのアプローチにしますか？
"""

User: "Option 2の段階的アプローチでお願い"
```

### Phase 2: PBI化の依頼

```markdown
User: "じゃあこれをPBI化してください"

Claude Code:
"""
わかりました。PBIを登録します。

📝 読み取り: docs/test-coverage-plan.md
🔍 解析: タイトル、説明、受け入れ基準を抽出
📦 生成: .deespec/specs/pbi/PBI-001.yaml
✅ 登録: deespec pbi register を実行

PBI-001として登録完了しました。

**PBI-001: テストカバレッジ50%達成**
- ストーリーポイント: 8
- 優先度: 高
- 受け入れ基準: カバレッジ >= 50%, CI passing

詳細は `deespec pbi show PBI-001` で確認できます。
"""
```

### Phase 3: 実行（将来の機能）

```markdown
User: "じゃあ実行してください"

Claude Code:
"""
PBI-001を実行します。

🔀 分解中: PBI → SBI への分解
  ✓ SBI-001: value_object_test.go 作成
  ✓ SBI-002: task_test.go 作成
  ✓ SBI-003: pbi_test.go 作成
  ✓ SBI-004: epic_test.go 作成

▶️  実行開始: SBI-001

（以降、自動的に実装・テスト・レビューを実施）
"""
```

### キーポイント

- ✅ ユーザーは「PBI化してください」と言うだけ
- ✅ `.deespec/specs/pbi/`の存在を意識しない
- ✅ `docs/`配下のドキュメントは普通に見える
- ✅ Claude Codeがすべての技術的詳細を処理

---

## 2. ドキュメント配置戦略：2層アーキテクチャ

### 2.1 ディレクトリ構造

```
workspace/
├── docs/                           ← ユーザーの資産（Git管理、共有可能）
│   ├── test-coverage-plan.md      ← 人間が読む計画書
│   ├── api-authentication.md       ← 技術仕様
│   ├── architecture.md             ← 設計ドキュメント
│   └── pbi_how_to_work.md          ← deespec使い方ガイド
│
└── .deespec/                       ← deespecの内部状態（隠蔽）
    ├── specs/
    │   ├── pbi/
    │   │   ├── PBI-001.yaml        ← 機械可読な仕様
    │   │   └── PBI-002.yaml
    │   └── sbi/
    │       └── SBI-001/
    │           ├── spec.yaml
    │           ├── implement_1.md   ← 実行ログ
    │           └── review_1.md
    ├── var/
    │   └── journal.ndjson           ← 実行履歴
    └── prompts/
        ├── IMPLEMENT.md
        ├── REVIEW.md
        └── DONE.md
```

### 2.2 役割分担

#### **docs/ = ユーザーの資産**

**特徴**:
- Markdownで自由に書ける
- Gitで管理、PRでレビュー
- チームで共有
- **人間が読む**

**用途**:
- 計画書・設計書
- 技術仕様
- アーキテクチャドキュメント
- 学習メモ・調査結果

**例**:
```markdown
# docs/test-coverage-plan.md

## 目的
CIの要件である50%カバレッジを達成する

## 現状
- 全体: 34%
- ドメインモデル: 未テスト
- Application層: 0%

## アプローチ
Phase 1: ドメインモデルテスト（1-2日）
Phase 2: Repository層テスト（2-3日）
Phase 3: Usecase層部分テスト（3-5日）

## 受け入れ基準
- [ ] カバレッジ >= 50%
- [ ] CI passing
- [ ] ドメインモデル >= 90%
```

#### **.deespec/ = 機械的な状態管理**

**特徴**:
- YAML/JSON形式
- deespecが自動生成・更新
- ユーザーは直接触らない（理想的には）
- **機械が読む**

**用途**:
- PBI/SBIの構造化データ
- 実行履歴・状態管理
- プロンプトテンプレート

**例**:
```yaml
# .deespec/specs/pbi/PBI-001.yaml

id: PBI-001
title: "テストカバレッジ50%達成"
type: pbi
status: PENDING

description: |
  CIの要件である50%カバレッジを達成するため、
  ドメインモデル、Repository、Application層のテストを追加する。

acceptance_criteria:
  - condition: "Coverage"
    expected: ">= 50%"
    measurement_command: "go test -cover ./..."
  - condition: "CI"
    expected: "passing"

estimated_story_points: 8
priority: 1
labels: [testing, ci-fix, technical-debt]

# 元ドキュメントへのリンク
source_document: ../../docs/test-coverage-plan.md
generated_from: "claude-code conversation on 2025-10-11"
```

### 2.3 双方向リンク

#### PBI → docs（必須）

```yaml
# .deespec/specs/pbi/PBI-001.yaml

source_document: ../../docs/test-coverage-plan.md
```

#### docs → PBI（オプション）

```markdown
<!-- PBI: PBI-001 -->
# テストカバレッジ改善計画

このドキュメントはPBI-001として登録されています。
進捗状況: `deespec pbi show PBI-001`
```

### 2.4 戦略の利点

| 観点 | docs/ | .deespec/ |
|------|-------|-----------|
| **フォーマット** | Markdown（自由） | YAML/JSON（構造化） |
| **編集者** | 人間 | 機械（deespec） |
| **可読性** | 高（人間向け） | 低（機械向け） |
| **バージョン管理** | 必須 | オプション |
| **共有** | チーム全体 | 個人・ローカル |
| **変更頻度** | 低（計画段階） | 高（実行中） |

**結論**: 両方を活用することで、人間と機械の両方にとって最適な体験を提供

---

## 3. ユースケース例

### ケース1: 新機能開発

```markdown
User: "API認証機能を追加したいんだけど"

Claude Code:
「OAuth 2.0とJWT、どちらを検討されていますか？」

User: "OAuth 2.0でお願い"

Claude Code:
「わかりました。計画を作成します」
→ docs/api-authentication-plan.md を作成

User: "じゃあこれをPBI化してください"

Claude Code:
✓ PBI-002として登録しました
**タイトル**: API認証機能追加（OAuth 2.0）
**ストーリーポイント**: 13
**優先度**: 高
```

### ケース2: バグ修正

```markdown
User: "ログインがタイムアウトする問題を調査して"

Claude Code:
（調査実行）
「原因を特定しました。docs/login-timeout-fix.md に修正計画を作成しました」

User: "緊急で直したいのでPBI化してください"

Claude Code:
✓ PBI-003として登録しました
**タイトル**: ログインタイムアウト問題修正
**ストーリーポイント**: 3
**優先度**: 緊急
**ラベル**: hotfix, bug
```

### ケース3: リファクタリング

```markdown
User: "コードが複雑になってきたのでリファクタリングしたい"

Claude Code:
「どの部分が特に問題ですか？」

User: "usecaseディレクトリの大きなファイル"

Claude Code:
「分析して提案を作成します」
→ docs/refactoring-plan.md を作成

User: "Option 2でPBI化してください"

Claude Code:
✓ PBI-004として登録しました
**タイトル**: Usecase層リファクタリング（段階的分割）
**ストーリーポイント**: 8
**Phase 1-3**: 3つのSBIに分解済み
```

---

## 4. エラーハンドリング

### ドキュメントが見つからない場合

```markdown
User: "PBI化してください"

Claude Code:
「どのドキュメントをPBI化しますか？
最近作成したドキュメント:
- docs/test-coverage-plan.md (10分前)
- docs/api-auth-plan.md (1時間前)

番号で選択するか、ファイルパスを教えてください」
```

### deespecコマンドが失敗した場合

```markdown
Claude Code:
「PBI登録中にエラーが発生しました：

Error: .deespec/specs/pbi/ directory does not exist

deespecを初期化する必要があります。
`deespec init` を実行しますか？」
```

### 必須情報が不足している場合

```markdown
Claude Code:
「ドキュメントからタイトルを抽出できませんでした。
タイトルを教えてください」

User: "テストカバレッジ改善"

Claude Code:
✓ タイトルを設定しました
✓ PBI-005として登録しました
```

---

## 5. まとめ

### 5.1 設計の核心原則

1. **Invisible Infrastructure**: ユーザーは`.deespec/`を意識しない
2. **Document-First**: `docs/`が情報源、`.deespec/`は派生物
3. **Seamless Integration**: Claude Codeとの自然な対話
4. **Progressive Enhancement**: 段階的な機能追加

### 5.2 成功の指標

**ユーザーがこう言えたら成功**:

> "計画を立てて、Claude Codeに『PBI化してください』と言うだけ。
> あとはdeespecが全部やってくれる。
> `.deespec`ディレクトリ？触ったことないよ。"

### 5.3 次のステップ

技術的な実装提案については、[pbi_how_to_work_03.md](./pbi_how_to_work_03.md)を参照してください：

- Claude Codeがdeespecを認識する方法
- Agent管理コマンド設計
- 段階的実装計画
- ファイルフォーマット詳細
- コード例と実装ロードマップ

---

**この設計により、ユーザーは「やりたいこと」に集中でき、deespecの技術的詳細を意識せずにPBIを活用できます。**
