# 引き継ぎ書 v2（PBI対応＋将来RUN構想）

> 目的：このドキュメントだけで、**現状の設計／“今日からの実運用”／PBI→SBI化の拡張**／**将来のRUNオーケストレーション**／**壊してはいけないガード**が把握できる。

**対象バージョン**: deespec v1（拡張計画付き）
**WIP前提**: **SBIはWIP=1**、`run`（=現行実行）はSBIのみを見る。
**ID方針**: **ULID採用**（時系列性＋実質衝突ゼロ）。人間可読IDは別途スラグとして補助的に使用。

---

## 0. いま何を作っているか（1行）

**放置で回る小さな開発ワーカー**：長文はMarkdownでAIに渡し、**Goが配線/登録/実行/検査**を厳密に担保する **deespec v1**。
（v2計画：PBI→SBI生成の安全な注入、RUNの上位オーケストレーション）

---

## 1. ディレクトリ配置（v1基盤＋PBI拡張）

```
.deespec/
  etc/
    workflow.yaml                # steps[].prompt_pathのみ（長文はMD外出し）
    policies/review_policy.yaml
    # 将来: run.yaml（上位RUNオーケストレーション設定）
  prompts/
    system/                      # 長文テンプレ（MD）
  specs/
    sbi/                         # 既存：1タスク=1ディレクトリ（実行対象）
      SBI-<ulid>-<slug>/
        meta.yaml
        ...
    pbi/                         # 追加：PBI本体（実行非対象）
      PBI-<ulid>-<slug>/
        meta.yaml                # PBI定義（UnknownFields禁止）
        brief.md                 # 問題/背景/価値（短文）
        detail.md                # 仕様/制約/非機能/受入ヒント（長文）
        candidates/              # PBI→SBI候補置き場（手作業/Claudeで肉付け）
          SBI-CAND-0001.yaml
          ...
        hooks/ (任意)
          on_approve.sh
  var/
    state.json
    health.json
    journal.ndjson
    artifacts/
    inbox/                       # PBI承認済SBI候補の登録用キュー（atomic）
      sbi_ready_<ulid>.yaml
      archive/
    locks/
      sbi.lock
      pbi_generate.lock
      sbi_register.lock
    logs/
```

**原則**

* **`specs/pbi` と `specs/sbi` は完全分離**。`run` は引き続き **`specs/sbi` のみ**を見る。
* 書き込みは **Atomic Write（tmp→rename）**、**UTF-8/LF/末尾改行**。

---

## 2. 既存のガード（恒久・厳守）

### 2.1 journal（常に7キー）

```json
{"ts":"<UTC RFC3339Nano>","turn":N,"step":"plan|implement|test|review|done",
 "decision":"OK|NEEDS_CHANGES|PENDING","elapsed_ms":int,"error":"","artifacts":[...]}
```

* **UTC（Z）**、**配列**、**turn整合（/turnN/含む）**、**decision列挙（空は禁止。PENDINGを正）**

### 2.2 state/health

* `state.json`：`{"version":1,"step":"plan","turn":0,"meta.updated_at":"UTC"}`（**`current`禁止**）
* `health.json`：`{"ts":"UTC","turn":N,"step":"...","ok":bool,"error":""}`（**直近 error=="" で ok:true**）

### 2.3 workflow/prompt

* **`prompt_path`のみ**／**絶対・`..`禁止**／**UnknownFields禁止**
* 変数 `{turn,task_id,project_name,language}` 以外は **Fail-fast**
* `constraints.max_prompt_kb` 既定 **64KB**（1..512KB）—超過は **即エラー**
* **reviewだけ** `decision.regex`（既定 `^DECISION:\s+(OK|NEEDS_CHANGES)\s*$`）

### 2.4 I/O・運用

* **Atomic Write**／**UTF-8/LF**／`.gitignore` は `.deespec/var` 除外（冪等）
* **登録はGo CLI**：AIは **直接ファイルを書かない**
* **WIP=1**：常に1SBIのみ `doing`（`state.current_task_id` も1件）

---

## 3. PBI→SBI フロー（新設・非干渉）

### 3.1 CLI（新設コマンド案）

* `deespec pbi register`

    * 入力：`--stdin` or `--file`（PBI `meta.yaml`）
    * 生成：`.deespec/specs/pbi/PBI-<ulid>-<slug>/`（**atomic**）
