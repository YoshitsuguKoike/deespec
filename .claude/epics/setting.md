了解です。`./deespec/setting.json` を導入し、**init時に生成 → 実行時はJSONから読取（infra層）→ app層はロジックのみ**に分離する方針で、手戻り少なく移行できる作業ステップと指示書をまとめました。
（リポジトリは `cmd/deespec`, `internal`, `scripts`, `README` 等の構成が確認できました。READMEには `deespec init` / `deespec run` / `doctor --json` などの導線とENV例が記載されています。） ([GitHub][1])

---

# 作業ステップ（実装順）

## 0) 設計方針と互換ポリシー

* **設定の優先順位**（上書き規則）

    1. **CLIフラグ**（将来導入／既存があれば最上位）
    2. **`./deespec/setting.json`**
    3. **環境変数（現行）**
    4. **ビルドインのデフォルト値**
* **後方互換**: 既存の環境変数は**当面有効**（読み取りOK）。`setting.json`があればそちらを優先。廃止予定系は**warningログ**のみ出し、本体では使わない。
* **責務分離**:

    * infra層: ファイルI/O・環境変数の読取・マージ。
    * app層: `Config`の**インターフェース**に依存し、ビジネスロジックのみ。

## 1) JSONスキーマ定義（仕様）

`./deespec/setting.json` の最小スキーマ（キー＝意味は下表参照）

```json
{
  "home": ".deespec",
  "agent_bin": "claude",
  "timeout_sec": 60,
  "artifacts_dir": ".deespec/var/artifacts",
  "project_name": null,
  "language": null,
  "turn": null,
  "task_id": null,
  "validate": false,
  "auto_fb": false,
  "strict_fsync": false,

  "tx_dest_root": null,
  "disable_recovery": false,
  "disable_state_tx": false,
  "use_tx": false,
  "disable_metrics_rotation": false,
  "fsync_audit": false,
  "test_mode": false,
  "test_quiet": false,
  "workflow": null,
  "policy_path": null,
  "stderr_level": null
}
```

### 環境変数 ↔ JSONキー対応表

（右列が `setting.json` のキー名）

| 環境変数                | JSONキー          | 備考                            |
| ------------------- | --------------- | ----------------------------- |
| `DEE_HOME`          | `home`          | 既定 `.deespec`                 |
| `DEE_AGENT_BIN`     | `agent_bin`     | READMEのENV例でも登場 ([GitHub][1]) |
| `DEE_TIMEOUT_SEC`   | `timeout_sec`   | READMEのENV例でも登場 ([GitHub][1]) |
| `DEE_ARTIFACTS_DIR` | `artifacts_dir` | READMEのENV例でも登場 ([GitHub][1]) |
| `DEE_PROJECT_NAME`  | `project_name`  | 任意                            |
| `DEE_LANGUAGE`      | `language`      | 任意                            |
| `DEE_TURN`          | `turn`          | 任意（int or string許容）           |
| `DEE_TASK_ID`       | `task_id`       | 任意                            |
| `DEE_VALIDATE`      | `validate`      | `1/true`→true                 |
| `DEE_AUTO_FB`       | `auto_fb`       | `true`→true                   |
| `DEE_STRICT_FSYNC`  | `strict_fsync`  | `1`→true                      |

**機能制御（DEESPEC_系）**

| 環境変数                               | JSONキー                     | 既定/備考                       |
| ---------------------------------- | -------------------------- | --------------------------- |
| `DEESPEC_TX_DEST_ROOT`             | `tx_dest_root`             | 最優先で使うTX反映ルート               |
| `DEESPEC_DISABLE_RECOVERY`         | `disable_recovery`         | `1`→true（既定: false=有効）      |
| `DEESPEC_DISABLE_STATE_TX`         | `disable_state_tx`         | **廃止予定**→JSONでは保持しつつwarning |
| `DEESPEC_USE_TX`                   | `use_tx`                   | `1`→true                    |
| `DEESPEC_DISABLE_METRICS_ROTATION` | `disable_metrics_rotation` | `1`→true                    |
| `DEESPEC_FSYNC_AUDIT`              | `fsync_audit`              | `1`→true                    |
| `DEESPEC_TEST_MODE`                | `test_mode`                | `true`→true                 |
| `DEESPEC_TEST_QUIET`               | `test_quiet`               | `1/true`→true               |
| `DEESPEC_WORKFLOW`                 | `workflow`                 | パス                          |
| `DEESPEC_POLICY_PATH`              | `policy_path`              | パス                          |
| `DEESPEC_STDERR_LEVEL`             | `stderr_level`             | 例: info/warn/error          |

