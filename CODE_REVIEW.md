# Code Review Contract

Review 只評估 requested change、適用規格、final diff 與必要的 adjacent contracts，
不因個人偏好擴張實作範圍。

## Review Priorities

依序檢查：

1. Correctness、資料 scope、unit conversion、cache consistency 與 user-visible
   regressions。
2. Credential/token/TLS、Inventory API authorization、secret redaction、輸入邊界與
   不受控 ECS 呼叫等安全問題。
3. Prometheus metric name/type/unit/label/cardinality、Inventory schema、
   configuration 與 Semantic Versioning 相容性。
4. Scheduler overlap、bounded concurrency、timeout、retry/backoff、pagination、
   circuit breaker、partial failure 與 stale-data recovery。
5. Tests 與 verification evidence 是否覆蓋正常、失敗、邊界、permission、
   compatibility 及 regression 情境。
6. 可能造成實際缺陷的 maintainability 問題。

純 style 偏好只有在違反已提交的 formatter/linter/content contract，或實質降低
安全與可理解性時才是 finding。

## Dell ECS and Metrics Invariants

Reviewer 必須確認：

- 實際 URI、schema 與欄位可追溯到已選定 ECS 版本的
  `DELL_ECS_API_MAPPING.md`，沒有推測值。
- Profile range、capability 與 known issue 必須與 `profiles/` 及 `docs/ecs-api/`
  一致；`candidate-inherited` 不得當成 sandbox-verified。
- `tested_builds` 只能由真實隔離 ECS evidence 更新，synthetic fixture 不足以加入。
- ECS 3.7/3.8.0 不得啟用 Flux interval-derived rate；3.8.1 必須通過 live
  range-boundary gate。
- ECS 3.8.0/3.8.1 經 proxy/load balancer 時必須保留 hostname、TLS SNI 與 Host
  一致，不得以 IP 繞過 accepted server names。
- `/metrics` 僅讀 cache，Prometheus scrape 不會觸發 ECS API。
- Collector 使用原子替換；不完整 page/resource 結果不覆蓋最後完整成功 snapshot。
- 未設定 quota 不輸出 quota metric，Inventory 回傳 `null` 與 configured flag，
  且不使用 cluster capacity 代替。
- Counter 只對來源的單調累計值使用；區間統計使用 Gauge 或有明確定義的衍生值。
- Node network Flux counter 保留 bounded `interface` label，不得重複聚合 bond 與
  member interface。
- Ratio 為 0 到 1、容量為 bytes、時間為 seconds/UTC ISO 8601。
- Label 不包含 owner、description、完整 URL、error text、token、未正規化或未受控
  高 cardinality 值。
- 多 Cluster 的 credential、token、cache、circuit breaker 與 labels 彼此隔離。
- 一般 4xx 不重試；authentication failure 交給 authentication manager；只對規格
  允許的 timeout、connection error、429 與部分 5xx 重試。
- Liveness 不因單一 ECS Cluster failure 失敗；readiness 以可服務 cache 為準。

## Evidence

- 檢視 task、`AGENTS.md`、`PROJECT.md`、`SPECIFICATION.md`、
  `DELL_ECS_API_MAPPING.md`、active change、受影響程式碼、設定、tests 與 deployment
  artifacts。
- 以可重現結果驗證 outcome，不依賴 agent claim 或 transcript。
- Mock ECS API 是 unit/component evidence；只有隔離的真實 ECS boundary 才能宣稱
  integration。
- 未看到成功輸出的 command 不得報告為 passed；required stage 若 skipped、
  blocked 或 unavailable，即為 handoff failure。
- Review 完成前檢查 final diff 是否引入 secret、production endpoint、response dump、
  generated output 或無關 reformat。

## Findings

每項 finding 必須包含：

- severity：`critical`、`high`、`medium` 或 `low`；
- 缺陷及影響的簡潔描述；
- 精確檔案與行號；
- 可觸發的 input、state 或 sequence；
- 不明顯時提供可執行的 correction。

優先提出少量、可證明且會影響結果的 finding。若沒有 material issue，明確說明沒有
finding，並列出未執行的 checks、環境限制與 residual risk。

## Severity Guidance

- `critical`：可洩露 credential/production data、繞過 authorization、破壞大量資料
  或造成不可逆供應鏈 compromise。
- `high`：產生錯誤 public metrics/API、跨 Cluster 資料混淆、刪除最後成功 cache、
  預設關閉 TLS 驗證，或使服務大範圍不可用。
- `medium`：特定常見情境下資料錯誤、retry storm、unbounded cardinality/memory、
  相容性回歸或缺少重要 failure test。
- `low`：影響有限但有明確 defect、診斷落差或 contract 不一致。

## Independence

Authentication、authorization、secret handling、CI write permission、release
signing、metric breaking change 與 production deployment 屬高風險變更，必須由未
實作該變更的 Project Maintainer 或 Security Reviewer 複核。Review record 必須記錄
真實 reviewer 與 evidence；角色名稱本身不代表已核准。