* `deespec pbi generate --pbi-id <id> [--count N] [--template <path>]`

    * 用途：**SBI候補の雛形**を `candidates/` に生成（肉付けは人/Claude別スレ）
* `deespec pbi approve --pbi-id <id> --cand <file.yaml>`

    * 用途：選んだ候補を **`.deespec/var/inbox/` にatomic enqueue**（まだ正式SBIにしない）
* `deespec sbi generate`

    * 用途：inboxの候補を **スキーマ検査→ULID採番→`.deespec/specs/sbi/` に正式登録**（バッチ）

> これにより **run実行中でも** PBI側の作業は**完全非干渉**で進められ、**次ターン**から自然にSBIがpick対象になる。

### 3.2 スキーマ最小例

**PBI `meta.yaml`**

```yaml
id: PBI-<ulid>
title: "Short feature title"
por: 1
labels: [feature, backend]
status: DRAFT # or REFINING | READY
owner: you@example.com
links: []
```

**SBI候補（`candidates/SBI-CAND-0001.yaml`）**

```yaml
title: "Implement X for Y"
priority: 2
por: 1
depends_on: []
acceptance:
  - "doctor passes (prompt_path/size/placeholders)"
  - "journal has 7 keys; decision in {OK,NEEDS_CHANGES,PENDING}"
notes: "現行ガード互換"
```

> 本登録時に **ULID採番**＋`SBI-<ulid>-<slug>/meta.yaml` を生成。

### 3.3 Doctor/CI 拡張（PBI範囲）

* `deespec doctor --scope=pbi`：

    * PBI `meta.yaml` の UnknownFields禁止・必須キー
    * `brief.md` / `detail.md` のUTF-8/サイズ
    * `candidates/*.yaml` が **SBI互換スキーマ**を満たすか
    * `var/inbox/*.yaml` の健全性（壊れた候補無し）

---

## 4. RUNの将来構想（段階導入）

### 4.1 いま（現行）

* `deespec run --once` ＝ **SBI専用実行ワーカー**（以後は `deespec sbi run --once` にリネーム予定）
* **1ターンでbacklog読込→実行→終了**。新規SBIは**次ターン**から検出。

### 4.2 近い将来（内部ループの導入）

* `deespec sbi run --loop [--max-idle=3]`

    * **各ターン冒頭でbacklogを再読込**
    * idleが連続 `N` 回で終了（例：`3` は数回様子見して抜ける運用）
    * exit codes: `0=処理あり / 2=idle / 1=fatal`

### 4.3 上位RUN（オーケストレータ）— **PBI generate & HBが揃ってから着手**

* `deespec run --profile default` が **複数ワーカーを並列実行**：

    * `sbi run`（SBI実行／WIP=1）
    * `gc run`（ログ/アーティファクトのリテンション&ローテ）
    * `hb run`（外部サーバーへの状態通知）
    * （将来）`pbi agent`（候補生成/承認の自動オーケストレーション）
* プロセス分離＋ロック＋ログ分離。`journal.ndjson` の**追記は sbi のみ**、`gc` は**現行ファイル非タッチ**（ローテ対象は過去分）。

**`etc/run.yaml`（例：将来）**

```yaml
version: 1
profile: default
workers:
  - name: sbi
    cmd: ["deespec","sbi","run","--loop","--max-idle=3"]
    schedule: "always"
    lock: ".deespec/var/locks/sbi.lock"
    restart: "on-failure"
  - name: gc
    cmd: ["deespec","gc","--logs-retain-days","14","--artifacts-retain-days","30"]
    schedule: "cron:15 3 * * *"
    lock: ".deespec/var/locks/gc.lock"
  - name: hb
    cmd: ["deespec","hb","run","--endpoint","https://api.example.com/health"]
    schedule: "every:60s"
    lock: ".deespec/var/locks/hb.lock"
```

---

## 5. 外部ハートビート（HB）計画（先に実装）

**目的**：`state.json` / `health.json` を **60秒ごと**に外部へ通知（観測可能性・運用可視化）
**CLI**：`deespec hb run --endpoint <URL> [--api-key ***] [--interval 60s]`
**送信例（JSON）**：

