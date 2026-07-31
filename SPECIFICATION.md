# Dell ECS Metrics Exporter
## Product & Integration Specification

- **Version:** 1.0
- **Status:** v1.0.0-rc.1 Release Candidate；stable Gate 4 尚未核准
- **Active Change:** ECS-012
- **Specification Date:** 2026-08-01
- **Scope:** Dell ECS API Exporter、Prometheus Metrics、Inventory API 與整合規格
- **Implementation Baseline:** Go language 1.26.0；toolchain 1.26.5
- **License:** Apache License 2.0

### 文件控制與未決條件

本文件已將使用者提供的 V1.0 附件納入儲存庫，作為產品與整合規格的 working
baseline。使用者已於 2026-07-25 確認採用 Go、GitHub Actions、Docker 與 Helm 的
建議方向；此確認不等同於對尚未檢視之完整實作計畫或 production release 的核准。

目標版本族已選定為 ECS 3.6.x、3.7.x、3.8.0.x 與 3.8.1.x。Profile 位於
`profiles/`，版本化 API mapping 位於 `docs/ecs-api/`，合成 fixtures 位於
`testdata/ecs/`。ECS 3.6 已建立官方文件 mapping；後續版本仍有需要 Dell Support
登入的 REST API ZIP 與真機 schema 待驗證。文件衍生 fixture 不等同 sandbox
integration evidence，所有 Profile 的 `tested_builds` 在真機通過前保持空白。

ECS-004 已完成 multi-cluster config、ECS token/TLS client、版本 bootstrap、各領域
collector/scheduler、原子快取、Prometheus mapping、Inventory/auth/health API、
結構化紀錄與 Docker/Helm/CI 實作，並以 synthetic fixture 與 mock transport 完成
component verification。`/api/v1/health` 現在依 cache freshness 與 collector 狀態
計算；這些 mock evidence 仍不改變任何 Profile 的真機認證狀態。

ECS-005 以隔離 ECS CE `3.8.0.3.138685.3a0a9b6bf3a` 完成部分 Management API
integration，修正 top-level capacity/quota、HAL Node health 與 Namespace-scoped
Bucket listing，並把 Node Management 與 Flux Resources 分成獨立 collector。CE Flux
持續回 HTTP 503，因此此環境的正確狀態是 HTTP 200 `DEGRADED`、保留 Node
inventory/health 且省略未取得的 CPU/Memory/Network。這不等同正式設備或整個
`ecs-3.8.0` Profile 認證，`tested_builds` 仍為空。

ECS-006 完成十項可由 fixture/component 證實的缺口：VDC/Namespace Performance
解析與公開 Gauge、conditional capability 顯式啟用、Node disk/service/process、
Namespace/Bucket inventory 欄位、`maxStale` domain cutoff、per-cluster API rate
limit 與首次排程 jitter、bounded network policy、Namespace batch Bucket billing
及 fallback、TB/response-size telemetry，以及 per-domain cache refresh 計數。
這些結果不新增任何 `tested_builds`，也不解除 ECS 3.8.0.3 CE Flux 與正式設備
3.8.1.4 的 live compatibility gate。

ECS-007 依同一 ECS CE 3.8.0.3 的非空 Bucket follow-up 修正 quota/billing
envelope 與 billing KB conversion，並重新確認 batch billing request contract。
三個 Bucket 共含四個物件及 10,000,000 bytes；ECS 回報 Namespace billing
`9765.625 KB`，證明 billing `KB = 1024 bytes`。Batch API 對 no entity、`{}`、
`[]`、`null` 分別回 415、200/plural empty envelope、400/code1013、500/code999；
內建 `BucketListParam.class` 與 non-empty probe 進一步確認 body 是
`{"id":[bucket names...]}`，並回 plural `bucket_billing_infos` 三筆。因此 code999
不能視為 endpoint unsupported，也不得 fallback。此證據不能外推至其他 unit，
修正後 Exporter live rerun 已成功取得三個 Bucket 與正確 aggregate。這仍只是
exact CE build 的 partial-live evidence；Profile 認證狀態維持不變。

ECS-008 建立 production-ready delivery contract：新增 hardened Linux
Bare Metal/systemd 部署、受限 Helm ingress/egress NetworkPolicy、確定性 release
archives、checksums、SBOM、Critical/High vulnerability gate、keyless Sigstore
簽章、GitHub provenance/attestation、container/Kubernetes schema gate、target-scale
效能量測、deployed E2E 與 protected exact-build live validation。這些機制已實作，
但缺少的真實 ECS、Docker/Kubernetes、外部 scanner、Git remote 或 reviewer evidence
仍必須 fail closed；不能由 synthetic result 或 workflow 定義本身推導 production
approval。

ECS-009 依 Dell 官方 Monitoring Guide 與授權實體 ECS 3.8.1.1 唯讀觀察修正
Flux 契約：Node CPU、Memory、Network 與 conditional Disk 改為獨立 `keep()` /
`last()` 查詢後原子合併；VDC performance 只接受精確 measurement/field，
latency 以 `id=ttfb_read|ttlb_write` 映射 READ/WRITE，Namespace 僅輸出文件與
實體 response 都證實的 transaction/error request-rate Gauge。未經證實的
Namespace throughput/latency public families 已移除。`tls.verify` 預設仍為
`true`，但企業自簽憑證可明確設定 `false`；正式環境不再因該選項拒絕啟動，
Exporter 會記錄不含 endpoint 的 WARN。

ECS-010 在無法取得 3.6/3.7/3.8.0 實體設備時建立分層驗證方式：以 Dell 官方
文件比對 contract，以四 Profile synthetic fixture replay 驗證共同 mapping，並提供
`ecs-flux-probe` 讓客戶、Dell 或合作夥伴在 exact-build appliance 執行相同的唯讀
query/parser。Probe report 只含 build、Profile、policy、受控錯誤與 series 數量，
省略 endpoint、credential/token、resource identity、原始 response 與 metric value。
文件/fixture/CE/probe evidence 均不會自動提升 Profile certification。

ECS-011 依使用者決策改採功能級跨版本驗證：同一 production path 功能只要在任一
目標 ECS 版本取得真實 CE/appliance evidence，即視為 3.6、3.7、3.8.0、3.8.1 的
共用功能均已驗證。已知版本差異、`unavailable` capability 與從未在任何版本產生
必要欄位的功能不得繼承。`tested_builds` 與 `sandbox_certified` 繼續只記錄
exact-build／完整 Profile 事實，不再限制共享功能驗證狀態。

ECS-012 將第一個評估版本固定為 `v1.0.0-rc.1`。Prerelease tag 仍必須通過
deterministic/race、container、Kubernetes schema、合成 target-scale、source/license、
linux/amd64 與 linux/arm64 image scan、SBOM、keyless signing 及 OCI provenance，並以
GitHub Pre-release 發布。GitHub-native attestation 依 repository plan 執行；private RC
缺少平台支援時必須發布 signed boundary asset，stable 仍要求成功。只有帶 Semantic
Version prerelease suffix 的 tag 可略過 exact
ECS 3.8.1.4、exact CE 3.8.0.3、deployed E2E 與 deployed performance protected jobs；
stable tag 缺少任一成功結果都不能發布。RC 不代表 production Gate 4。

### Requirement Index

