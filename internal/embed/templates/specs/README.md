# .deespec/specs — タスク仕様（v1）

## 方針
- **1タスク=1ディレクトリ**（SBI/PBI）で管理します。
- 長文の指示/受入は Markdown。配線は `.deespec/etc/workflow.yaml`（prompt_path参照）。
- 登録/生成は **AIではなく Go CLI**：`deespec sbi register ...` 経由で行います。

## 構造
```
.deespec/specs/
  sbi/SBI-[pbiId|0]-slug/
    instruction.md      # 指示書（必須）
    acceptance.md       # 受入基準（必須）
    meta.yaml           # メタ（必須）
    impl_note.md?       # 実装ノート（任意）
    review_note.md?     # レビュー/判定（任意、末尾に DECISION 行）
  pbi/PBI-xxx-slug/
    requirement.md      # PBI要件（必須）
```

## 命名規約
- **SBI**: `SBI-[pbiId|0]-<slug>/`（PBIなしは `0`）
- **PBI**: `PBI-<数字>-<slug>/`
- **ID**: `^SBI-\d{3,}$` / `^PBI-\d{3,}$` を原則とする（CLIで検証）。

## 登録
- `cat sbi.json | deespec sbi register --stdin [--pick]`
- `--from-dir` で既存素材を取り込み（不足はテンプレ補完）

## 不能時（不完全指示）
- 実装者: `impl_unimplementable` テンプレで `impl_note.md` に記録 → `NEEDS_CHANGES`
- レビュア: `unreviewable` テンプレで `review_note.md` に記録 → `NEEDS_CHANGES`