# deespec

## Installation

### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.sh | bash
```

### Windows (PowerShell)
```powershell
iwr -useb https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.ps1 | iex
```

### Quick start
```bash
deespec init
ls -1
# => workflow.yaml, state.json, .artifacts/
```

## Startup Sequence (Important)

**Transaction Recovery**: DeeSpec performs automatic transaction recovery at startup. This must happen **before acquiring any locks** to ensure data consistency.

The startup sequence is automatically handled by:
1. `RunStartupRecovery()` - Transaction recovery (before locks)
2. Lock acquisition for exclusive operation
3. Normal workflow execution

## Path Resolution and Environment Variables

- Path base: DeeSpec resolves paths relative to `DEE_HOME` if set; otherwise it falls back to a local `.deespec` under the project. For TX commit/recovery dest root, the priority is:
  1) `DEESPEC_TX_DEST_ROOT` → 2) `DEE_HOME` → 3) `.deespec`.

- Environment variables:
  - `DEE_HOME`: Base directory for DeeSpec (typically `<project>/.deespec`).
  - `DEESPEC_TX_DEST_ROOT`: Explicit destination root for TX finalization (highest priority).
  - `DEESPEC_DISABLE_RECOVERY`: Set `1` to skip startup recovery.
  - `DEESPEC_DISABLE_STATE_TX`: Set `1` to disable TX mode for state/journal (legacy direct write).
  - `DEESPEC_DISABLE_METRICS_ROTATION`: Set `1` to disable metrics rotation (stabilize tests).
  - `DEESPEC_FSYNC_AUDIT`: Set `1` (and/or build tag `fsync_audit`) to enable fsync audit.
  - `DEE_STRICT_FSYNC`: Set `1` to treat fsync failures as errors (default is WARN).

## Atomic Writes and Temp Files

- All atomic writes create a unique temporary file in the same directory as the destination using `os.CreateTemp(dir, pattern)`, then follow: write → `fsync(file)` → `close()` → `rename()` → `fsync(parent dir)`.
- This avoids temp-name collisions under concurrency and guarantees same-filesystem atomicity.

### Development/Testing

**Fsync Audit Mode**: For testing data durability guarantees:
```bash
# Enable fsync audit mode (build tag)
go test -tags fsync_audit ./...

# Or via environment variable
DEESPEC_FSYNC_AUDIT=1 go test ./...
```

This tracks all fsync operations to verify proper data persistence.

## Quality Gates & CI Integration

**Metrics-based Quality Gates**: DeeSpec provides automated quality checking through metrics thresholds for CI/CD integration.

### Default Threshold Configuration

```bash
# Success rate thresholds (recommended values)
EXCELLENT: ≥95%   # Production-ready quality
GOOD:      ≥90%   # Acceptable for most scenarios
WARNING:   ≥80%   # Investigation recommended
CRITICAL:  <80%   # Immediate action required
```

### CI/CD Integration Examples

**Basic Quality Gate (GitHub Actions)**:
```yaml
- name: Check DeeSpec Quality
  run: |
    # Fail CI if success rate below 90%
    deespec doctor --json | jq '.metrics.success_rate >= 90' -e