- **REQ-001 — Multi-cluster connection and authentication:** 支援至少一個、設計上至少
  十個 Dell ECS Cluster；每個 Cluster 的 credential、token、TLS、timeout 與錯誤
  狀態互相隔離。
- **REQ-002 — Scheduled collection and cache:** 各 Collector 獨立排程、避免重疊，
  以原子方式更新 thread-safe cache，scrape 只能讀 cache，失敗時保留最後成功資料。
- **REQ-003 — Cluster and node observability:** 收集可由目標 ECS API 證實的 Cluster、
  VDC 與 Node health/capacity/resource 資料，使用正確 base unit 與 metric type。
- **REQ-004 — Namespace, bucket and quota:** 提供 Namespace/Bucket inventory、used
  capacity、object count 與已設定的 quota；未設定 quota 不得推導或替代。
- **REQ-005 — Performance metric semantics:** Throughput、latency、request 與 HTTP
  status metrics 必須保留 ECS 原始 scope 與時間語意，非單調累計值不得宣告為
  Counter。
- **REQ-006 — Replication and recovery:** 在目標 ECS API 支援且 mapping 完成時，
  收集 replication status/lag 與 recovery status/progress。
- **REQ-007 — Inventory REST API:** 提供 `/api/v1` 唯讀 API、受控 pagination/filter、
  null semantics、Bearer/reverse-proxy authentication 與 RFC 9457 error response。
- **REQ-008 — Health and self-monitoring:** 提供 `/health`、`/api/v1/health` 及
  `ecs_exporter_*` metrics，區分 `UP`、`DEGRADED`、`DOWN` 與 stale cache。
- **REQ-009 — Resilience and isolation:** 明確處理 timeout、retry/backoff/jitter、
  rate limit、circuit breaker、pagination、partial failure 與 bounded concurrency。
- **REQ-010 — Configuration, secrets and logging:** 啟動時驗證 YAML/environment/secret
  reference；預設驗證 TLS，並從 log、metric、evidence 排除 credential/token 與敏感
  response。
- **REQ-011 — Deployment and non-functional targets:** 提供 Docker、Kubernetes／
  Helm 與 Linux Bare Metal/systemd deployment，支援規定規模，並驗證 response
  time、memory、CPU、availability、release integrity 與 UTC/base-unit 目標。
- **REQ-012 — Compatibility and API mapping:** 採 Semantic Versioning，維持 V1.x
  public contract，並在實作前用官方文件、sandbox 與 redacted fixture 完成目標 ECS
  版本的 API mapping。

### Acceptance Criteria

- Given 兩個使用獨立 credential 的 Cluster，when 同時 polling，then token、cache、
  errors 與輸出 labels 不得跨 Cluster 混用。
- Given Prometheus 呼叫 `/metrics`，when cache 有資料、stale 或尚未初始化，then
  request 不呼叫 ECS，並在三種狀態都輸出可用的 Exporter self-monitoring metrics。
- Given 任一 Collector 成功取得完整 response，when 更新 cache，then reader 只會看見
  更新前或更新後的完整 snapshot，不會看見部分資料。
- Given page、resource 或 Cluster 暫時失敗，when refresh 未完整成功，then 保留最後
  完整成功資料、增加錯誤 metrics，並將相關 health 降為 `DEGRADED`。
- Given Bucket 未設定 soft/hard quota，when 產生 metrics 與 Inventory response，
  then 不輸出對應 quota metric，欄位為 `null`，且 configured flag 為 `false`。
- Given ECS 回傳非 `*_delta` request rate，when mapping Prometheus metric，then 使用
  requests/second Gauge，不使用 `_total` Counter。
- Given API 回傳缺少或 null 欄位，when 建立 internal model，then exporter 不崩潰、
  不填入推測值，並依 contract 回傳 null、略過 metric 或標記 collector error。
- Given Inventory API query 超過 page limit 或不合法，when request 到達，then 不觸發
  未受控 ECS request，並回傳受控的 RFC 9457 client error。
- Given password、token、cookie、Authorization header 或敏感 response，when 發生正常
  或錯誤 logging，then log、metric labels 與 verification artifacts 均不包含原值。
- Given 單一 ECS Cluster authentication 或 network failure，when Kubernetes 執行
  probe，then liveness 保持成功；readiness 依是否仍有可服務 cache 判斷。
- Given 10,000 Buckets、至少 10 Clusters 與每 15 秒 scrape 的測試負載，when 執行
  performance test，then 以實測報告判定第 20 節的 latency、memory 與 CPU 目標，
  不得以估計值宣稱通過。
- Given adapter 偵測到 ECS 3.6.x、3.7.x、3.8.0.x 或 3.8.1.x，when 選擇 Profile，
  then 必須使用版本範圍與 capability 契約，未知版本拒絕，mixed-version 只使用能力
  交集且停用 interval-derived rates。
- Given Node Management API 成功但獨立 Flux Resources API 失敗，when 更新 cache 與
  health，then 必須保留 Node inventory/health、不得捏造 resource values，並以
  `DEGRADED` 與 API/collector error telemetry 呈現。
- Given release prerequisite、scanner、runtime、exact-build endpoint、protected
  credential 或 reviewer evidence 缺少，when 執行 production release gate，then
  必須以 blocked/failed 結束，不得把 skip、fixture 或 synthetic precheck 當成通過。
- Given tag 含合法 Semantic Version prerelease suffix，when 執行 release workflow，
  then 必須保留 RC 的程式、container/schema、供應鏈、signing/OCI provenance gates，
  並標記 GitHub Pre-release；private RC 缺 native attestation 支援時發布 signed boundary
  asset；given tag 為 stable，then native attestation、exact ECS/CE、deployed E2E 與
  deployed performance jobs 必須全部成功後才能 publish。
- Given 某功能已在任一目標 ECS 版本透過 production path 完成真實驗證，when 更新
  支援矩陣，then 四個目標 Profile 都標示該功能為 `validated-shared`，不要求每個
  版本重複相同功能測試。
- Given 功能有已知版本差異、Profile 為 `unavailable` 或沒有任何 target-version
  live evidence，when 套用共享驗證，then 不得標示為 `validated-shared`。
- Given Profile 沒有 exact-build execution，when 更新 `tested_builds` 或
  `sandbox_certified`，then 仍不得填入未執行或未完成完整 certification 的事實；
  這兩欄與功能級共享驗證分離。
- Given operator 執行相容性 Probe，when 產生或分享 report，then 只允許完整 build、
  Profile/capability、受控 error type/HTTP status 與 series count；不得包含 endpoint、
  credential/token、resource identity、raw response 或 metric value。
- Given 四版本 fixture replay 或 ECS CE Flux 結果，when 更新版本特定能力，then
  3.7/3.8.0 interval rates 仍為 unavailable，3.8.1 仍為 conditional；成功的
  3.8.1.1 latest-snapshot 功能可依 ECS-011 繼承至其他 Profile。
- Given capability 在 Profile 是 `conditional`，when cluster 未明列啟用，then
  Collector 不呼叫該 API、不更新該 domain cache，並以 `skipped` 而非 error/success
  cache refresh 呈現。
- Given 某 domain 的最後成功時間超過 `maxStale`，when Prometheus scrape，then 不輸出
  該 domain series，但仍輸出 build/Profile、cache age、last-success 與 cached-resource
  self metrics。