**廃止予定/将来対応**: JSONにも**記載は可**だが**未使用**。起動時に「未サポート設定」「今後の導入計画」などのINFO/WARNを出す。

---

## 2) ディレクトリ/ファイル追加（推奨構成）

* `internal/infra/config/loader.go`

    * JSON読取、環境変数読取、マージ、デフォルト適用を**ここに集約**。
* `internal/app/config/config.go`

    * `type Config interface { … }` と `struct AppConfig{…}`（ロジック層が使う表現）。
    * infraの型に依存せず、**必要なgetterのみ**公開。
* `internal/app/*`（既存ユースケース・サービス）

    * 既存の「環境変数直読み」の箇所を、**Config依存**に置換。
* `cmd/deespec/init.go`

    * `deespec init` 実行時に `./deespec/setting.json` を生成（存在しなければ）。初期値は上のスキーマのデフォルト値。READMEの「Quick start」に沿う出力と導線を維持。 ([GitHub][1])

---

## 3) 実装タスク詳細

### T1. Configインターフェース定義（app層）

* 目的: app層からは `Config` 経由で設定取得。直接I/Oをしない。
* 内容:

    * `internal/app/config/config.go`

        * `type Config interface { Home() string; AgentBin() string; TimeoutSec() int; … }`
        * `type AppConfig struct { … }`（infraから受け取ったDTOをマップ）
    * ※ 将来のテストで `fakeConfig` を差し替え可能に。

### T2. Loader（infra層）

* 目的: **1ファイル＋環境変数**のマージローダ。
* 内容:

    * `internal/infra/config/loader.go`

        * `type RawSettings struct { … }`（JSON直列化用）
        * `func Load(baseDir string) (RawSettings, error)`

            1. `./deespec/setting.json` を探す（`baseDir` からの相対 or 絶対）
            2. 無ければ空として扱う
            3. **環境変数を上書き反映**（上表のマッピングでbool/int変換）
            4. 未設定は**デフォルト補完**
            5. **Deprecatedキーの警告**ログ出力
        * `func BuildAppConfig(RawSettings) (config.AppConfig, error)`（型変換）

### T3. initコマンド拡張

* 目的: 初期化時に `./deespec/setting.json` を**安全に作成**。
* 内容:

    * `cmd/deespec/init.go`

        * 既存の初期化に加え、**親ディレクトリ作成**→**存在チェック**→**テンプレ生成**。
        * 既存の `deespec init` の期待出力（`workflow.yaml` 等）との整合性は維持。 ([GitHub][1])
    * 例: 既存ファイルがある場合は `--force` が無い限り上書きしない。

### T4. 実行パスの読取をConfig化

* 目的: 既存コードの「直接ENV参照」を排除。
* 内容:

    * `cmd/deespec/*.go` / `internal/app/*` の各ユースケースで **Config注入**に置換。
    * 代表的には「ホームディレクトリ」「artifacts先」「agentバイナリ」「timeout」など。

### T5. ログと自己診断(doctor)の更新

* 目的: ユーザが詰まらないよう可視化。
* 内容:

    * `deespec doctor --json` のJSON出力に `config_source`（`json|env|default`）や `setting_path` を追加。
    * READMEのトラブルシュート節に `setting.json` の説明を追記（`deespec doctor` 導線はすでに記載あり）。 ([GitHub][1])

---

## 4) 受け入れ基準（Acceptance Criteria）

1. **init**

    * `deespec init` 実行後、`./deespec/setting.json` が**なければ作成**される（既存なら保持）。
    * 内容は上記デフォルトで妥当なJSON。

2. **設定の優先順位**

    * 同じキーに `ENV` と `setting.json` が存在する場合、**`setting.json` が優先**される。
    * `ENV` しかない場合はENVが反映。どちらも無ければ**デフォルト**。

3. **実行動作**

    * `deespec run --once` 実行時、`agent_bin`/`timeout_sec`/`artifacts_dir` 等が `setting.json` の値で動作。
    * `doctor --json` に `config_source` が含まれ、読み込み元が判別できる。READMEの「最短スモークテスト」手順と併用して確認可能。 ([GitHub][1])

