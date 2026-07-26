# Dell ECS API Mapping Specification

- Version: 1.1 working contract
- Active Change: ECS-008
- Status: Runtime mappings implemented; ECS CE 3.8.0.3 and 3.8.1.4 Management partially live-verified; certification pending
- Owner: Project Maintainer

## Purpose

本文件定義 Dell ECS API 與 Exporter internal model、Prometheus metrics、Inventory
API 之間的可驗證 mapping 契約。它不以範例或推測取代 Dell 官方文件。每一個真實
adapter 必須先完成本文件對應紀錄，並以目標版本 sandbox 與去識別化 fixture 驗證。

## Current Decision

目標版本族已選定為 ECS 3.6.x、3.7.x、3.8.0.x 與 3.8.1.x。版本選擇、transport
差異與 capability 定義於 `profiles/`；共同 API 契約與逐版本差異定義於
`docs/ecs-api/`；合成、去識別化 fixtures 定義於 `testdata/ecs/`。

ECS 3.6 的 Management API 與 Flux mapping 已用 Dell 官方 REST API Reference 與
Monitoring Guide 建立文件證據。ECS 3.7、3.8.0、3.8.1 的官方 REST API ZIP 目前需要
Dell Support 登入，本儲存庫尚未完成內容比對，因此其 Management API mapping 標為
`candidate-inherited`。ECS CE 3.8.0.3 已取得部分隔離 Management API evidence，
包括非空 Bucket 的 quota/billing envelope、`{"id":[bucket names...]}` batch
contract、request-body/status matrix、known-size billing KB multiplier 與修復後
三 Bucket refresh；但缺少正式版 Flux、REST ZIP、replication 與其餘 failure
gates，因此不能宣稱整個 Profile 相容或把 `tested_builds` 填入 Profile。

ECS CE 3.8.1.4 exact build `3.8.1.4.140200.8103892f11b` 另已驗證 Profile
selection、Management login/whoami/logout、Cluster/Node、Namespace/Bucket
inventory/quota/billing、known-size KB conversion、Inventory authentication 與
Prometheus exposition。CE Flux external query 仍回 HTTP 503，單 VDC RG 也沒有
link status/RPO；因此 Node resources、Performance、Flux range boundary 與
multi-VDC replication/recovery 仍不能視為已驗證。

Mock/fixture tests 只能證明 parser、mapping 與 failure policy；不能證明真實
integration。未知版本預設拒絕，且不得把最接近的已知 Profile 當成靜默 fallback。
ECS-008 沒有變更 API URI/schema/unit mapping；它只新增受保護的 exact-build
integration/E2E gate 與 production evidence boundary。

## Preconditions for Sandbox Certification

Project Maintainer 必須針對每個宣稱支援的精確 build 取得並記錄：

1. `GET /vdc/nodes` 所回報的完整 product version/build，並確認 mixed-version 行為。
2. 對應官方 Management API／Monitoring API 文件與可稽核版本識別；3.7/3.8 REST
   ZIP 必須完成與 `docs/ecs-api/common-contract.md` 的差異比對。
3. 隔離、非 production 的 sandbox endpoint。
4. 權限最小化的 `SYSTEM_MONITOR` 測試帳號與允許的 API scope。
5. Login、token expiration、pagination、success、null/missing field、429、代表性
   4xx/5xx、timeout、Host Header 與 Flux range boundary 的去識別化 response fixtures。
6. 代表性 Cluster、Node、Namespace、Bucket、Performance、Replication 與 Recovery
   response size/duration measurements。
7. 已知大小的 object/quota/capacity，用來逐 logical API 與 unit 驗證 KB/MB/GB 到
   bytes 的 multiplier；某一 unit 的結果不得自動外推至其他 unit。
8. Project Maintainer 對 mapping correctness 的 review；涉及 credential/TLS/permission
   時另需 Security Reviewer。

上述資料不得在本文件記錄 production credential、token、Cookie、Authorization
header、私人 endpoint 或未遮罩 response body。

## Required Record for Each API

每支 API 都必須有獨立 mapping record，包含：