- Given Namespace batch Bucket billing endpoint 不支援或遺漏特定 Bucket，when
  enrichment，then 只對必要項目 fallback 至單筆 GET；任一必要項目失敗時不替換完整
  Bucket snapshot。
- Given Bucket quota 或單筆 billing 回傳 inherited nested 或 ECS 3.8.0 top-level
  envelope，when parser 建立 Bucket model，then 兩種各自可接受；若同一 response
  同時出現兩種 envelope，then 以 ambiguous mapping error 拒絕。
- Given Namespace batch Bucket billing 因 JSON `null` body 回傳 HTTP 500/code999，
  when 分類相容性，then 不得判定 endpoint unsupported 或 fallback。
- Given ECS 3.8.0.3 batch billing，when 查詢三個 Bucket，then request body 使用
  `{"id":[bucket names...]}` 且接受 plural `bucket_billing_infos[]` response。
- Given batch endpoint 回 HTTP 404/405/501 或成功 response 遺漏 requested Bucket，
  when enrichment，then fallback 至必要的單筆 GET；generic 500 與
  500/code999 不 fallback，任一必要項目失敗時不替換完整 snapshot。
- Given billing API 對已知 10,000,000 bytes 回傳 `9765.625 KB`，when 轉換成
  Prometheus base unit，then 結果必須精確為 10,000,000 bytes；不得由此推測
  billing MB/GB/TB 或 capacity/quota GB 也使用 binary multiplier。

---

## 1. 文件目的

本文件定義 **Dell ECS Metrics Exporter** 的產品需求、系統架構、Dell ECS API 整合方式、Prometheus Metrics、Inventory API、快取策略、錯誤處理與部署需求。

本 Exporter 用於定期擷取 Dell ECS Management API／Monitoring API 資料，轉換為 Prometheus 可擷取的 Metrics，並提供 Inventory REST API，作為 Prometheus 與 Grafana 的 Dell ECS 監控資料來源。

本文件聚焦於 Exporter 本身及 Dell ECS API 整合，不涵蓋 Grafana Dashboard 實作、告警平台、SNMP 或其他儲存產品。

---

## 2. 系統目標

本系統負責：

- 定期向 Dell ECS Management API／Monitoring API 擷取監控資料。
- 將 Dell ECS 資料轉換為 Prometheus Metrics。
- 提供 `/metrics` Endpoint，供 Prometheus 使用 Pull Model 擷取。
- 提供 Cluster、Node、Namespace、Bucket 等 Inventory REST API。
- 提供 Bucket 使用容量、Soft Quota、Hard Quota 與物件數等資料。
- 提供 Metrics Cache，避免每次 Prometheus Scrape 都直接呼叫 Dell ECS API。
- 支援多個 Dell ECS Cluster。
- 提供 Exporter Health Check 與自我監控 Metrics。

本系統不保存歷史監控資料。歷史資料、趨勢分析、容量預測與告警由 Prometheus、Grafana 及 Alertmanager 負責。

---

## 3. 系統範圍

### 3.1 包含範圍

- 多 Dell ECS Cluster 設定與管理。
- Dell ECS API Authentication 與 Token Lifecycle 管理。
- Cluster、Node、Namespace、Bucket、Performance、Replication 與 Recovery 資料收集。
- Prometheus Metrics Mapping。
- Inventory REST API。
- In-memory Cache。
- Health Check。
- Exporter Self-monitoring。
- Docker、Kubernetes 與 Helm 部署。

### 3.2 不包含範圍

- Grafana Dashboard JSON 或 Dashboard Provisioning。
- Prometheus Alert Rule 實作。
- 告警事件管理與通知。
- 長期歷史資料保存。
- SNMP Query、SNMP Trap 或 Syslog 收集。
- Dell ECS 設定變更或管理操作。
- Dell ECS 以外的儲存產品。
- OpenTelemetry Native Export。

---

## 4. 系統架構

```text
                    Dell ECS Cluster
                            │
                 HTTPS REST Management API
                            │
          ┌───────────────────────────────────┐
          │ Dell ECS Metrics Exporter         │
          │                                   │
          │ • Scheduler                       │
          │ • ECS API Client                  │
          │ • Authentication Manager          │
          │ • Cluster Collector               │
          │ • Node Collector                  │
          │ • Namespace Collector             │
          │ • Bucket Collector                │
          │ • Performance Collector           │
          │ • Replication Collector           │
          │ • Metrics Mapper                  │
          │ • Inventory Mapper                │
          │ • Memory Cache                    │
          │ • Prometheus Endpoint             │
          │ • Inventory REST API              │
          │ • Health & Self-monitoring        │
          └───────────────────────────────────┘
                      │               │
                      ▼               ▼
                 Prometheus      Inventory API
                      │
                      ▼
                   Grafana
```

### 4.1 核心資料流程

```text
Scheduler
   ↓
Authentication Manager
   ↓
ECS API Client
   ↓
Collector
   ↓
Parser / Validator / Unit Converter
   ↓
Internal Data Model
   ↓
Metrics Cache + Inventory Cache
   ↓
/metrics + /api/v1/*
```

Prometheus Scrape 不得直接觸發 Dell ECS API 呼叫；`/metrics` 必須只讀取 Exporter Cache。

---

## 5. 核心模組

### 5.1 Scheduler

Scheduler 負責依資料類型觸發不同 Collector。

要求：

- 各 Collector 使用獨立排程。
- 單一 Collector 失敗不得阻塞其他 Collector。
- 排程應避免同一時間大量呼叫 Dell ECS API。
- 支援加入隨機抖動時間，避免多個 Exporter Instance 同步呼叫 ECS。
- 每次執行需記錄開始時間、完成時間、執行結果與持續時間。

### 5.2 Authentication Manager

負責：

- Dell ECS Login。
- Token 取得、保存與更新。
- Token 到期處理。
- Authentication Failure 後重新登入。
- 多 Cluster 的獨立 Credential 與 Token 管理。
- 避免將帳號、密碼與 Token 寫入 Log 或 Metrics Label。

### 5.3 ECS API Client

負責：

- HTTPS Connection。
- Connect Timeout 與 Read Timeout。
- Retry 與 Backoff。
- HTTP Status Code 處理。
- Pagination。
- Rate Limiting。
- TLS Certificate 驗證。
- Response Size 與 API Duration 記錄。

### 5.4 Collector

V1.0 至少包含：

- Cluster Collector
- Node Collector
- Namespace Collector
- Bucket Collector
- Performance Collector
- Replication Collector
- Recovery Collector

每個 Collector 必須：

1. 呼叫指定 ECS API。
2. 驗證 Response。
3. 轉換時間、容量、百分比與狀態值。
4. 建立統一 Internal Data Model。
5. 原子更新對應 Cache。
6. 更新 Collector Health 與 Self-monitoring Metrics。

### 5.5 Parser 與 Validator

負責：

- JSON Parsing。
- 必要欄位驗證。
- Null 與缺少欄位處理。
- Bytes、KB、MB、GB、TB 依 logical API 與已驗證 unit contract 轉換為 Bytes；
  billing KB 使用 1024，其他既有 decimal mapping 不因 suffix 猜測而改變。
- 毫秒統一轉換為 Seconds。
- ECS 狀態字串轉換為數值狀態。
- API Version 差異兼容。

### 5.6 Metrics Mapper

將 Internal Data Model 轉換為 Prometheus Metrics。

要求：