```

**Advanced Multi-threshold Check**:
```yaml
- name: Quality Assessment
  run: |
    RESULT=$(deespec doctor --json | jq '
      if .metrics.success_rate >= 95 then "EXCELLENT"
      elif .metrics.success_rate >= 90 then "GOOD"
      elif .metrics.success_rate >= 80 then "WARNING"
      else "CRITICAL" end' -r)

    echo "Quality: $RESULT"

    # Fail only on CRITICAL
    if [ "$RESULT" = "CRITICAL" ]; then
      exit 1
    fi
```

**Team Agreement Template**:
```bash
# チーム合意しきい値設定例

# 本番リリース必須条件
deespec doctor --json | jq '.metrics.success_rate >= 95 and .metrics.cas_conflicts <= 10' -e

# 開発フィーズ品質基準
deespec doctor --json | jq '.metrics.success_rate >= 85' -e

# ナイトリービルド警告レベル
deespec doctor --json | jq '
  .metrics.success_rate as $sr |
  .metrics.cas_conflicts as $cc |
  if $sr < 80 or $cc > 50 then "WARN: Quality degradation detected"
  else "OK" end' -r
```

### Monitoring Integration

**Prometheus Metrics Export** (future enhancement):
```bash
# Convert doctor --json to Prometheus format
deespec doctor --json | jq -r '
  "deespec_success_rate \(.metrics.success_rate)",
  "deespec_total_commits \(.metrics.total_commits)",
  "deespec_cas_conflicts \(.metrics.cas_conflicts)"'
```

**Slack Alert Integration**:
```bash
#!/bin/bash
QUALITY=$(deespec doctor --json | jq '.metrics.success_rate')
if (( $(echo "$QUALITY < 85" | bc -l) )); then
  curl -X POST -H 'Content-type: application/json' \
    --data "{\"text\":\"⚠️ DeeSpec quality dropped to ${QUALITY}%\"}" \
    $SLACK_WEBHOOK_URL
fi
```

**Build Tags Reference:**
- `fsync_audit`: Enable fsync monitoring and detailed audit logging
- Default (no tags): Production optimized build

**Common Development Commands:**
```bash
# Run all tests with coverage
make test-coverage

# Run tests with fsync audit
go test -tags fsync_audit ./...

# Run coverage check
make coverage-check

# Generate HTML coverage report
make coverage-html
```

---

1. 【\[必須] 対応OS/CPU表】
   読者が自分の環境で動くか一目で判断できます。

   ```text
   | OS     | CPU     | 配布名例                  |
   |--------|---------|---------------------------|
   | macOS  | arm64   | deespec_darwin_arm64      |
   | macOS  | amd64   | deespec_darwin_amd64      |
   | Linux  | arm64   | deespec_linux_arm64       |
   | Linux  | amd64   | deespec_linux_amd64       |
   | Win10+ | amd64   | deespec_windows_amd64.exe |
   ```

2. 【\[必須] 最短スモークテスト】
   インストール直後に“動いた”を確認する3行。

   ```bash
   # インストール後の最短動作確認
   deespec --help
   deespec init
   deespec run --once && deespec status --json
   ```

3. 【\[必須] 5分自走（運用導線）の入口】
   launchd/systemd の設定先リンク or 簡易手順を README に1ブロックだけ置く。

   ```bash
   # macOS (launchd) の例: 5分間隔で1ターン実行
   # ~/Library/LaunchAgents/com.deespec.runner.plist を作成後に:
   launchctl load  ~/Library/LaunchAgents/com.deespec.runner.plist
   launchctl start com.deespec.runner
   # 直近状態
   cat health.json | jq .
   ```

4. 【\[必須] 環境変数の例（.env.example への導線）】
   とくに macOS の launchd は PATH が狭いので、**絶対パス**記載の注意書きを。

   ```bash
   # .env の例
   DEE_AGENT_BIN=/usr/local/bin/claude   # ← 絶対パス推奨（launchdではPATHが狭い）
   DEE_TIMEOUT_SEC=60
   DEE_ARTIFACTS_DIR=.artifacts
   ```

5. 【\[推奨] トラブルシューティング（詰まり解消の一行）】

   ```bash
   # まずは自己診断（人間向け）
   deespec doctor

   # JSON形式で診断（自動化・監視向け）
   deespec doctor --json | jq .
   # exit code: 0=正常、2=警告（inactive/未設定）、1=重大（書込不可/agent不在）

   # メトリクス品質チェック（CI/CD統合用）
   deespec doctor --json | jq '.metrics.success_rate >= 90' -e
   # 成功率90%以上で exit code 0、未満で exit code 1

   # 実用的なしきい値チェック例
   deespec doctor --json | jq '
     if .metrics.success_rate >= 95 then "EXCELLENT"
     elif .metrics.success_rate >= 90 then "GOOD"
     elif .metrics.success_rate >= 80 then "WARNING"
     else "CRITICAL" end' -r

   # 5分自走の確認
   cat health.json | jq -r '.ts'  # タイムスタンプが5分毎に前進
   tail -n1 journal.ndjson | jq '.decision'  # PENDING/NEEDS_CHANGES/OK のいずれか

   # jq が未導入なら (macOS)
   brew install jq
   ```

6. 【\[推奨] アンインストール／更新】

   ```bash
   # 更新（再インストール）
   curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.sh | bash

   # アンインストール
   # /usr/local/bin または ~/.local/bin の deespec を削除
   rm -f /usr/local/bin/deespec ~/.local/bin/deespec
   ```

7. 【\[推奨] 付帯情報リンク】
    * [CHANGELOG](./CHANGELOG.md)


---

## \[注意] Windows ユーザー向け

PowerShell の実行ポリシーや PATH 反映で詰まりがちなので、以下を1行追記すると親切です。

```powershell
# PowerShell 実行ポリシーでエラーになる場合
# 管理者権限で実行し、必要に応じて:
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
```

---

````md
### Quick start
```bash
# インストール直後の最短スモーク
deespec --help
deespec init
deespec run --once && deespec status --json
````