| Field | Required content |
|---|---|
| Mapping ID | 穩定 ID，例如 `MAP-CLUSTER-INFO-001` |
| ECS version/build | 已驗證的精確版本；不得只寫「latest」 |
| Official source | 官方文件名稱、版本與章節或受控連結 |
| Logical name | 不含敏感 URL 的穩定 logical API name |
| HTTP contract | Method、相對 URI、content type、success codes |
| Authentication | Token/login flow、renewal/reauth behavior、權限需求 |
| Request parameters | 型別、required/optional、allowed range、encoding |
| Pagination | Cursor/page semantics、page size、termination 與 duplicate handling |
| Response schema | 必要/optional/null 欄位、型別、unit、timestamp 與 enum |
| Internal model | 目標 struct/field、normalization、unit conversion、validation |
| Prometheus mapping | Metric name、type、unit、labels、scope、missing semantics |
| Inventory mapping | Endpoint 與 JSON field、null semantics、redaction |
| Polling/cache | Interval、timeout、bounded concurrency、atomic replacement、TTL |
| Error handling | Retryable/non-retryable statuses、reauth、partial failure、fallback |
| Evidence | Redacted fixture、contract/component test、sandbox integration result |
| Known limitations | Version差異、資料 scope、cardinality 與不支援欄位 |
| Review status | Reviewer role、真實 reviewer identity/date，或明確未核准 |

## Logical Coverage Backlog

下表只確立需要 mapping 的資料領域，不代表 ECS 已提供對應 endpoint。

| Mapping family | Required domain | Exporter consumer | Default poll | Current evidence |
|---|---|---|---:|---|
| MAP-AUTH | Login、token lifecycle、logout/expiry | Authentication Manager | 依 token lifecycle | `MAP-AUTH-001`；3.6 documented，後續版本 candidate-inherited |
| MAP-CLUSTER-INFO | VDC/cluster identity、health、version、resource counts | Cluster Collector、`/api/v1/clusters` | 60s | `MAP-CLUSTER-HEALTH-001`、`MAP-VERSION-001` |
| MAP-CLUSTER-CAPACITY | total/used/available bytes | Cluster Collector、cluster metrics | 60s | `MAP-CLUSTER-CAPACITY-001`；ECS CE 3.8.0 top-level envelope observed；known-size unit gate pending |
| MAP-NODE-INFO | node identity、health、state、CPU、memory、disk、service | Node Management/Resources Collectors、`/api/v1/nodes` | 60s | ECS CE 3.8.0 Management observed；Flux resource mapping production-pending |
| MAP-NODE-NETWORK | receive/transmit values 與其累計語意 | Node Collector、node network metrics | 60s | Flux `net.bytes_recv/bytes_sent` 為 cumulative；保留 `interface` |
| MAP-NAMESPACE | owner、capacity、quota、bucket/object count、replication、audit | Namespace Collector、`/api/v1/namespaces` | 300s | ECS CE 3.8.0 empty and four-object Namespace billing observed；billing KB known-size assertion passed |
| MAP-BUCKET-INFO | identity、owner、capacity、objects、features、timestamps | Bucket Collector、`/api/v1/buckets` | 300s | ECS CE 3.8.0 namespace-scoped empty and corrected three-item Exporter refresh passed |
| MAP-BUCKET-QUOTA | soft/hard quota 與未設定語意 | Bucket Collector、bucket quota metrics | 300s | `-1` unset documented；ECS CE 3.8.0 top-level configured 1/2 GB observed，GB multiplier still pending |
| MAP-PERFORMANCE | throughput、latency、operation/status statistics 與 scope | Performance Collector、performance metrics | 60s | VDC/Namespace Flux documented；Bucket scope unsupported |
| MAP-REPLICATION | group、source/target VDC、status、lag | Replication Collector、`/api/v1/replications` | 120s | Dashboard RG/RG link documented；lag derived |
| MAP-RECOVERY | status、progress | Recovery Collector、replication/recovery inventory | 120s | conditional；必須保留 failover/bootstrap/recovery kind |

## Mapping Invariants

- ECS 沒有提供的欄位在 Inventory API 回傳 `null` 或省略規格允許的 metric，不得
  推導或填入看似合理的值。
- 來源 unit 必須在 boundary 明確轉為 bytes、seconds、ratio 或 count。
- Percent 必須轉成 0 到 1 ratio；若來源可能同時回傳 0–1 或 0–100，mapping 必須
  以 schema/version 識別，不得用數值猜測。
- 只有單調累計來源可對應 `_total` Counter；windowed count、instantaneous rate 或
  snapshot value 必須使用符合語意的 Gauge/derived metric。
- 來源 scope 是 Cluster、Namespace 或 Node 時，不得展開或重複成 Bucket scope。
- Pagination 必須定義排序穩定性、終止條件、重複/遺漏處理與完整成功判定。
- 任何 page/resource 失敗時不得用部分清單覆蓋最後完整成功 snapshot。
- Metric labels 必須是 bounded、normalized、長度受限的穩定維度；owner、description、
  URL、error text、token 與其他高 cardinality/敏感值禁止使用。