- Metric 命名符合 Prometheus Naming Best Practices。
- Base Unit 使用 bytes、seconds、ratio 或 count。
- 狀態資訊以 Gauge 表示。
- 累計型請求數以 Counter 表示；若 ECS API 僅提供區間統計值，不得錯誤宣告為單調遞增 Counter。
- 高 Cardinality Label 必須受控。
- 不得將 Owner、Description、完整 URL、錯誤訊息或 Token 放入 Metric Label。

### 5.7 Inventory Mapper

將 ECS 靜態與低頻變動資訊轉換為 Inventory REST API Model，包括：

- Cluster Inventory
- Node Inventory
- Namespace Inventory
- Bucket Inventory
- Replication Inventory

### 5.8 Memory Cache

Cache 分為：

- Metrics Cache
- Inventory Cache
- Collector State Cache

要求：

- Thread-safe。
- Collector 更新採原子替換，不得讓讀取者取得部分更新資料。
- 每筆資料保留 `collectedAt`。
- 每個 Collector 保留 `lastSuccessAt`、`lastAttemptAt` 與 `lastError` 摘要。
- API 更新失敗時保留最後一次成功資料。
- Cache 過期時 Health 應顯示 `DEGRADED`。
- Cache 不保存歷史版本。

### 5.9 Prometheus Exporter

提供：

```http
GET /metrics
```

要求：

- 使用 Prometheus Text Exposition Format 或 OpenMetrics Format。
- 不主動 Push Metrics。
- 回應只讀取 Cache。
- 預設回應時間小於 3 秒。
- Cache 無資料時仍需輸出 Exporter Self-monitoring Metrics。

---

## 6. Dell ECS API 整合範圍

實際 API URI、HTTP Method、Request Parameter 與 Response Schema，須依部署中的 Dell ECS 版本及官方 API 文件確認。本規格定義 V1.0 必須涵蓋的資料領域。

### 6.1 Cluster

收集：

- Cluster／VDC Health
- ECS Version
- Total Capacity
- Used Capacity
- Available Capacity
- Object Count
- Bucket Count
- Namespace Count

### 6.2 Node

收集：

- Node Identifier
- Node Name／IP
- Node Health
- Node State
- CPU Usage
- Memory Usage
- Disk Usage
- Network Receive／Transmit
- ECS Service／Process Status

無法由 ECS Management API 取得的硬體資訊，不屬於 V1.0 必須輸出範圍，不得以推測值填入。

### 6.3 Namespace

收集：

- Namespace Name
- Owner
- Used Capacity
- Quota
- Bucket Count
- Replication Policy／Group
- Audit Setting（若 API 提供）

### 6.4 Bucket

收集：

- Bucket Name
- Namespace
- Owner
- Used Capacity
- Object Count
- Soft Quota
- Hard Quota
- Versioning
- Encryption
- Replication Group
- Object Lock
- Retention
- Audit Setting
- Created Time
- Last Modified Time

若 Bucket 未設定 Quota，Exporter 不得將 Cluster Capacity 當作 Bucket Quota；應依 Metric 規格選擇不輸出該 Metric，並在 Inventory API 明確表示 `null` 或 `configured=false`。

### 6.5 Performance

視 ECS API 實際支援情況收集：

- Read Throughput
- Write Throughput
- Read Latency
- Write Latency
- GET Requests
- PUT Requests
- DELETE Requests
- HEAD Requests
- HTTP 2xx／4xx／5xx Distribution

若 ECS API 僅提供 Cluster、Namespace 或 Node 層級 Performance，不得推導成 Bucket 層級資料。

### 6.6 Replication 與 Recovery

收集：

- Replication Group
- Source／Target VDC
- Replication Status
- Replication Lag
- Recovery Status
- Recovery Progress

---

## 7. Authentication 與連線安全

### 7.1 支援方式

Exporter 應支援 Dell ECS 實際版本所提供的 Management API Authentication，包括：

- Login 後取得 Authentication Token。
- Token Renewal 或重新登入。
- HTTPS。

若特定版本另支援 Basic Authentication 或其他 Token 機制，可透過 Adapter 擴充，但不得降低預設安全要求。

### 7.2 Credential 管理

- Password 不得直接硬編碼在 Image。
- Kubernetes 環境使用 Secret 掛載或 Secret Manager。
- 設定檔可引用環境變數。
- Log 必須遮罩 Password、Token、Cookie 與 Authorization Header。
- 每個 ECS Cluster 使用獨立 Credential。

### 7.3 TLS

- 預設啟用憑證驗證。
- 支援掛載企業 CA Bundle。
- `insecureSkipVerify` 僅供測試環境使用，Production 預設禁止。

---

## 8. Polling 與排程規格

預設 Polling Frequency：

| Collector | 預設頻率 |
|---|---:|
| Cluster Collector | 60 秒 |
| Node Collector | 60 秒 |
| Performance Collector | 60 秒 |
| Replication Collector | 120 秒 |
| Recovery Collector | 120 秒 |
| Namespace Collector | 300 秒 |
| Bucket Collector | 300 秒 |

要求：

- 所有頻率可由 Cluster 級或全域設定覆寫。
- Prometheus Scrape Interval 與 Collector Polling Interval 彼此獨立。
- Polling Duration 不得超過下一次排程週期；若發生，應跳過重疊執行或採 Single-flight 機制。
- Bucket 數量較多時，必須支援 Pagination、批次處理與有限並行。
- Performance Collector 不應預設逐一同步查詢所有 Bucket；應優先使用 ECS 提供的聚合或批次 API。

---

## 9. Prometheus Metrics 規格

### 9.1 通用 Label

依 Metric Scope 使用以下 Label：

- `cluster`
- `site`
- `environment`
- `vdc`
- `node`
- `namespace`
- `bucket`
- `operation`
- `status_class`
- `replication_group`
- `source_vdc`
- `target_vdc`
- `interface`

所有 Label 值必須先正規化與限制長度。

### 9.2 Cluster Metrics

| Metric | Type | Unit | 說明 |
|---|---|---|---|
| `ecs_cluster_health` | Gauge | state | Cluster Health，正常為 1，異常為 0 |
| `ecs_cluster_capacity_total_bytes` | Gauge | bytes | Cluster 總容量 |
| `ecs_cluster_capacity_used_bytes` | Gauge | bytes | Cluster 已使用容量 |
| `ecs_cluster_capacity_available_bytes` | Gauge | bytes | Cluster 可用容量 |
| `ecs_cluster_buckets` | Gauge | count | Bucket 數量 |
| `ecs_cluster_namespaces` | Gauge | count | Namespace 數量 |
| `ecs_cluster_objects` | Gauge | count | Object 數量 |

### 9.3 Node Metrics

| Metric | Type | Unit | 說明 |
|---|---|---|---|
| `ecs_node_health` | Gauge | state | Node Health，正常為 1，異常為 0 |
| `ecs_node_cpu_usage_ratio` | Gauge | ratio | CPU 使用率，範圍 0～1 |
| `ecs_node_memory_used_bytes` | Gauge | bytes | 已使用記憶體 |
| `ecs_node_memory_total_bytes` | Gauge | bytes | 總記憶體 |
| `ecs_node_disk_used_bytes` | Gauge | bytes | 已使用磁碟容量 |
| `ecs_node_disk_total_bytes` | Gauge | bytes | 總磁碟容量 |
| `ecs_node_network_receive_bytes_total` | Counter | bytes | Flux `net.bytes_recv` 累計接收流量，保留 bounded `interface` Label 並處理 reset |
| `ecs_node_network_transmit_bytes_total` | Counter | bytes | Flux `net.bytes_sent` 累計送出流量，保留 bounded `interface` Label 並處理 reset |
| `ecs_node_service_health` | Gauge | state | 設定啟用且既有 Node health response 含 service/process 時輸出；labels 為 cluster、node、kind、service |

