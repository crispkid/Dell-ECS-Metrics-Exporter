# Test Plan

Active Change: ECS-008

## ECS-008 Production Release and Bare Metal Verification

Deterministic/static cases:

- Reproducible archive ordering, timestamp, ownership, mode and symlink rejection.
- Semantic release tag、clean worktree、full commit、checksums、metadata 與 required
  release assets。
- Docker build FROM digest、all workflow action full-SHA、least-privilege job permissions、
  protected release/exact-build jobs。
- Helm default/full/production render、explicit ingress selector、DNS/ECS egress、
  immutable digest placeholder、Secret non-generation 與 kubeconform schema。
- Bare Metal Bash syntax、executable mode、systemd unit verification（Linux 可用時）、
  dedicated account/path/mode、config/Profile preflight、safe upgrade/uninstall contract。
- Prometheus alert rules syntax（promtool 可用時）與 runbook/checklist link。
- Synthetic 10 Cluster/100 Node/10,000 Bucket metrics/Inventory p95 and heap precheck。

Required external release evidence:

- Docker non-root/read-only startup and health/metrics smoke。
- Source/image High/Critical vulnerability scan、SPDX/CycloneDX SBOM、checksums、
  keyless signature and provenance verification。
- Deployed target-scale 10/100/10,000 p95、RSS、CPU、response-size measurement。
- Exact ECS 3.8.1.4 protected read-only live integration with `UP` readiness。
- Exact ECS CE 3.8.0.3 protected Management compatibility rerun；DEGRADED 僅允許
  `node-resources` positive collector error。
- Deployed HTTPS E2E including unauthenticated rejection、five Inventory collections、
  exact ECS version and conditionally required metric families。
- Live Kubernetes admission/apply/rollout/rollback and selected Bare Metal verification。

`./scripts/release-check.sh` 是 fail-closed production entry point。Exit 3 代表 prerequisite
缺少並阻擋發布。Mock、fixture、CE partial-live 或 synthetic result 不得替代上述
external evidence。

2026-07-26 local execution results:

- Unit/component 與 fresh race suites 通過；application coverage 84.3%。
- Build/Profile、governance、CI policy、Helm/Bare Metal static checks 通過。
- 兩次 release build 的 SHA256 manifest 完全相同，所有 archive/Helm package
  checksum 驗證通過，manifest 未包含暫存檔。
- Synthetic 10 Cluster/100 Node/10,000 Bucket、16 concurrent scrapes、
  metrics/Inventory p95 與 live-cache heap precheck 通過。
- Full Harness 在 `govulncheck` 連線 `vuln.go.dev` 失敗；release policy、scanner、
  Docker daemon、kubeconform、exact-build credential、deployed E2E/performance
  prerequisite 缺少時均以 exit 3 阻擋。這些外部 gate 尚未通過。

## ECS-007 ECS 3.8.0.3 Bucket Compatibility Regression

ECS-007 的 deterministic cases 必須涵蓋：

- Bucket quota inherited nested 與 ECS 3.8.0 top-level success；missing envelope、
  invalid quota 與 dual-envelope ambiguity rejection。
- Single-bucket billing inherited nested 與 top-level success；missing/dual-envelope
  rejection。
- ECS error `code` 為 JSON number 或 numeric string 時只保留 normalized integer；
  malformed/missing code 不得被誤判為 999，raw response 不進入 error/log。
- Batch billing request 必須使用 JSON object，而非 no entity、array 或 `null`；
  regression 覆蓋 415、200/plural empty envelope、400/code1013、500/code999 的
  observed matrix。
- Non-empty request 使用 `{"id":[bucket names...]}`，解析三筆 plural
  `bucket_billing_infos` 並以 Namespace/name 對應。
- 404/405/501 與 missing requested item fallback；generic 500、500/code999 與
  non-missing malformed batch 不 fallback。
- Missing batch item只對缺項單筆查詢；任一必要單筆失敗仍保留上一份完整 snapshot。
- Known-size unit assertion：
  `6835.9375 + 2929.6875 + 0 = 9765.625 KB`，
  `9765.625 * 1024 = 10,000,000 bytes`。
- Billing MB/GB/TB 與 capacity/quota GB 的既有 decimal test 保持不變，防止把
  billing KB evidence 誤泛化。
