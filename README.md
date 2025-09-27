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

### Development/Testing

**Fsync Audit Mode**: For testing data durability guarantees:
```bash
# Enable fsync audit mode (build tag)
go test -tags fsync_audit ./...

# Or via environment variable
DEESPEC_FSYNC_AUDIT=1 go test ./...
```

This tracks all fsync operations to verify proper data persistence.

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