### 9.4 Namespace Metrics

| Metric | Type | Unit | 說明 |
|---|---|---|---|
| `ecs_namespace_capacity_used_bytes` | Gauge | bytes | Namespace 已使用容量 |
| `ecs_namespace_quota_bytes` | Gauge | bytes | Namespace Quota；未設定時不輸出 |
| `ecs_namespace_buckets` | Gauge | count | Namespace 內 Bucket 數量 |
| `ecs_namespace_objects` | Gauge | count | Namespace 內 Object 數量，若 API 提供 |

### 9.4.1 VDC 與 Namespace Performance Metrics

Performance 值只保留 API 原始 VDC 或 Namespace scope，不展開成 Bucket。Dell
`monitoring_vdc` 的非 `*_delta` measurement 是 rate：VDC throughput、latency、
request rate 與 Namespace request rate 都使用 Gauge。Namespace scope 沒有已證實的
throughput/latency measurement，因此不輸出對應 family：

| Metric | Type | Unit | Scope |
|---|---|---|---|
| `ecs_vdc_read_throughput_bytes_per_second` | Gauge | bytes/second | VDC |
| `ecs_vdc_write_throughput_bytes_per_second` | Gauge | bytes/second | VDC |
| `ecs_vdc_request_latency_seconds` | Gauge | seconds | VDC + operation + quantile |
| `ecs_vdc_requests` | Gauge | requests/second | VDC + operation + status_class |
| `ecs_namespace_requests` | Gauge | requests/second | VDC + Namespace + operation + status_class |

### 9.5 Bucket Metrics

| Metric | Type | Unit | 說明 |
|---|---|---|---|
| `ecs_bucket_used_bytes` | Gauge | bytes | Bucket 已使用容量 |
| `ecs_bucket_soft_quota_bytes` | Gauge | bytes | Bucket Soft Quota；未設定時不輸出 |
| `ecs_bucket_hard_quota_bytes` | Gauge | bytes | Bucket Hard Quota；未設定時不輸出 |
| `ecs_bucket_objects` | Gauge | count | Bucket Object 數量 |
| `ecs_bucket_read_throughput_bytes_per_second` | Gauge | bytes/second | 保留名稱；四個目標 Profile 均無 Bucket scope evidence，因此 V1.0 不輸出 |
| `ecs_bucket_write_throughput_bytes_per_second` | Gauge | bytes/second | 保留名稱；四個目標 Profile 均無 Bucket scope evidence，因此 V1.0 不輸出 |
| `ecs_bucket_request_latency_seconds` | Gauge | seconds | 保留名稱；四個目標 Profile 均無 Bucket scope evidence，因此 V1.0 不輸出 |
| `ecs_bucket_requests` | Gauge/Counter | count | 保留名稱；四個目標 Profile 均無 Bucket scope evidence，因此 V1.0 不輸出 |

### 9.6 Replication Metrics

| Metric | Type | Unit | 說明 |
|---|---|---|---|
| `ecs_replication_status` | Gauge | state | Replication Status，正常為 1，異常為 0 |
| `ecs_replication_lag_seconds` | Gauge | seconds | Replication Lag |
| `ecs_recovery_progress_ratio` | Gauge | ratio | Recovery Progress，範圍 0～1；labels 包含 cluster、replication group、source/target VDC 與 kind，避免多 link series 衝突 |

### 9.7 Metric Type 限制

ECS API 若回傳「最近一段時間的 Request Count」，該值不是單調遞增 Counter，應輸出為 Gauge 或重新計算成 Rate，不得直接命名為 `_total`。

---

## 10. Bucket Quota 與容量整合規格

Exporter 必須讓 Prometheus 與 Grafana 能取得每個 Bucket 的：

- Used Capacity
- Soft Quota
- Hard Quota
- Object Count

主要 Metrics：

```text
ecs_bucket_used_bytes
ecs_bucket_soft_quota_bytes
ecs_bucket_hard_quota_bytes
ecs_bucket_objects
```

Bucket Hard Quota 使用率由 PromQL 計算：

```promql
100 *
ecs_bucket_used_bytes
/
ecs_bucket_hard_quota_bytes
```

Bucket Hard Quota 剩餘容量：

```promql
ecs_bucket_hard_quota_bytes
-
ecs_bucket_used_bytes
```

Soft Quota 使用率：

```promql
100 *
ecs_bucket_used_bytes
/
ecs_bucket_soft_quota_bytes
```

Exporter 不額外輸出 Percentage 或 Remaining Metrics，避免重複資料與計算邏輯分散。

### 10.1 未設定 Quota

當 Bucket 未設定 Soft 或 Hard Quota：

- 不輸出對應 Quota Metric。
- Inventory API 回傳 `null`。
- Inventory API 另提供 `softQuotaConfigured` 與 `hardQuotaConfigured`。
- Grafana 查詢需處理缺少 Quota 的 Bucket。

---

## 11. Inventory REST API

Base Path：

```text
/api/v1
```

### 11.1 Endpoint 清單

| Method | Endpoint | 說明 |
|---|---|---|
| GET | `/api/v1/clusters` | 查詢 Cluster Inventory |
| GET | `/api/v1/clusters/{cluster}` | 查詢單一 Cluster |
| GET | `/api/v1/nodes` | 查詢 Node Inventory |
| GET | `/api/v1/namespaces` | 查詢 Namespace Inventory |
| GET | `/api/v1/buckets` | 查詢 Bucket Inventory |
| GET | `/api/v1/replications` | 查詢 Replication Inventory |
| GET | `/api/v1/health` | 查詢 Exporter 與各 Cluster Health |
| GET | `/api/v1/version` | 查詢 Exporter 版本與 Build 資訊 |

### 11.2 分頁與篩選

`/api/v1/buckets` 與 `/api/v1/namespaces` 必須支援：

- `cluster`
- `namespace`
- `name`
- `page`
- `size`
- `sort`

預設 `size=100`，最大值由設定控制。

### 11.3 Bucket Inventory Model

每個 Bucket 至少包含：

```json
{
  "cluster": "prod-ecs",
  "namespace": "ai",
  "name": "model-artifacts",
  "owner": "ai-team",
  "usedBytes": 1649267441664,
  "objectCount": 18000000,
  "softQuotaBytes": 1979120929996,
  "hardQuotaBytes": 2199023255552,
  "softQuotaConfigured": true,
  "hardQuotaConfigured": true,
  "versioningEnabled": true,
  "encryptionEnabled": true,
  "replicationGroup": "rg-prod-dr",
  "objectLockEnabled": false,
  "retention": null,
  "auditEnabled": true,
  "createdAt": "2026-07-01T02:30:00Z",
  "lastModifiedAt": "2026-07-25T08:15:00Z",
  "collectedAt": "2026-07-25T09:00:00Z"
}
```

若 ECS API 不提供某欄位，回傳 `null`，不得填入推測值。