- Label 清理或截斷時必須加入穩定雜湊後綴，避免不同原值正規化成同一 series。
- API logical name 可進入 self-monitoring label；實際 URL 不可進入。

## Fixture and Test Rules

- 文件階段的共用合成 fixture 存放於 `testdata/ecs/`；實作後 package-specific
  fixture 可存放於 adapter package 的 `testdata/`，但必須引用相同 mapping ID。
- 每個 mapping 至少需有 success、missing optional、null、invalid type/unit、empty
  page、last page、authentication expiry 與 relevant error fixtures。
- Secret scanner 必須檢查 fixture 與 test evidence。
- Contract tests 驗證 schema、unit、enum、pagination、metric type 與 Inventory null
  semantics。
- Sandbox integration 必須記錄 ECS version、logical API name、result、duration、
  response size 與 fixture hash；不得保留敏感原始 response。

## Compatibility

不同 ECS versions 以 `profiles/ecs-*.json` 的 adapter capability 隔離。Exporter
啟動時必須輸出已支援及偵測版本；未知或未支援版本預設拒絕啟動該 Cluster 的
collector，並提供不洩露敏感資訊的 actionable error。若採 capability negotiation，
行為與 fallback 必須先在本文件被核准，不得靜默選用相近版本。

滾動升級時如果 `/vdc/nodes` 回報多個 Profile，僅能啟用 capability 交集，並停用
interval-derived Flux rates。所有節點進入同一 Profile 且 probe 成功後，adapter 才能
原子切換。

ECS 3.7 與 3.8.0 依 Dell KB 000211906 停用 Flux interval rate；只允許 `last()`
snapshot 與來源本身的 reset-aware cumulative counter。ECS 3.8.1 雖由 Dell 標示已
修正，仍須先通過 range-boundary sandbox gate 才可啟用 interval rate。

ECS 3.8.0 經 proxy/load balancer 時必須符合 accepted server names；3.8.1 必須兼容
從 3.8.0 upgrade 後保留的設定。Transport 不得以改用 IP 規避 Host Header 檢查。

## Runtime Implementation (ECS-004 / ECS-005 / ECS-006 / ECS-007)

- `internal/ecs/client.go` 實作 per-cluster TLS/auth、first-401 reauthentication、
  bounded retry/concurrency、circuit、response bound、safe logical errors 與
  cross-origin redirect rejection。
- `internal/ecs/parser.go` 實作本文件 V1 Management/Flux mapping、billing KB 的
  1024 multiplier、既有 decimal MB/GB/TB 與 capacity/quota GB、0–1 ratio、
  known enum、half-open Flux range、finite overflow guard 與 null/unset semantics。
- `internal/collector/runner.go` 實作 Cluster/Node/Namespace/Bucket/Performance/
  Replication/Recovery refresh。Bucket `marker`/`next_marker` 只有所有 pages 與
  quota/billing enrichment 完成後才替換 cache。
- `internal/profile` 的 capability resolution 決定 conditional/unavailable query；
  mixed-version 永遠停用 interval-derived rates。
- Replication API 沒有已驗證的安全 list endpoint，因此 V1 只查詢設定中明列的
  group/link IDs。此限制避免發明 URI，也保留真機補證後擴充空間。
- Recovery metric 保留 replication group、source/target VDC 與 operation kind，
  避免同一 group 有多個 link 時產生重複 Prometheus series。
- 以上 implementation 已由 synthetic fixtures/mock transport 驗證，但沒有提升
  `candidate-inherited` 或 sandbox evidence status。

ECS-005 另以隔離 ECS CE 3.8.0.3 觀察到並修正：

- Capacity 與 Namespace quota 的 3.8.0 top-level envelope，同時保留舊式 nested
  envelope，雙 envelope 視為 ambiguous error。
- Node health HAL `_embedded._instances`。
- Bucket list 必須先列 Namespace，再以 `namespace` scope 執行分頁。
- Node Management 與 Flux Resources 分離；Flux 503 保留可觀察 error 並使 readiness
  `DEGRADED`，不丟棄 Node inventory/health 或捏造 resource metrics。

去識別化結果位於
`docs/ecs-api/validation/ecs-ce-3.8.0.3-2026-07-25.md`。這是部分 CE evidence，
沒有解除 REST ZIP、正式版 Flux、完整 failure、修復後 rerun 或 reviewer gates。

ECS-006 補完 fixture/component 可證實的 runtime mapping：

- Performance Flux response 會嚴格解析 measurement/field/unit/scope/time，原子替換
  VDC/Namespace cache，並輸出 throughput、latency、request/status-window Gauge。