- `testdata/ecs/ecs-3.8.0.3-live/` 的去識別化 fixture、manifest classification、
  mapping reference 與 credential scan。

Required code handoff 為 `go test -race -timeout=3m ./...` 與完整
`./HARNESS/harness.sh verify`。Race 已通過；Harness 的 deterministic stages 與
獨立 deployment check 已通過，但 required `govulncheck` 外部漏洞資料庫連線未獲
執行政策授權，因此完整 `verify` 尚未通過。

Corrected live rerun 已通過：

- Batch billing HTTP 200；Bucket collector 至少連續成功兩次。
- Inventory HTTP 200，三個 Bucket 分別為 0 bytes/0 objects、
  7,000,000 bytes/3 objects + soft 1e9/hard 2e9 quota、
  3,000,000 bytes/1 object 且 quota 未設定。
- Cluster aggregate 為三 Bucket／一 Namespace／四 objects。
- Namespace 為 10,000,000 used bytes／四 objects／三 Buckets。
- Health HTTP 200 `DEGRADED`；唯一 collector error 是已知 CE Flux
  `node-resources` HTTP 503。
- Version 使用 `ecs-3.8.0`，`sandboxCertifiedProfiles` 維持空集合。

這是 exact CE build partial-live evidence，不取代未完成的 vulnerability gate 或
Profile certification。

## ECS-006 Ten-gap Component Verification

ECS-006 的 deterministic cases 必須涵蓋：

- VDC/Namespace throughput、latency、request/status-window Flux mapping、unit/scope、
  duplicate/invalid response 與 Prometheus Gauge exposition。
- Profile native/conditional/unavailable 與 explicit enable/deny；Recovery 同樣受 guard，
  skipped collector 不增加 cache refresh/error。
- Node service/process known enum、disk filesystem allowlist、network interface
  allowlist/max/bond preference，以及 raw Counter decrease/reset acceptance。
- Namespace owner/audit、Bucket retention/last-modified 與 billing sample time 分離。
- Namespace batch Bucket billing success、unsupported fallback、missing item fallback 與
  atomic failure retention。
- per-cluster rate limiter cancellation、login/logout/request 共用 limiter、首次 schedule
  jitter，以及 API response-size success/error observation。
- `maxStale` 之後停止 domain series，但保留 self metrics。
- future sandbox certification profile 只有在 tested build、sandbox evidence
  classification/status 與 reviewed API reference 全部存在時才可載入。

Required handoff 仍為九個 Harness stages，加上 `go test -race -timeout=3m ./...`。
這些測試是 fixture/component/static evidence；ECS 3.8.0.3 CE Flux、正式設備
3.8.1.4、10,000 Buckets、container startup 與 live Kubernetes 仍是外部 gate。

ECS-006 最終結果：完整 race test 通過；`./HARNESS/harness.sh verify` 九個 required
stages 全數通過，總 coverage 84.4%；`govulncheck` 無已知漏洞；Profile build、
Helm deployment static check 與 CI policy check 通過。Fixture repository 現有
32 個 JSON、5 份 manifest、26 筆 fixture records 與 6 個 Flux fixtures。

## ECS-005 Partial ECS CE 3.8.0.3 Integration

The isolated live run used an exact `3.8.0.3.138685.3a0a9b6bf3a` build and a dedicated
`SYSTEM_MONITOR` account. It verified login/whoami/logout, version/Profile selection,
Host accepted/rejected behavior, Cluster/Node Management, empty Namespace quota/billing,
namespace-scoped empty Bucket listing, Inventory authentication/envelopes and Prometheus
output. ECS-007 later added partial live evidence for three non-empty/empty Bucket scenarios,
top-level quota/single billing, the batch request-body/status matrix and a known-size billing
KB assertion.

Observed live response regressions cover top-level capacity/quota envelopes, HAL
`_embedded._instances` Node health, required Bucket Namespace scoping and independent
Node Management/Flux failure behavior. Regression and race tests cover all four changes.

The CE Flux endpoint returned HTTP 503/code 6503 and the environment had no separate
Flux/Influx backend. Readiness therefore correctly returns HTTP 200 `DEGRADED` while
Management inventory remains available. This does not count as production Flux evidence.

