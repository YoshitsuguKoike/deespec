# レビュー（{project_name} / turn={turn} / task={task_id}）

観点（箇条書き）：
- 仕様準拠：acceptance満たすか
- セキュリティ：危険パス/秘密情報混入なし
- 可観測性：journal/health/CIを壊していないか
- diffの妥当性：不要な変更や欠落がないか

出力：
- 上記観点の指摘（必要な場合のみ）
- **最下行に判定**：`DECISION: OK` または `DECISION: NEEDS_CHANGES` を**1行だけ**出力