### 11.4 Namespace Inventory Model

至少包含：

- Cluster
- Namespace Name
- Owner
- Used Capacity
- Quota
- Bucket Count
- Object Count（若 API 提供）
- Replication Policy／Group
- Audit Setting（若 API 提供）
- Collected Time

### 11.5 API 回應格式

集合型 API 建議格式：

```json
{
  "items": [],
  "page": 0,
  "size": 100,
  "totalElements": 0,
  "totalPages": 0,
  "collectedAt": "2026-07-25T09:00:00Z"
}
```

錯誤回應應採 RFC 9457 Problem Details。

---

## 12. Health Check

### 12.1 Endpoint

```http
GET /health
GET /api/v1/health
```

### 12.2 狀態

- `UP`
- `DEGRADED`
- `DOWN`

### 12.3 判斷原則

`UP`：

- Scheduler 正常。
- 至少最近一次主要 Collector 成功。
- Cache 未超過允許的最大年齡。
- Authentication 正常。

`DEGRADED`：

- 部分 Cluster 或 Collector 失敗。
- 使用上一版 Cache。
- Cache 已超過 TTL，但未超過最大容忍時間。

`DOWN`：

- Exporter 無法啟動。
- 所有 Cluster Authentication 失敗。
- 所有主要 Cache 無可用資料且超過最大容忍時間。

Kubernetes Liveness 不應因單一 ECS Cluster 無法連線而失敗；Readiness 可依是否具備可服務的 Cache 判斷。

---

## 13. Exporter 自我監控 Metrics

所有 Self-monitoring Metrics 建議使用 `ecs_exporter_` Prefix：

| Metric | Type | 說明 |
|---|---|---|
| `ecs_exporter_api_requests_total` | Counter | 呼叫 ECS API 次數，Label 包含 cluster、api、result |
| `ecs_exporter_api_errors_total` | Counter | ECS API 錯誤次數 |
| `ecs_exporter_api_request_duration_seconds` | Histogram | ECS API 呼叫時間 |
| `ecs_exporter_api_response_size_bytes` | Histogram | ECS API response bytes，Label 包含 cluster、api、result |
| `ecs_exporter_collector_runs_total` | Counter | Collector 執行次數 |
| `ecs_exporter_collector_errors_total` | Counter | Collector 錯誤次數 |
| `ecs_exporter_collector_duration_seconds` | Histogram | Collector 執行時間 |
| `ecs_exporter_cache_refresh_total` | Counter | 該 collector domain 完整替換成功次數；`skipped` 不增加 |
| `ecs_exporter_cache_refresh_errors_total` | Counter | Cache 更新失敗次數 |
| `ecs_exporter_last_success_timestamp_seconds` | Gauge | Collector 最近成功時間 |
| `ecs_exporter_cache_age_seconds` | Gauge | Cache 年齡 |
| `ecs_exporter_cached_resources` | Gauge | Cache 中 Resource 數量，Label 包含 resource_type |
| `ecs_exporter_build_info` | Gauge | Build、Version、Commit 資訊，固定為 1 |

不得在 Label 中放入例外訊息或完整 URL。

---

## 14. Error Handling 與韌性

### 14.1 Timeout

每個 Cluster 可設定：

- Connect Timeout
- Read Timeout
- Overall Request Timeout

### 14.2 Retry

預設：

- 最大重試 3 次。
- 指數退避。
- 支援 Jitter。
- 僅對 Timeout、Connection Error、HTTP 429、部分 5xx 重試。
- 4xx Authentication Failure 交由 Authentication Manager 處理。
- 一般 4xx 不自動重試。

### 14.3 Circuit Breaker

建議每個 Cluster 使用獨立 Circuit Breaker，避免單一失效 Cluster 消耗全部執行緒。

### 14.4 Partial Failure

- 單一 Bucket 或單一 Page 解析失敗時，記錄錯誤並依設定決定是否保留上一版完整 Cache。
- 不得以部分清單覆蓋上一版完整 Bucket Inventory，除非明確標記此次更新為完整成功。
- 多 Cluster 彼此隔離。

### 14.5 Stale Data

API 更新失敗時：

- 保留最後一次成功 Cache。
- Self-monitoring Metric 顯示 Cache Age。
- Health 改為 `DEGRADED`。
- `/metrics` 持續提供舊資料，直到超過 `maxStale`；超過後停止該 collector domain
  series，但保留 build/Profile/cache age/last-success/cached-resource self metrics。

---

## 15. Logging 與稽核

Log 使用結構化 JSON，至少包含：

- Timestamp
- Log Level
- Service Name
- Cluster
- Collector
- Correlation ID
- API Logical Name
- HTTP Status
- Duration
- Retry Count
- Result
- Error Type

禁止記錄：

- Password
- Token
- Cookie
- Authorization Header
- 完整私人 URL 或 query
- 完整敏感 Response Body

Inventory API 的讀取不屬於 ECS 管理操作，不要求業務稽核，但應保留 Access Log。

---

## 16. Configuration Specification

建議 YAML：

```yaml
server:
  listenAddress: ":8080"
  tls:
    certFile: ""
    keyFile: ""

prometheus:
  path: /metrics
  protected: false

cache:
  staleTolerance: 15m
  maxStale: 1h

collector:
  defaultTimeout: 30s
  retry:
    maxAttempts: 3
    initialBackoff: 1s
    maxBackoff: 10s
  rateLimit:
    requestsPerSecond: 10
    burst: 4

  intervals:
    cluster: 60s
    node: 60s
    performance: 60s
    replication: 120s
    recovery: 120s
    namespace: 300s
    bucket: 300s

  concurrency:
    maxConcurrentRequestsPerCluster: 4
    bucketPageSize: 500
  jitterRatio: 0.1

security:
  inventoryApi:
    enabled: true
    authentication: token
    tokenFile: /var/run/secrets/ecs-exporter/inventory-token
    maxPageSize: 500

ecs:
  clusters:
    - name: prod-ecs
      site: primary-dc
      environment: production
      endpoint: https://ecs-management.example.internal
      usernameFile: /var/run/secrets/ecs-exporter/primary-username
      passwordFile: /var/run/secrets/ecs-exporter/primary-password
      tls:
        verify: true
        caFile: /etc/ecs-exporter/certs/company-ca.pem
      timeouts:
        connect: 10s
        read: 30s
        overall: 30s
      capabilities:
        enabledConditional: []
      nodeResources:
        filesystems: []
        networkInterfaces: []
        maxNetworkInterfaces: 16
        preferBondInterfaces: true
      replication:
        groups: []
        links: []

    - name: dr-ecs
      site: dr-dc
      environment: production
      endpoint: https://ecs-dr-management.example.internal
      username: ${ECS_DR_USERNAME}
      passwordFile: /var/run/secrets/ecs-exporter/dr-password
      tls:
        verify: true
        caFile: /etc/ecs-exporter/certs/company-ca.pem
```

要求：

- 啟動時驗證設定格式。
- Cluster Name 不得重複。
- 不合法 Polling Interval 應拒絕啟動。
- API rate 必須大於 0；burst 與每 Cluster request concurrency 必須有界。
- conditional capability 只有 Profile 值為 `conditional` 且 Cluster 明列時啟用；
  `unavailable` 不得由設定覆寫。
- `node_disk_capacity` 必須同時提供 filesystem allowlist；network interfaces 可明列
  allowlist，且永遠受最大 interface 數與 bond preference 控制。
