# Amazon Linux での systemd 運用ガイド

DeeSpecをAmazon Linux環境でsystemdサービスとして常駐実行するための完全ガイドです。

## 目次

- [システム要件](#システム要件)
- [リソース制限の目安](#リソース制限の目安)
- [インスタンスタイプ別推奨構成](#インスタンスタイプ別推奨構成)
- [systemd設定詳細](#systemd設定詳細)
- [セットアップ手順](#セットアップ手順)
- [運用管理](#運用管理)
- [トラブルシューティング](#トラブルシューティング)
- [ベストプラクティス](#ベストプラクティス)

---

## システム要件

### 最小要件

| 項目 | 最小値 | 推奨値 |
|------|--------|--------|
| **OS** | Amazon Linux 2 / AL2023 | AL2023 |
| **CPU** | 1 vCPU | 2 vCPU以上 |
| **メモリ** | 1GB | 2GB以上 |
| **ディスク** | 5GB空き容量 | 10GB以上 |
| **ネットワーク** | インターネット接続 | 安定した接続 |

### 対応バージョン

- **Amazon Linux 2**: systemd 219以降
- **Amazon Linux 2023**: systemd 252以降（推奨）
- **Go**: 1.22以降（ビルド時）

---

## リソース制限の目安

### 並列実行数の決定方法

並列実行数（`--parallel`）は以下の式で決定します：

```
推奨並列数 = min(利用可能vCPU数 - 1, タスク数)
```

**計算例:**
- 2 vCPU環境: `--parallel 1`（1タスク、残り1 vCPUはシステム用）
- 4 vCPU環境: `--parallel 3`（3タスク、残り1 vCPUはシステム用）
- 8 vCPU環境: `--parallel 7`（7タスク、残り1 vCPUはシステム用）

### メモリ制限の計算

```
DeeSpec用メモリ = (総メモリ - システム予約 - 他アプリ使用量) × 0.7
```

**計算例:**
- 2GB環境: `MemoryMax=700M` (2GB - 500MB予約 - 300MB他アプリ) × 0.7
- 4GB環境: `MemoryMax=2G` (4GB - 500MB - 500MB) × 0.7
- 8GB環境: `MemoryMax=5G` (8GB - 500MB - 500MB) × 0.7

### CPU制限の推奨値

| 環境 | CPUQuota | 説明 |
|------|----------|------|
| **単独実行** | 80-90% | DeeSpecがメインプロセス |
| **VSCode併用** | 40-50% | Web VSCodeと共存 |
| **多数アプリ併用** | 30-40% | 複数サービスが稼働 |
| **バックグラウンド** | 20-30% | 低優先度タスク |

### 実行間隔の推奨値

| 間隔 | 用途 | CPUインパクト |
|------|------|--------------|
| `1s` | 即座にタスク実行（常駐向け） | 低 |
| `5s` | バランス型（デフォルト） | 極低 |
| `30s` | リソース節約型 | 無視できる |
| `5m` | バッチ処理型（timer推奨） | 無視できる |

---

## インスタンスタイプ別推奨構成

### t2.micro / t3.micro (1 vCPU, 1GB)

**用途:** 開発・テスト環境

```ini
[Service]
ExecStart=/usr/local/bin/deespec run --interval 5s
MemoryMax=500M
MemoryHigh=400M
CPUQuota=70%
```

- ✅ シーケンシャル実行のみ
- ✅ 長めのインターバル（5秒）
- ⚠️ 本番環境非推奨

### t2.small / t3.small (1 vCPU, 2GB)

**用途:** 小規模プロジェクト

```ini
[Service]
ExecStart=/usr/local/bin/deespec run --interval 1s
MemoryMax=1G
MemoryHigh=800M
CPUQuota=70%
```

- ✅ VSCodeとの共存可能
- ✅ シーケンシャル実行
- ⚠️ 並列実行は非推奨

### t2.medium / t3.medium (2 vCPU, 4GB)

**用途:** 標準環境（推奨）

```ini
[Service]
ExecStart=/usr/local/bin/deespec run --interval 1s --parallel 1
MemoryMax=2G
MemoryHigh=1.5G
CPUQuota=50%
```

- ✅ VSCodeと快適に共存
- ✅ 並列実行も可能
- ✅ 本番環境として推奨

### t2.large / t3.large (2 vCPU, 8GB)

**用途:** 複数ユーザー・高負荷環境

```ini
[Service]
ExecStart=/usr/local/bin/deespec run --interval 1s --parallel 2 --auto-fb
MemoryMax=4G
MemoryHigh=3G
CPUQuota=60%
```

- ✅ 複数タスク並行処理
- ✅ 自動FB登録推奨
- ✅ 複数ユーザー対応

### c5.xlarge / c6i.xlarge (4 vCPU, 8GB)

**用途:** 高パフォーマンス環境

```ini
[Service]
ExecStart=/usr/local/bin/deespec run --interval 1s --parallel 3 --auto-fb
MemoryMax=5G
MemoryHigh=4G
CPUQuota=70%
```

- ✅ 高速タスク処理
- ✅ 大量タスク対応
- ✅ エンタープライズ推奨

---

## systemd設定詳細

### 基本テンプレート

```ini
[Unit]
Description=DeeSpec Continuous Task Runner
Documentation=https://github.com/YoshitsuguKoike/deespec
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ec2-user
Group=ec2-user
WorkingDirectory=/home/ec2-user/workspace/deespec

# 実行コマンド
ExecStart=/usr/local/bin/deespec run --interval 1s --log-level info

# シャットダウン設定
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30s

# 再起動ポリシー
Restart=always
RestartSec=10s
StartLimitInterval=300s
StartLimitBurst=5

# リソース制限
MemoryMax=1G
MemoryHigh=800M
CPUQuota=50%

# ログ設定
StandardOutput=journal
StandardError=journal
SyslogIdentifier=deespec

# セキュリティ
NoNewPrivileges=true
PrivateTmp=true
ReadWritePaths=/home/ec2-user/workspace/deespec/.deespec

[Install]
WantedBy=multi-user.target
```

### パラメータ詳細説明

#### [Unit] セクション

| パラメータ | 説明 | 推奨値 |
|-----------|------|--------|
| `After=` | 起動順序の制御 | `network-online.target` |
| `Wants=` | 依存サービス（ソフト） | `network-online.target` |
| `Requires=` | 依存サービス（ハード） | 通常不要 |

#### [Service] セクション - Type

| Type | 説明 | DeeSpecでの推奨度 |
|------|------|-------------------|
| `simple` | フォアグラウンド実行 | ⭐⭐⭐⭐⭐ 推奨 |
| `forking` | バックグラウンド化 | ❌ 非推奨 |
| `oneshot` | 1回実行（timer用） | ⭐⭐⭐ timer併用時 |
| `notify` | 起動完了通知 | ⚠️ 未対応 |

#### [Service] セクション - 再起動ポリシー

| パラメータ | 説明 | 推奨値 |
|-----------|------|--------|
| `Restart=` | 再起動条件 | `always` |
| `RestartSec=` | 再起動待機時間 | `10s` |
| `StartLimitInterval=` | 制限期間 | `300s` (5分) |
| `StartLimitBurst=` | 期間内の最大再起動回数 | `5` |

**動作例:**
- 5分間に5回以上クラッシュした場合、自動再起動を停止
- 手動で `systemctl reset-failed` を実行するまで起動不可

#### [Service] セクション - リソース制限

| パラメータ | 説明 | 制限範囲 |
|-----------|------|----------|
| `MemoryMax=` | 最大メモリ（超過時kill） | 256M～16G |
| `MemoryHigh=` | 高水位マーク（スワップ発動） | MemoryMaxの70-80% |
| `CPUQuota=` | CPU使用率上限 | 10%～200% |
| `TasksMax=` | 最大プロセス/スレッド数 | 通常4096（デフォルト） |

**CPUQuota計算:**
- 1 vCPU = 100%
- 2 vCPU = 200%
- 例: 2 vCPUで50%制限 = 1 vCPU相当

#### [Service] セクション - シャットダウン制御

| パラメータ | 説明 | 推奨値 |
|-----------|------|--------|
| `KillMode=` | プロセス終了方法 | `mixed` |
| `KillSignal=` | 送信シグナル | `SIGTERM` |
| `TimeoutStopSec=` | 強制終了までの猶予 | `30s` |

**KillMode詳細:**
- `mixed`: メインプロセスにSIGTERM、子プロセスにSIGKILL（推奨）
- `control-group`: 全プロセスにSIGTERMの後SIGKILL
- `process`: メインプロセスのみ終了

#### [Service] セクション - セキュリティ強化

| パラメータ | 説明 | 推奨 |
|-----------|------|------|
| `NoNewPrivileges=true` | 特権昇格を禁止 | ✅ 有効化 |
| `PrivateTmp=true` | /tmpを隔離 | ✅ 有効化 |
| `ReadWritePaths=` | 書き込み許可パス | `.deespec`のみ |
| `ProtectSystem=strict` | システムディレクトリ保護 | ⚠️ 要テスト |
| `ProtectHome=true` | ホームディレクトリ保護 | ❌ 非推奨 |

---

## セットアップ手順

### 1. deespecのインストール

```bash
# 標準インストール
curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.sh | bash

# PATHの確認
export PATH="$HOME/.local/bin:$PATH"
which deespec
```

### 2. 作業ディレクトリの準備

```bash
# プロジェクトディレクトリの作成
mkdir -p ~/workspace/myproject
cd ~/workspace/myproject

# deespecの初期化
deespec init

# 設定の確認
ls -la .deespec/
```

### 3. systemd serviceファイルの作成

```bash
# serviceファイルを作成（環境に応じて調整）
sudo tee /etc/systemd/system/deespec.service <<'EOF'
[Unit]
Description=DeeSpec Continuous Task Runner
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ec2-user
Group=ec2-user
WorkingDirectory=/home/ec2-user/workspace/myproject

# 環境に応じて調整
ExecStart=/home/ec2-user/.local/bin/deespec run --interval 1s --log-level info

KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30s

Restart=always
RestartSec=10s
StartLimitInterval=300s
StartLimitBurst=5

# リソース制限（環境に応じて調整）
MemoryMax=1G
MemoryHigh=800M
CPUQuota=50%

StandardOutput=journal
StandardError=journal
SyslogIdentifier=deespec

NoNewPrivileges=true
PrivateTmp=true
ReadWritePaths=/home/ec2-user/workspace/myproject/.deespec

[Install]
WantedBy=multi-user.target
EOF
```

### 4. サービスの有効化

```bash
# systemd設定の再読み込み
sudo systemctl daemon-reload

# サービスの有効化（自動起動）
sudo systemctl enable deespec.service

# サービスの起動
sudo systemctl start deespec.service

# 状態確認
sudo systemctl status deespec.service
```

### 5. 動作確認

```bash
# サービスが起動しているか確認
sudo systemctl is-active deespec.service

# ログの確認
sudo journalctl -u deespec.service -n 50

# リアルタイムログ
sudo journalctl -u deespec.service -f
```

---

## 運用管理

### 基本操作

```bash
# サービスの起動
sudo systemctl start deespec.service

# サービスの停止
sudo systemctl stop deespec.service

# サービスの再起動
sudo systemctl restart deespec.service

# サービスのリロード（設定変更時）
sudo systemctl reload deespec.service

# 状態確認
sudo systemctl status deespec.service

# 自動起動の有効化
sudo systemctl enable deespec.service

# 自動起動の無効化
sudo systemctl disable deespec.service
```

### ログ管理

```bash
# 最新のログを表示
sudo journalctl -u deespec.service -n 100

# リアルタイムでログを表示
sudo journalctl -u deespec.service -f

# 今日のログのみ表示
sudo journalctl -u deespec.service --since today

# 特定期間のログを表示
sudo journalctl -u deespec.service --since "2025-01-01" --until "2025-01-31"

# エラーのみ表示
sudo journalctl -u deespec.service -p err

# ログをファイルに出力
sudo journalctl -u deespec.service --since today > deespec-$(date +%Y%m%d).log
```

### リソース監視

```bash
# 現在のリソース使用状況
sudo systemctl show deespec.service -p MemoryCurrent,CPUUsageNSec

# プロセス一覧
ps aux | grep deespec

# 詳細な統計情報
systemd-cgtop
```

### 設定変更の反映

```bash
# 1. serviceファイルを編集
sudo vim /etc/systemd/system/deespec.service

# 2. systemd設定を再読み込み
sudo systemctl daemon-reload

# 3. サービスを再起動
sudo systemctl restart deespec.service

# 4. 変更を確認
sudo systemctl show deespec.service
```

---

## トラブルシューティング

### サービスが起動しない

**症状:** `systemctl start` が失敗する

**確認手順:**

```bash
# 1. エラーメッセージを確認
sudo systemctl status deespec.service

# 2. 詳細ログを確認
sudo journalctl -u deespec.service -n 100 --no-pager

# 3. 設定ファイルの構文チェック
sudo systemd-analyze verify deespec.service

# 4. 手動実行でテスト
cd /home/ec2-user/workspace/myproject
deespec run --interval 1s
```

**よくある原因:**

1. **実行ファイルのパスが間違っている**
   ```bash
   # 正しいパスを確認
   which deespec
   # serviceファイルのExecStartを修正
   ```

2. **作業ディレクトリが存在しない**
   ```bash
   # ディレクトリを作成
   mkdir -p /home/ec2-user/workspace/myproject
   cd /home/ec2-user/workspace/myproject
   deespec init
   ```

3. **権限の問題**
   ```bash
   # 所有者を確認
   ls -ld /home/ec2-user/workspace/myproject
   # 必要に応じて修正
   sudo chown -R ec2-user:ec2-user /home/ec2-user/workspace/myproject
   ```

### サービスが頻繁にクラッシュする

**症状:** `Restart=always` で何度も再起動する

**確認手順:**

```bash
# 1. クラッシュログを確認
sudo journalctl -u deespec.service -p err -n 50

# 2. リソース超過をチェック
sudo systemctl show deespec.service -p MemoryCurrent,MemoryMax

# 3. OOM Killerを確認
sudo dmesg | grep -i "out of memory"
sudo dmesg | grep -i "oom"
```

**対処法:**

1. **メモリ不足の場合**
   ```ini
   # serviceファイルのメモリ制限を緩和
   MemoryMax=2G  # 1G から増やす
   ```

2. **CPU制限が厳しい場合**
   ```ini
   # CPU制限を緩和
   CPUQuota=70%  # 50% から増やす
   ```

3. **並列実行数が多すぎる場合**
   ```ini
   # 並列数を減らす
   ExecStart=/usr/local/bin/deespec run --interval 1s --parallel 1
   ```

### ログが出力されない

**症状:** `journalctl` にログが表示されない

**確認手順:**

```bash
# 1. journaldが動作しているか確認
sudo systemctl status systemd-journald

# 2. ログレベルを確認
sudo journalctl -u deespec.service --show-cursor

# 3. StandardOutput/StandardErrorの設定を確認
sudo systemctl show deespec.service -p StandardOutput,StandardError
```

**対処法:**

```ini
# serviceファイルのログ設定を確認
StandardOutput=journal
StandardError=journal
SyslogIdentifier=deespec
```

### リソース制限が効かない

**症状:** 設定した制限を超えてリソースを使用する

**確認手順:**

```bash
# 1. cgroup v2が有効か確認
mount | grep cgroup

# 2. 実際の制限値を確認
sudo systemctl show deespec.service -p MemoryMax,CPUQuota

# 3. 現在の使用量を確認
systemd-cgtop
```

**対処法:**

1. **cgroup v2を有効化（AL2の場合）**
   ```bash
   # カーネルパラメータを追加
   sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
   sudo reboot
   ```

2. **制限値の単位を確認**
   ```ini
   MemoryMax=1G     # 正しい
   MemoryMax=1000M  # 正しい
   MemoryMax=1      # 間違い（バイト単位）
   ```

### StartLimitBurst超過で起動不可

**症状:** `start request repeated too quickly` エラー

**確認手順:**

```bash
# 1. エラーメッセージを確認
sudo systemctl status deespec.service

# 2. 失敗カウントをリセット
sudo systemctl reset-failed deespec.service

# 3. 再度起動を試行
sudo systemctl start deespec.service
```

**対処法:**

```ini
# StartLimitの設定を緩和
[Service]
StartLimitInterval=600s  # 300s から延長
StartLimitBurst=10       # 5 から増やす
```

---

## ベストプラクティス

### 1. 段階的なリソース調整

```bash
# Step 1: 最小構成で動作確認
MemoryMax=500M
CPUQuota=30%
--parallel 1

# Step 2: 負荷テスト
# 実際のタスクを実行して監視

# Step 3: リソースを徐々に増やす
MemoryMax=1G
CPUQuota=50%

# Step 4: 最適値を見つける
# 安定動作する最小リソースが最適
```

### 2. ログローテーションの設定

```bash
# journaldのログサイズを制限
sudo vim /etc/systemd/journald.conf

# 以下を設定
[Journal]
SystemMaxUse=1G
SystemKeepFree=2G
MaxRetentionSec=1month
```

### 3. アラート設定（オプション）

```bash
# systemdでのアラート（メール送信など）
sudo tee /etc/systemd/system/deespec-failure@.service <<'EOF'
[Unit]
Description=DeeSpec Failure Alert for %i

[Service]
Type=oneshot
ExecStart=/usr/local/bin/send-alert.sh "DeeSpec failed: %i"
EOF
```

### 4. バックアップの自動化

```bash
# 定期的な状態バックアップ
sudo tee /etc/systemd/system/deespec-backup.service <<'EOF'
[Unit]
Description=DeeSpec State Backup

[Service]
Type=oneshot
User=ec2-user
ExecStart=/usr/local/bin/backup-deespec-state.sh
EOF

sudo tee /etc/systemd/system/deespec-backup.timer <<'EOF'
[Unit]
Description=Daily DeeSpec Backup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
EOF
```

### 5. モニタリングダッシュボード

```bash
# systemd-cgtopでリアルタイム監視
watch -n 5 'systemctl show deespec.service -p MemoryCurrent,CPUUsageNSec'

# または、Prometheusエクスポーターを使用
# node_exporter などでメトリクスを収集
```

### 6. 本番環境チェックリスト

- [ ] リソース制限が適切に設定されている
- [ ] 自動起動が有効化されている（`enable`）
- [ ] ログローテーションが設定されている
- [ ] バックアップ体制が整っている
- [ ] アラート通知が設定されている
- [ ] 負荷テストを実施済み
- [ ] ドキュメントが最新である

---

## まとめ

### クイックリファレンス

| 環境 | 並列数 | メモリ | CPU | インターバル |
|------|--------|--------|-----|------------|
| **開発** | 1 | 500M | 30% | 5s |
| **小規模** | 1 | 1G | 50% | 1s |
| **標準** | 2 | 2G | 50% | 1s |
| **大規模** | 3-5 | 4G | 70% | 1s |

### 重要ポイント

1. ✅ **Type=simple** を使用する（フォアグラウンド実行）
2. ✅ **Restart=always** で自動復旧を有効化
3. ✅ リソース制限を**必ず設定**する
4. ✅ ログを**journald**で一元管理
5. ✅ 段階的にリソースを調整する

### サポート

- GitHub Issues: https://github.com/YoshitsuguKoike/deespec/issues
- Documentation: https://github.com/YoshitsuguKoike/deespec/docs

---

**最終更新:** 2025-10-11
**対象バージョン:** deespec v0.2.1+