The redacted result is
`docs/ecs-api/validation/ecs-ce-3.8.0.3-2026-07-25.md`. `tested_builds` and
`sandbox_certified` remain unchanged because REST ZIP comparison, formal-appliance Flux,
replication, remaining unit and failure-injection gates are incomplete.

## ECS-004 Implemented Runtime Gates

本 change 的 deterministic evidence：

- `internal/config`：strict YAML、environment expansion、secret file、TLS、reserved
  path、duration/concurrency、duplicate cluster/replication 與 production safety。
- `internal/ecs`：login/header/cookie token、首次 401 reauthentication、503 retry、
  timeout/error classification、cross-origin redirect、logout、circuit、response bound、
  parser/unit/enum/overflow/Flux half-open range。
- `internal/collector`、`internal/cache`：全 domain collection、Profile capability、
  marker pagination、duplicate/loop/page failure、concurrent enrichment、last-success
  retention、deep-copy snapshot、single-flight/scheduler。
- `internal/metrics`：Prometheus parser 接受 exposition、domain/self families、null
  omission、bounded/control-free label 與 cache state。
- `internal/httpapi`、`internal/health`：liveness/readiness、UP/DEGRADED/DOWN、Bearer/
  proxy auth、exact filter、sort/page limit、single resource、method/error contract。
- Docker/Helm/CI：binary build、Helm lint/template、ConfigMap/existing Secret mounts、
  probes/security/resources/NetworkPolicy/PDB/ServiceMonitor、read-only permissions 與
  action full-SHA pinning。

`go test -race`、80% coverage、`govulncheck` 與所有 Harness required stages 是
ECS-004 handoff gate。真實 ECS integration、performance、container startup、live
Kubernetes、image scan 與 release supply-chain 仍依下列計畫外部執行。

## ECS-003 Implemented Bootstrap Gates

本 change 的實際測試範圍為：

- Dell 四段版本合法/不合法/overflow/suffix 與 comparison。
- 四個 Profile strict JSON decode、range boundary、重疊、未知欄位、evidence 與
  capability validation。
- 四版 `nodes.json` 的 uniform selection、3.7/3.8.0 mixed intersection，以及
  3.8.2 unknown-version rejection。
- 五份 manifest、24 個 fixture record、mapping ID、JSON、credential pattern、
  Flux column/type/row 與 range boundary contract。
- `/health`、`/api/v1/health` 的 503/`DOWN` readiness、`/api/v1/version`、
  bootstrap `/metrics` method、headers、escaping 與不得宣稱 sandbox certification。
- Helm lint/template 與 required Kubernetes resource/security/Secret-reference
  contract。

這些都是 unit/component/static deployment evidence。沒有執行真實 ECS、container
startup、Kubernetes live apply、performance 或 E2E。

## ECS-002 Version Profile and V1.0 Test Design

### Scope and Evidence Classification

本計畫涵蓋 REQ-001 至 REQ-012。測試 evidence 分成：

- Unit：純 parser、validator、unit conversion、mapping、state transition 與 error
  classification。
- Component：以合成 Mock ECS API 驗證 HTTP/auth/pagination/cache/API 行為；不能宣稱
  是真實 Dell ECS integration。
- Integration：使用已選定版本、隔離且非 production 的 Dell ECS sandbox。
- E2E：已部署 Exporter + sandbox ECS + Prometheus scraper + authenticated Inventory
  client。
- Performance：代表性 10 Clusters、100 Nodes、10,000 Buckets、15 秒 scrape 與故障
  注入。
- Security/deployment：secret/TLS/authz、dependency/image/policy scans 及 Kubernetes/
  Helm validation。

除 ECS-005 的 ECS CE 3.8.0.3 部分 Management API evidence 外，其餘版本與正式版
Flux、failure、performance tests 仍只有 specification、Profile、mapping、
synthetic fixtures 與 Harness；fixture 可解析不代表真實 ECS integration 結果。

### Requirement Coverage