- Profile `conditional` capability 必須由每個 Cluster 的
  `capabilities.enabledConditional` 明列；`unavailable` 與 mixed-version interval
  rates 不可覆寫。
- Node disk 只聚合 filesystem allowlist；network 受 allowlist、最多 interface 數與
  bond preference 限制。原始 cumulative bytes 保持 Counter，下降交由 Prometheus
  counter-reset semantics 處理，不合成 offset。
- Node health response 若含 `services[]`/`processes[]`，可在 conditional capability
  啟用時映射 `ecs_node_service_health`；不新增未證實 URI。
- Namespace/Bucket owner、audit、retention、created/last-modified 與 billing sample
  time 分開映射。ECS-006 原先將 B/KB/MB/GB/TB 視為 decimal；ECS-007 依 known-size
  live assertion 將 billing KB 修正為 1024，其他單位維持既有 decimal mapping。
- Bucket billing 先以 Namespace batch POST 取得；404/405/501 或 missing item 才
  fallback 單筆 GET，完整成功後才替換 cache。
- 每 Cluster request token bucket、initial schedule jitter、API response-size
  histogram、per-domain cache generation 與 `maxStale` domain cutoff 已實作。

以上仍是 synthetic fixture/component evidence；四個 Profile 的 `tested_builds=[]`
與 `sandbox_certified=false` 維持不變。

ECS-007 依同一隔離 ECS CE 3.8.0.3 的非空 Bucket follow-up 修正：

- Bucket quota 與單筆 Bucket billing 接受 inherited nested 或 3.8.0 top-level
  envelope；同時出現兩種 envelope 時拒絕。
- Namespace batch Bucket billing 對 no entity、`{}`、`[]`、`null` 分別回
  415、200 + plural `bucket_billing_infos: []`、400/code1013、500/code999。
  這證明 500/code999 是 `null` request body 的結果，不足以判定 endpoint
  unsupported 或定義 fallback。
- 內建 `BucketListParam.class` 與 non-empty live probe 確認 request 是
  `{"id":[bucket names...]}`；三 Bucket request 回 HTTP 200 與三筆 plural
  `bucket_billing_infos`。
- Batch fallback 僅適用 404/405/501 或缺少 requested item；generic 500 與
  500/code999 不 fallback。
- Billing `KB` 乘 1024。三個 Bucket 的 `6835.9375 + 2929.6875 + 0 KB`
  與 Namespace `9765.625 KB` 都對應四個物件的已知 10,000,000 bytes。
- `testdata/ecs/ecs-3.8.0.3-live/` 保留去識別化的 partial live fixtures；
  它們不代表整個 Profile、正式設備或修正後 Exporter 已完成認證。

修正後 Exporter live rerun 的 batch HTTP 200、Bucket collector 連續成功、
Inventory 與 aggregate metrics 已記錄。此結果仍只屬 exact CE build partial-live
evidence；Profile 的 `tested_builds=[]`、`sandbox_certified=false` 與正式認證缺口
維持不變。

ECS CE 3.8.1.4 partial-live follow-up 另證實：

- `/vdc/nodes` exact build 正確選擇 `ecs-3.8.1`，`mixedVersion=false`。
- Namespace/Bucket quota 仍使用 top-level fields；configured Bucket 是 1/2 GB，
  unset Namespace/Bucket 仍使用 `-1` sentinel。
- `{"id":[three Bucket names]}` batch 回 plural `bucket_billing_infos`，0/3072/7168 KB
  與 0/1/3 objects；Namespace `10240 KB`/4 objects 精確映射為 10,485,760 bytes。
- 單 VDC RG 可有 pending bytes 0，但沒有 `rglinkStatus` 或 RPO；parser 保留
  status/lag 未設定，不能捏造 metric。
- Exporter Management-backed collectors 成功且 29 個 metric families 通過檢查；
  唯一 readiness 降級來源是 Flux-backed `node-resources` HTTP 503。

去識別化 record 與 fixtures 位於
`docs/ecs-api/validation/ecs-ce-3.8.1.4-2026-07-26.md` 與
`testdata/ecs/ecs-3.8.1.4-live/`。這些資料沒有修改 API URI/unit mapping，也不解除
REST ZIP、production Flux、正式設備、failure injection、target-scale 或 reviewer
gates；`tested_builds=[]` 與 `sandbox_certified=false` 維持不變。

新增、刪除或修改任何 mapping 時，必須同步更新 `SPECIFICATION.md`、
`SPEC_CHANGELOG.md`、`TEST_PLAN.md`、`TRACEABILITY.md` 及對應 tests。
