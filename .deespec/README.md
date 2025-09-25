# .deespec v1 — 標準構成

> deespec の **ホームディレクトリ**。v1 では旧互換なしで **全ての設定・仕様・実行生成物** をここに集約します。

## ディレクトリ構成

```
.deespec/
  etc/                    # 配線・ポリシー（VCS推奨）
    workflow.yaml         # steps[].prompt_path 参照のみ（長文はMD外出し）
    policies/
  prompts/                # 人間向け長文（VCS推奨）
    system/impl.md
    system/review.md
    system/test.md
    system/plan.md
    system/done.md
  specs/                  # 1タスク=1ディレクトリ（SBI/PBI）
    sbi/SBI-[pbi|0]-slug/
      instruction.md
      acceptance.md
      impl_note.md?       # stepに応じて作成（必須ではない）
      review_note.md?     # 同上
      meta.yaml
    pbi/PBI-xxx/
      requirement.md
  var/                    # 実行時生成（VCS除外）
    state.json
    journal.ndjson
    health.json
    artifacts/
```

## 方針

- **長文は Markdown**、`workflow.yaml` は **配線（prompt_path）** のみ
- **SBI登録は Go CLI 経由**（AIは直接書かない）
- **実装/レビューのノート**（*_note.md）は作業フェーズで生成
- **journal** は NDJSON 7キー固定・UTC・turn整合・decision列挙
- **CI** は `.deespec/var/journal.ndjson` のみ検査対象

## 変数差し込み（許可リスト）

- `{turn}`, `{task_id}`, `{project_name}`, `{language}`
- これ以外の `{...}` はエラー（doctor/CI が検出）

## gitignore 推奨

```
/.deespec/var/
/.deespec/var/*
!/.deespec/var/.keep
```

## よくある質問

- **prompts をプロジェクトと共有したい** → `.deespec/prompts/` を VCS 管理
- **specs もコミットしたい** → 案件方針で可否決定（秘匿要件に注意）
- **旧ファイルの互換** → v1 では **なし**（`.deespec` に統一）