| Requirement | Required test focus |
|---|---|
| REQ-001 | 多 Cluster credential/token/cache 隔離、login/renew/expiry/reauth、TLS CA、重複 Cluster name |
| REQ-002 | schedule/jitter/single-flight、atomic replacement、concurrent read、cache bootstrap/stale/last-success |
| REQ-003 | health/state enum、capacity bytes、CPU ratio、missing hardware fields、network Counter eligibility |
| REQ-004 | namespace/bucket mapping、soft/hard quota set/unset、object count、null/missing optional fields |
| REQ-005 | scope preservation、throughput/latency unit、windowed count Gauge、monotonic Counter reset |
| REQ-006 | replication/recovery states、lag seconds、progress 0–1、unsupported capability |
| REQ-007 | pagination/filter/sort、max size、authentication/authorization、RFC 9457、no query-triggered ECS call |
| REQ-008 | UP/DEGRADED/DOWN、liveness/readiness、self metrics、cache age、build info |
| REQ-009 | connect/read/overall timeout、429/5xx retry、4xx no-retry、backoff/jitter、circuit isolation、partial pages |
| REQ-010 | YAML/env/secret validation、duplicate/invalid interval、TLS default、log/metric/artifact redaction |
| REQ-011 | `/metrics`/Inventory latency、memory/CPU、scale、Docker startup、Helm/Kubernetes security/probes |
| REQ-012 | 四段版本解析、Profile selection、mixed/unknown rejection、capability、3.7/3.8.0 Flux guard、3.8 Host Header、mapping traceability |

### Boundary and Failure Cases

- Empty Cluster list、1/10/超過設計限制的 Cluster、duplicate names、相同 endpoint
  不同 credentials。
- Empty/one/exact-limit/over-limit pagination、cursor loop、duplicate page、out-of-order
  page、mid-stream timeout 與 malformed item。
- Missing、null、wrong type、negative、overflow、NaN/Inf、0、最大合理 bytes/count、
  percent 0/100 與未知 enum。
- Quota 未設定、只有 soft、只有 hard、兩者都有、soft 大於 hard、used 大於 quota。
- Token 已過期、首次 401、reauth 後仍 401、429 with/without retry hint、retryable 5xx、
  non-retryable 4xx、TLS/CA failure。
- Collector 超過 interval、同一 collector overlapping、另一 collector/Cluster 正常、
  cache 尚未初始化與超過 stale tolerance。
- Inventory 無 token、錯誤 token、合法 token、oversized query、invalid sort/filter、
  injection-like input 與高並行 read。
- Secret 出現在 request header、error、response body 與 config 時的全路徑 redaction。
- `3.6.2.6`、`3.7.0.7`、`3.8.0.6`、`3.8.1.7` 與帶 build suffix 的版本解析。
- 同一 Cluster 節點跨 3.6/3.7、3.7/3.8.0、3.8.0/3.8.1 的 mixed-version。
- 3.5、3.8.2、3.9、缺段、負數、overflow、空白與非數字版本必須拒絕。
- Flux row 欄位順序不同、row length 不符、out-of-window、stale、counter reset。
- 3.8.0 direct/proxy/load balancer 的 accepted/rejected Host；hostname/SNI/Host 不一致。

### Metrics Validation

- 使用 Prometheus parser 驗證 text/openmetrics exposition。
- 對每個 metric 驗證 name、HELP/TYPE、base unit、labels、scope、duplicate series 與
  cardinality bound。
- 確認 `/metrics` handler 的 ECS client call count 維持 0。
- 確認 cache 無 domain data 時仍輸出 self-monitoring metrics。
- 確認未設定 quota 的 series 完全不存在，不輸出 0、cluster capacity 或 sentinel。
- 確認 `_total` 只對已由 API mapping 證明為單調累計的來源存在。
- Node network counter 必須保留 bounded `interface` label；不得重複聚合 bond 與
  member interface。
- ECS 3.7/3.8.0 不得輸出任何由 Flux window 計算的 interval-derived rate。
- 四個 Profile 均不得輸出 Bucket scope throughput/latency/request/status metrics。

### API Validation

- 依 `SPECIFICATION.md` 第 11 節驗證 collection envelope、page/size/totals、
  `collectedAt` 與 Bucket/Namespace minimum model。
- API 缺少來源欄位時回傳 `null`，不得填推測值。
- 錯誤使用 `application/problem+json` 並符合 RFC 9457 所需欄位；detail 不洩露
  credential、stack trace、private URL 或 raw ECS body。
- Inventory query 只讀本機 snapshot，所有 filter/sort/page 均不得直接產生 ECS API
  request。

### Integration and Performance Environment

