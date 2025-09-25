# CHANGELOG

本プロジェクトは [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) 形式に準拠し、[Semantic Versioning](https://semver.org/lang/ja/) に従います。

## \[Unreleased]

### 追加予定

* `decision:""` を `"PENDING"` に統一するオプトイン設定（後方互換を維持）
* `doctor` の検査強化（`.artifacts` 書込権限／`journal.ndjson` 非JSON行検知）
* `scripts/metrics.py`（レビューOK率・平均 `elapsed_ms` のローカル集計）
* CI: NDJSON 検証ステップの堅牢化（非オブジェクト行混入検出、r011 対応）

---

## \[v0.1.11] - 2025-09-25

### 追加

* **自動運用**: 5分間隔のスケジューラ投入（macOS: `launchd`／Linux: `systemd timer`）
* **観測性**: `health.json` 出力（`ts/turn/step/ok/error`、UTC・毎ターン上書き）
* **ドキュメント**: README に 1行インストール＋Quick Start を整備

### 変更

* **時刻表記の統一**: すべての時刻を **UTC（`Z` 終端の RFC3339Nano）** に統一（`status --json` 含む）

### 修正

* なし（前版の修正を踏襲した上で運用導線を整備）

---

## \[v0.1.9] - 2025-09-25

### 追加

* **状態出力の連動**: `status --json` の `ok` を直近 `journal.ndjson` の `error` と連動

### 変更

* なし

### 修正

* **ターン整合性**: `state.Turn` の **インクリメント時機を“journal 追記後”に統一**
  → `journal.turn == ".artifacts/turnN/..."` を常時保証

---

## \[v0.1.8] - 2025-09-25

### 追加

* **検証強化（r008）**: 連続実行＋ジッター／失敗注入を含む追加テスト計画
* **スキーマ連動**: `status --json` の `ok` リンク（初実装）

### 変更

* **時刻表記**: `journal.ndjson.ts` を UTC（`Z`）に統一

### 修正

* **スキーマ安定**: `journal.ndjson` の **7キー** を欠落禁止で固定
  `ts, turn, step, decision, elapsed_ms, error, artifacts`（`artifacts` は常に **array**）

---

## \[v0.1.7] - 2025-09-25

### 追加

* **`status --json`（スケルトン）**: 最低限の機械可読ステータス出力を追加
* **`JournalWriter` / `NormalizeJournalEntry`**: 出力の一本化・ゼロ値補完

### 変更

* なし

### 修正

* `artifacts` の型を `[]string` に統一（`nil`・文字列混入を排除）

---

## \[v0.1.2] - 2025-09-25

### 追加

* **インストーラ**: `scripts/install.sh`（Linux/macOS）を整備（`/usr/local/bin` への配置、不可時 `~/.local/bin` へフォールバック、PATH 追記）
* **Windows**: `scripts/install.ps1` の導線準備（PowerShell ワンライナー）

### 変更

* なし

### 修正

* なし

---

## \[v0.1.1] - 2025-09-25

### 追加

* **配布導線**: GitHub Actions によるタグ駆動リリース（クロスビルド）
* README: Quick Start の雛形

### 変更

* なし

### 修正

* なし

---

## \[v0.1.0] - 2025-09-25

### 追加

* **MVP**: `init / status / run --once` の最小ワークフロー（plan→implement→test→review→done）
* **フォールバック**: 外部依存の失敗時にブーメラン（`NEEDS_CHANGES`）で前進を継続
* **排他**: CAS(version)＋lock による実行排他
* **ログ**: `journal.ndjson`（追記）と `.artifacts/` 生成

---

### 付記

* 2段階リリース（build→artifact集約→release一回）への切替は運用安定のため推奨です。
* 既存タグの再利用は GitHub 側の制約により不可（`422 already_exists`）。新タグでの発行が安全です。