- 支援 Environment Variable Override。
- 支援 Secret File Reference。
- `username`/`usernameFile` 與 `password`/`passwordFile` 各只能選一種。
- TLS verification 預設啟用並建議搭配正確 CA/SAN。企業自簽憑證可明確設定
  `tls.verify: false`，包括 production Cluster；Exporter 必須在啟動時輸出
  cluster/environment WARN，且不得記錄 endpoint。此選項會停用憑證鏈與主機身分
  驗證，必須由部署方承擔風險。自訂 CA、server certificate/key 與 authentication
  token/credential files 必須在啟動時可讀。

---

## 17. Prometheus 整合

Prometheus 設定範例：

```yaml
scrape_configs:
  - job_name: dell-ecs-exporter
    scrape_interval: 60s
    scrape_timeout: 10s
    metrics_path: /metrics

    static_configs:
      - targets:
          - dell-ecs-exporter.monitoring.svc:8080
        labels:
          service: dell-ecs-exporter
```

Prometheus Scrape Interval 可以短於 Bucket Polling Interval；在 Collector 尚未更新前，Prometheus 會擷取相同 Cache 值。

---

## 18. API 與服務安全

- `/metrics` 是否需認證由部署環境決定；內部 Kubernetes Network 可搭配 NetworkPolicy 限制。
- Inventory API 預設要求 Bearer Token 或企業既有反向代理認證。
- 支援 TLS Termination 於 Ingress 或 Service 本身。
- 僅允許 Read-only API。
- 不提供任何修改 Dell ECS 設定的 Endpoint。
- 支援限制 Inventory API 回傳筆數。
- 防止透過 Query Parameter 形成未受控的 ECS API 呼叫。

---

## 19. 部署需求

### 19.1 支援形式

- Docker Image
- Kubernetes Deployment
- Helm Chart
- Linux Bare Metal/systemd

### 19.2 Kubernetes

必須提供：

- Deployment
- Service
- ConfigMap
- Secret Reference
- Liveness Probe
- Readiness Probe
- Resource Requests／Limits
- PodDisruptionBudget（正式環境建議）
- NetworkPolicy（正式環境建議）
- ServiceMonitor（使用 Prometheus Operator 時）

NetworkPolicy 必須能限制 Prometheus ingress source 與 DNS/ECS egress destination。
Production values 必須使用 immutable image digest；文件用 CIDR placeholder 在套用前
必須替換。

### 19.3 Bare Metal/systemd

必須提供 dedicated non-login account、root-owned binary/Profile、外部 secret files、
啟動前 config/Profile validation、systemd sandboxing、safe upgrade/rollback、health
verification 與預設保留設定的 uninstall。預設 listen address 為 loopback；對外監聽
時必須另行限制 host firewall/reverse proxy。

### 19.4 Stateless 與多副本

Exporter 應設計為 Stateless，但多副本會各自呼叫 ECS API。V1.0 預設建議單副本搭配 Kubernetes 自動重啟。

若部署兩個以上副本，應採以下任一方式：

- Leader Election，僅 Leader 執行 Polling。
- 各副本獨立 Polling，但加入 Jitter 並確認 ECS API 負載可接受。
- 導入共享 Cache，非 V1.0 必要範圍。

---

## 20. 非功能需求

| 項目 | 目標 |
|---|---|
| `/metrics` 回應時間 | 正常情況小於 3 秒 |
| Inventory API P95 | 小型查詢小於 2 秒 |
| 可用性 | 99.9% 部署目標 |
| Cluster 數量 | 至少 10 個 |
| Node 數量 | 至少 100 個 |
| Bucket 數量 | 至少 10,000 個 |
| Memory | 目標小於 512 MB；依 Bucket 數調整 |
| CPU | 一般負載目標小於 1 Core |
| 時間格式 | UTC、ISO 8601 |
| Metric Base Unit | bytes、seconds、ratio、count |

實際資源需求須透過 ECS API Response Size 與 Bucket 數量進行壓力測試確認。

---

## 21. 測試與驗收

### 21.1 Unit Test

涵蓋：

- Authentication Token Lifecycle
- Response Parsing
- Unit Conversion
- Metrics Mapping
- Inventory Mapping
- Null／Missing Field
- Quota 未設定情境
- API Version 差異
- Nested/top-level envelope 與 ambiguous dual-envelope rejection
- Billing KB known-size conversion
- ECS error code normalization and exact fallback classification
- Cache 原子更新
- Retry 與 Error Classification

### 21.2 Integration Test

- 使用 Mock ECS API Server。
- 驗證 Login、Token Expiration、Pagination、429、5xx、Timeout。
- 驗證 batch billing request body encoding、plural success envelope，以及
  no entity/`{}`/`[]`/`null` 的 observed status/code matrix。
- 驗證 `{"id":[bucket names...]}` non-empty batch、404/405/501 或 missing-item
  fallback，以及 generic 500/500-code999 不 fallback。
- 驗證 `/metrics` 格式可被 Prometheus Parser 接受。
- 驗證 Inventory API Pagination 與 Filter。

### 21.3 Performance Test

至少測試：

- 10,000 Bucket Inventory。
- 多 Cluster 同時 Polling。
- Prometheus 每 15 秒 Scrape。
- ECS API 延遲與部分失敗。
- `/metrics` Response Size 與產生時間。

### 21.4 驗收標準

- Grafana 可透過 Prometheus 查詢每個 Bucket Used Capacity。
- 已設定 Quota 的 Bucket 可取得 Soft／Hard Quota。
- 未設定 Quota 的 Bucket 不產生錯誤 Quota Metric。
- ECS API 暫時失敗時仍可提供最後成功資料。
- Exporter 可指出 Cache Age 與 Collector 錯誤。
- 多 Cluster 資料不混淆。
- Release artifacts 的版本、full commit、checksums、SBOM、signature、provenance 與
  OCI digest 一致，Critical/High finding 無未核准例外。
- Bare Metal、container 或 Helm 中實際採用的部署模式通過 startup、security、
  liveness/readiness、authenticated Inventory 與 rollback 驗證。
- Target-scale deployed test 記錄 10 Cluster、100 Node、10,000 Bucket 的 p95、RSS、
  CPU 與 metrics response size；synthetic precheck 不取代此證據。

---

## 22. Dell ECS API Coverage Matrix

本章定義 Dell ECS API 資料領域與 Exporter 功能的對應關係。實際 URI、Response
Schema 與版本差異由 `DELL_ECS_API_MAPPING.md`、`profiles/` 與
`docs/ecs-api/` 管理。