4. **後方互換**

    * `setting.json` を削除しても、**現行ENVだけで動く**。
    * **廃止予定キー**に関しては、ログにWARNを1回出すが、動作は阻害しない。

5. **Clean分離**

    * app層は`Config`インターフェース以外のI/Oに触れない。
    * infra層でのみファイル読み込み・ENV読取・型変換を実施。

---

## 5) テスト計画

### 単体テスト（Go, `-race`/cover）

* `internal/infra/config/loader_test.go`

    * ケース:

        1. JSONのみ
        2. ENVのみ
        3. JSON+ENV（JSON優先）
        4. 欠損→デフォルト補完
        5. bool/int文字列の変換（`"1"`, `"true"`, `"FALSE"` 等）
        6. 廃止予定キーのWARN発行
* `internal/app/config/config_test.go`

    * `RawSettings` → `AppConfig` 変換の境界値（ゼロ値、負値、空文字の扱い）。

### E2E/統合テスト（`cmd` ベース）

* `deespec init` → `setting.json` 生成確認 → `deespec run --once` → `deespec doctor --json` で `config_source` が `json` になることを検証（READMEのスモーク導線に接続）。 ([GitHub][1])

---

## 6) マイグレーションとドキュメント

### README更新（最短スモークの直後に1ブロック追記）

* 導入例:

  ```bash
  deespec init
  # ./deespec/setting.json が作成されます（存在時は保持）
  deespec run --once && deespec doctor --json | jq .
  ```
* `.env`例のすぐ下に、「**`setting.json`がある場合はENVより優先**」を明記（READMEにはENV例が既にあるので、併記の体裁でOK）。 ([GitHub][1])
* **Deprecated一覧**と廃止予定のロードマップ（例: v0.2.0で`DEESPEC_DISABLE_STATE_TX`完全削除）。

---

# 指示書（実装担当向け）

## 目的

* 設定の**外部化**と**再現性の向上**（CI/E2E/配布後の上書き容易化）。
* 責務分離によりテスタビリティ向上（Clean方向へ寄せる）。

## 対象ブランチ

* `feature/config-json` を作成しPR。

## 作業

1. `internal/app/config/config.go` で `Config` インターフェースと `AppConfig` 実装を追加。
2. `internal/infra/config/loader.go` を新規作成。

    * `Load(baseDir)` で JSON/ENV/Default マージ。
    * `BuildAppConfig()` でapp層の型に変換。
    * 廃止予定キーのWARN出力。
3. `cmd/deespec/init.go` を修正。`setting.json` 生成（セーフティ: 既存は保持、`--force`時のみ上書き）。
4. `cmd/deespec/*` / `internal/app/*` の直接ENV参照を**Config注入**に置換。
5. `deespec doctor --json` 出力に `config_source` と `setting_path` を追加。
6. 単体テスト/E2Eを追加。`make test` で `-race -cover` 合格を確認。
7. README更新（`setting.json`導線・優先順位・トラブルシュート）。

## 完了の定義（DoD）

* `deespec init` 後に `./deespec/setting.json` が生成される（初回のみ）。
* `setting.json` の値が `run/doctor` に反映され、`doctor --json` で読取元が確認可能。
* ENVのみでも従来通り動作する（互換維持）。
* 主要ユースケースが `Config` のみを通じて設定にアクセス。
* CIの `go test -v -race -coverprofile=...` がPASS。

---

# 参考（READMEの現状導線）

* Quick start: `deespec init` → `deespec run --once` → `deespec status --json` の流れが提示されています。`doctor` と `doctor --json` の導線も記載あり。ENV例もREADMEに明記されています（`DEE_AGENT_BIN`, `DEE_TIMEOUT_SEC`, `DEE_ARTIFACTS_DIR`）。これらに**`setting.json`の説明を追記**してください。 ([GitHub][1])

---

必要なら、このまま**雛形コード（`loader.go`/`config.go`）**も用意します。どの言語仕様（エラーハンドリング・ロガー・JSONタグ方針など）に合わせるか教えてくれれば即座に出します。

[1]: https://github.com/YoshitsuguKoike/deespec "GitHub - YoshitsuguKoike/deespec"