真實 integration 前提是 `DELL_ECS_API_MAPPING.md` 的 Preconditions 全部滿足。測試
只使用隔離帳號與合成/可清理資源，記錄精確 ECS version/build；禁止使用 production
credential 或 production data。

### Version Certification Matrix

| Profile | Primary build candidate | Required special tests | Current evidence |
|---|---|---|---|
| ECS 3.6 | `3.6.2.6` | Dashboard removed fields、Flux replacement、unit assertion | document-derived fixtures only |
| ECS 3.7 | `3.7.0.7` | Flux range defect guard、`last()` freshness | document-derived fixtures only |
| ECS 3.8.0 | `3.8.0.6` | Flux guard、accepted server names、proxy/LB 403 | ECS CE `3.8.0.3` Management partial；正式版 primary/Flux/failure gates pending |
| ECS 3.8.1 | `3.8.1.7` | Flux range-boundary re-enable gate、persisted Host config | document-derived fixtures only |

宣稱整個 `x` range 前，除 primary candidate 外還需測試該 Profile 的最低支援 build
或取得能證明 schema 相容的正式 upgrade/certification evidence。

### Fixture Contract Tests

- 驗證 `profiles/profile.schema.json` 與四個 Profile。
- 驗證 `testdata/ecs/manifest.schema.json`、manifest reference 與所有 JSON 可解析。
- 每個 manifest 所列 fixture 必須存在且 mapping ID 可在 `docs/ecs-api/` 找到。
- fixture secret scan 禁止 token、Authorization/Cookie、private endpoint、`secretKeys`。
- 3.7/3.8.0 `flux-range-out-of-window.json` 必須拒絕 rate calculation。
- 3.8.1 `flux-interval-valid.json` 即使通過 component test，仍不得自動把 build 加到
  `tested_builds`。

Performance scenarios：

1. 10 Clusters 同時依預設 intervals polling。
2. 至少 100 Nodes 與 10,000 Buckets 的完整 inventory/cache。
3. Prometheus 每 15 秒 scrape，Bucket collector 每 300 秒 refresh。
4. ECS API 高 latency、429、部分 5xx、單 Cluster authentication failure 與 page
   failure。
5. 測量 `/metrics` response size/generation、Inventory P50/P95/P99、heap peak、CPU、
   goroutine count、API concurrency 與 cache refresh duration。

成功標準依 `SPECIFICATION.md` 第 20、21 節；估計值不能取代測量。若 10,000 Bucket
情境無法在 512 MB/1 Core 目標內運作，需記錄容量模型、調整方案與規格 change。

### Security and Deployment Tests

- Config/fixture/log/evidence secret scan。
- Go dependency vulnerability、license 與 checksum integrity scan。
- Container 以 non-root、read-only filesystem（可行時）、固定 base image digest 與
  無內嵌 secret 啟動。
- Helm lint/template、Kubernetes schema、resource requests/limits、liveness/readiness、
  Secret reference、NetworkPolicy 與 ServiceMonitor。
- Inventory Bearer/reverse-proxy auth positive/negative tests，並確認 `/metrics`
  protection 依部署設定一致。
- Release 產生 checksums、SBOM、provenance；sign/verify policy 在首次 release 前固定。

### Coverage and Commands

- Coverage threshold: Go application packages 最低 80%；核心 auth/parser/cache/mapping/
  pagination/error classification 要有直接 branch/failure coverage。
- Current implementation commands:
  - `./HARNESS/harness.sh selftest`
  - `./HARNESS/harness.sh doctor`
  - `./HARNESS/harness.sh governance:doctor`
  - `./HARNESS/harness.sh <lint|format:check|typecheck|test|coverage|build|security|deploy:check|ci:policy>`
  - `./HARNESS/harness.sh verify`
- `integration`、`e2e`、`supply-chain` 不屬於日常 deterministic required stages，
  但已配置為 `release-check.sh` 的 production blocking gates；只有取得對應外部
  環境與通過 evidence 後才能宣稱成功。

### Acceptance Rule

每個 requirement 必須在 `TRACEABILITY.md` 連到實際 implementation、tests、Harness
command 與 evidence。Required stage skipped、blocked 或 unavailable 視為 handoff
failure。Gate 4 只有在所有 V1.0 acceptance criteria、真實 integration prerequisites
與 release-required evidence 完成後才能核准。