| API 資料領域 | 對應 Exporter 模組 | 對應 Prometheus Metrics | 對應 Inventory API | Polling 頻率 | 預估 API 回應大小 |
|---|---|---|---|---:|---:|
| Cluster Information | Cluster Collector | `ecs_cluster_health`、`ecs_cluster_buckets`、`ecs_cluster_namespaces`、`ecs_cluster_objects` | `/api/v1/clusters` | 60 秒 | 小於 50 KB |
| Cluster Capacity | Cluster Collector | `ecs_cluster_capacity_total_bytes`、`ecs_cluster_capacity_used_bytes`、`ecs_cluster_capacity_available_bytes` | `/api/v1/clusters` | 60 秒 | 小於 30 KB |
| Node Information | Node/Node Resources Collectors | `ecs_node_health`、`ecs_node_service_health`、`ecs_node_cpu_usage_ratio`、`ecs_node_memory_used_bytes`、allowlisted disk metrics | `/api/v1/nodes` | 60 秒 | 約 5～20 KB／Node |
| Node Network Statistics | Node Resources Collector | Flux `net.bytes_recv/bytes_sent` -> Counter metrics；bounded interface allowlist/max/bond policy | `/api/v1/nodes` | 60 秒 | 約 2～10 KB／Node |
| Namespace Information | Namespace Collector | `ecs_namespace_buckets`、`ecs_namespace_objects` | `/api/v1/namespaces` | 300 秒 | 約 2～5 KB／Namespace |
| Namespace Capacity／Quota | Namespace Collector | `ecs_namespace_capacity_used_bytes`、`ecs_namespace_quota_bytes` | `/api/v1/namespaces` | 300 秒 | 約 2 KB／Namespace |
| Bucket Information | Bucket Collector | `ecs_bucket_used_bytes`、`ecs_bucket_objects` | `/api/v1/buckets` | 300 秒 | 約 2～5 KB／Bucket |
| Bucket Soft／Hard Quota | Bucket Collector | `ecs_bucket_soft_quota_bytes`、`ecs_bucket_hard_quota_bytes` | `/api/v1/buckets` | 300 秒 | 約 1～2 KB／Bucket |
| Bucket Performance | Performance Collector | 四個目標 Profile 均 `unavailable`；不得由 VDC/Namespace scope 展開 | 不提供 | 不排程 | 不適用 |
| VDC Performance | Performance Collector | VDC throughput、latency、request-rate Gauge；不得輸出為 Bucket metric | 不提供 Inventory performance resource | 依 Profile/capability | ECS 3.8.1.1 partial-live |
| Namespace Performance | Performance Collector | `ecs_namespace_requests` request-rate Gauge；沒有已證實的 Namespace throughput/latency mapping | 不提供 Inventory performance resource | 依 Profile/capability | ECS 3.8.1.1 partial-live |
| S3 Request Statistics | Performance Collector | `ecs_vdc_requests`、`ecs_namespace_requests`，保留 operation/status_class 與來源 scope | 不提供 Bucket scope | 依 Profile/capability | ECS 3.8.1.1 partial-live |
| HTTP Status Statistics | Performance Collector | VDC/Namespace request-rate Gauge；不得輸出為 Bucket status class | 不提供 Bucket scope | 依 Profile/capability | ECS 3.8.1.1 partial-live |
| Replication Status | Replication Collector | `ecs_replication_status`、`ecs_replication_lag_seconds` | `/api/v1/replications` | 120 秒 | 約 10～30 KB |
| Recovery Status | Recovery Collector | `ecs_recovery_progress_ratio`、相關狀態 Metric | `/api/v1/replications` 或後續獨立 Recovery API | 120 秒 | 約 10 KB |
| Exporter Health | Internal Health Collector | `ecs_exporter_api_requests_total`、`ecs_exporter_api_errors_total`、`ecs_exporter_cache_age_seconds`、`ecs_exporter_last_success_timestamp_seconds` 等 | `/health`、`/api/v1/health` | 每次 Collector 或 Scrape 更新 | 小於 5 KB |

### 22.1 Response Size 使用原則

以上回應大小為架構與容量規劃的初始估計值，不代表 Dell 官方保證值。正式相容性
認證前應使用目標 ECS 版本進行實測，並記錄：

- 平均 Response Size
- P95 Response Size
- API Duration
- `ecs_exporter_api_response_size_bytes` 的 average/P95
- Pagination Page Count
- Bucket／Namespace 數量成長對 Response Size 的影響

Bucket API 資料量通常會隨 Bucket 數量線性增加，必須支援 Pagination 與批次處理。

---

## 23. API Mapping Specification 要求

`DELL_ECS_API_MAPPING.md` 與 `docs/ecs-api/` 已建立 mapping contract。每支實際 API
至少定義：

- ECS Version
- API Logical Name
- HTTP Method
- URI
- Authentication Requirement
- Request Parameter
- Pagination 規格
- Response Schema
- 必要欄位與 Optional 欄位
- 對應 Internal Data Model
- 對應 Prometheus Metric
- Metric Type
- Label
- Base Unit
- 對應 Inventory API 欄位
- Polling Frequency
- Cache Strategy
- Error Handling
- 已知限制

所有新增或修改 Metrics 的變更，必須同步更新 API Mapping Specification。

---

## 24. 版本與相容性原則

- 採 Semantic Versioning。
- V1.x 不任意刪除或重新命名既有 Metric。
- Metric Deprecated 時至少保留一個 Minor Version。
- Inventory API 使用 `/api/v1`。
- 不相容 API 變更才新增 `/api/v2`。
- ECS API Adapter 應隔離不同 ECS Version 差異。
- Exporter 啟動後應輸出其支援與偵測到的 ECS Version。
- 支援 Profile 為 ECS 3.6、3.7、3.8.0、3.8.1；每個 Cluster 獨立選擇。
- ECS product version 使用四段 parser，不以一般三段 SemVer parser 取代。
- ECS 3.7/3.8.0 停用 Flux interval-derived rates；3.8.1 通過 live range gate 後才啟用。
- ECS 3.8.0/3.8.1 transport 必須保留 configured hostname、TLS SNI 與 HTTP Host。
- 未知版本預設拒絕；mixed-version 只允許 capability 交集。
- 相容性 Probe 必須使用與 Exporter 相同 Profile/query/parser、只執行唯讀 API，並
  產生去識別化 report；Probe 通過不是自動 certification。

---

## 25. V1.0 完成定義

V1.0 完成時必須具備：

1. 可設定並連線至少一個 Dell ECS Cluster。
2. 可完成 ECS Authentication 與 Token Lifecycle 管理。
3. 可定期收集 Cluster、Namespace、Bucket 與 Quota 資料。
4. 可透過 `/metrics` 提供 Prometheus Metrics。
5. 可透過 Inventory API 查詢 Bucket 設定與容量資料。
6. 可讓 Grafana 顯示每個 Bucket 的 Used Capacity、Soft Quota 與 Hard Quota。
7. 可區分未設定 Quota 的 Bucket。
8. ECS API 暫時失敗時可保留最後成功 Cache。
9. 具備 Health Check 與 Self-monitoring Metrics。
10. 可透過 Docker、Kubernetes/Helm 與 Linux Bare Metal/systemd 部署。
11. 完成 Dell ECS API Coverage Matrix 與實際 API Mapping Specification。

---

## Appendix A. 建議 PromQL

### Bucket Hard Quota 使用率

```promql
100 *
ecs_bucket_used_bytes
/
ecs_bucket_hard_quota_bytes
```

### Bucket Soft Quota 使用率

```promql
100 *
ecs_bucket_used_bytes
/
ecs_bucket_soft_quota_bytes
```

### Bucket Hard Quota 剩餘容量

```promql
ecs_bucket_hard_quota_bytes
-
ecs_bucket_used_bytes
```

### 超過 90% Hard Quota 的 Bucket

```promql
100 *
ecs_bucket_used_bytes
/
ecs_bucket_hard_quota_bytes
>= 90
```

### Cache 過期

```promql
ecs_exporter_cache_age_seconds > 600
```
