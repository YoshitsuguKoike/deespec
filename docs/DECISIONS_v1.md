# DECISIONS v1（合意事項と設計原則）

## 要点
- `.deespec` 集約（旧互換なし）：etc/prompts/specs/var に一本化
- 長文は Markdown 外出し、`workflow.yaml` は prompt_path のみ
- SBI登録は **Go CLI** 経由（AIは直接書かない）
- journal 7キー＋UTC＋turn整合＋decision列挙、CI は `.deespec/var/journal.ndjson` を唯一対象
- フィードバック上限超で Takeover→Re-Eval→反省/ナレッジ記録（policy適用）
- セッション：session_affinity/lease により並走/孤児採用を制御（将来導入）

## バージョニング
- v1.0.0：本構成をデフォルトに採用、旧互換削除