```json
{
  "ts": "2025-09-26T07:00:00Z",
  "turn": 42,
  "step": "review",
  "ok": true,
  "error": "",
  "agent": {
    "host": "devbox-01",
    "pid": 12345,
    "version": "v1.0.0"
  }
}
```

**リトライ**：一時失敗は指数バックオフ、連続失敗でも sbi の実行には非干渉。

---

## 6. GC（パージ/ローテ）計画（任意）

**CLI**：`deespec gc --artifacts-retain-days 30 --logs-retain-days 14 [--dry-run]`
**方針**：

* `artifacts/` と `logs/` を**対象限定**で削除/圧縮。
* `journal.ndjson` は **現行ファイル非対象**、月次ローテのみ（例：`journal-2025-09.ndjson.gz`）。

---

## 7. ULID 運用ルール

* **採番対象**：SBI/PBI/候補/inboxの**主キー**。
* **ディレクトリ名**：`<KIND>-<ulid>-<slug>`（slugは任意）。
* **互換**：旧形式を読み込む場合は**移行時にsid等を付与**（前方互換）。
* **重複**：現実的に無視可能＋**atomic-rename**でレース回避。

---

## 8. Exit Codes（標準化）

* `sbi run --once`：`0=処理あり` / `2=idle（処理なし）` / `1=fatal`
* `gc`：`0=成功` / `1=エラー`
* `hb`：`0=送信成功` / `1=一時失敗`
* 上位RUN（将来）：各workerの状態から要約（監視・CIで活用）

---

## 9. 受け入れ基準（サンプル）

1. **PBI登録→候補→承認→SBI登録**が **run非干渉**で通る
2. `doctor --scope=pbi` がPBI/候補/inboxを検査し、欠落/サイズ超/UnknownFieldsでブロック
3. `sbi run --once` が既存SBIを進め、**journal7キー/decision3値**等のガードが維持
4. `sbi run --loop` で各ターン先頭にbacklog再読込、`--max-idle=N`で穏当に抜ける
5. `hb run` が 60秒で心拍送信、ネットワーク失敗でも sbi の進行に影響なし
6. `gc` が現行 `journal.ndjson` を汚さずにローテ/削除できる

---

## 10. 直近のタスク（優先順）

* **PBI-REG-001**：`deespec pbi register`（atomic／UnknownFields禁止）
* **PBI-GEN-001**：`deespec pbi generate`（雛形生成）
* **PBI-APP-001**：`deespec pbi approve`（inbox enqueue；atomic）
* **SBI-GEN-002**：`deespec sbi generate`（inbox→正式SBI登録／ULID採番）
* **HB-001**：`deespec hb run`（外部通知・再送）
* **DR-PBI-001**：`doctor --scope=pbi`（PBI/候補/inbox健全性）
* （後段）**SBI-RUN-RENAME-001**：`run`→`sbi run` リネーム（旧コマンドは当面エイリアス）
* （後段）**RUN-ORCH-001**：上位 `deespec run`（orchestration；`etc/run.yaml`）

---

## 11. よく使うコマンド（テンプレ）

```bash
# 1ターン（SBI）
deespec sbi run --once
jq . .deespec/var/health.json
tail -n1 .deespec/var/journal.ndjson | jq .

# doctor（配線＋policy＋var）
deespec doctor
deespec doctor --scope=pbi   # PBI/候補/inbox検査

# Echo緑化スモーク
DEE_AGENT_BIN=echo deespec sbi run --once
jq . .deespec/var/health.json  # "ok": true

# PBI→SBI（例）
deespec pbi register --file pbi_meta.yaml
deespec pbi generate --pbi-id PBI-<ulid> --count 3
# （候補を手修正 or Claudeで肉付け）
deespec pbi approve --pbi-id PBI-<ulid> --cand candidates/SBI-CAND-0002.yaml
deespec sbi generate
```

---

## 12. DECISIONS（要点）

* **配線＝YAML、長文＝MD**（分離は厳守）
* **SBI実行はWIP=1**（`run`はSBIのみを見る）
* **PBIは非干渉レイヤ**として導入（`specs/pbi` と `var/inbox`）
* **IDはULID**、atomic-renameでレース回避
* **doctor/CI** で **配線・サイズ・未定義 `{}`・journalスキーマ** を守る
* RUNの二層化は **PBI generate／外部HB** 実装後に段階導入

