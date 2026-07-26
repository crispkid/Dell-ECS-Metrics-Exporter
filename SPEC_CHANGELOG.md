# Specification Changelog

Active Change: ECS-008

| Date | Change ID | Spec section | Reason | Compatibility/migration | Test impact | Approver |
|---|---|---|---|---|---|---|
| 2026-07-26 | ECS-008 | REQ-010–REQ-012、sections 19–21 production delivery | 使用者要求完成 Production 前工作並增加 Bare Metal | Backward-compatible 新增 systemd deployment、bounded NetworkPolicy values、release/SBOM/sign/provenance、live/E2E/performance fail-closed gates；不修改 metrics/Inventory schema 或 Profile certification | 新增 archive determinism、release policy、container/Kubernetes、synthetic/deployed scale、exact-build live/E2E 與 operations checks；外部 evidence 缺少仍阻擋 | 使用者授權實作；沒有 production Gate 4 或具名 reviewer 核准 |
| 2026-07-26 | ECS-007 | REQ-004、REQ-009、REQ-012 ECS 3.8.0.3 Bucket compatibility | 非空 Bucket live follow-up 發現 top-level quota/single billing、batch request-body差異與 billing KB=1024 | Backward-compatible 保留 nested envelope；新增 top-level/plural envelope、`{"id":[...]}` batch body 與 narrow fallback；只修正 billing KB | 新增 envelope ambiguity、request matrix、non-empty batch/fallback/error-code、known-size unit regression；修復後 partial-live/race 通過，Harness 的外部漏洞資料庫存取未獲執行政策授權 | 使用者要求修復實測失敗項目；沒有 production Gate 4 或具名 reviewer 核准 |
| 2026-07-26 | ECS-006 | REQ-002–REQ-005、REQ-008–REQ-010、REQ-012 runtime completeness | 使用者要求完成規格稽核列出的十項未實作／不完整項目 | Backward-compatible 新增 VDC/Namespace Gauge、node service Gauge、自我 telemetry 與 optional config；conditional/disk 預設關閉，stale domain 超時停止輸出 | 新增 Performance、capability、disk/service、inventory、stale、rate/jitter、network、batch billing、TB/response-size、per-domain refresh tests；live certification 仍 pending | 使用者授權完成十項；沒有 production Gate 4 或具名 reviewer 核准 |
| 2026-07-25 | ECS-005 | REQ-001–REQ-005、REQ-008–REQ-010、REQ-012 live compatibility | 使用者授權在隔離 ESXi 部署 ECS CE 3.8.0.3 並繼續 API 相容性驗證 | Backward-compatible 支援 top-level capacity/quota、HAL Node health、Namespace-scoped Bucket；Node Management/Resources 分離，Flux failure 由 DOWN 改為有明確 error 的 DEGRADED | 新增 live envelope regression、cross-Namespace/partial resource tests、race/full Harness 與 redacted exact-build smoke；CE Flux/正式認證仍 pending | 使用者授權隔離 CE 驗證；沒有 production Gate 4 或具名 reviewer 核准 |
| 2026-07-25 | ECS-004 | REQ-001–REQ-012 runtime、config、metrics/API、deployment/CI | 使用者要求完成全部開發工作；建立可部署且可由 mock/fixture 驗證的 V1 runtime | 新增 `config.yaml` public contract、domain metrics/Inventory API 與 Helm values；保留四 Profile 未認證狀態及 unavailable Bucket performance 行為 | 新增 auth/client/parser/collector/cache/health/metrics/API tests、race/86%+ coverage、Helm/CI policy gates；真機/效能仍外部執行 | 使用者授權完成開發；沒有 production Gate 4 或具名 reviewer 核准 |
| 2026-07-25 | ECS-003 | Implementation baseline、Health bootstrap、Profile selection、deployment verification | 固定 Go 1.26.5 並建立第一個可編譯、可測試的相容性核心與開發部署骨架 | 尚未新增 ECS domain metric/config/Inventory public contract；bootstrap readiness 明確維持 HTTP 503／`DOWN`，module path 在 remote 決定前暫定 | 新增 version/Profile/mixed selection、fixture integrity、HTTP bootstrap tests；Harness 啟用 8 個 required stages | 使用者要求開始準備開發；未記錄正式 Gate 2/Gate 4 核准 |
| 2026-07-25 | ECS-002 | Compatibility、API Coverage/Mapping、Node/Bucket metrics、Profile/fixture contracts | 建立 ECS 3.6、3.7、3.8.0、3.8.1 四個版本 Profile、mapping 與 synthetic fixtures | 新增 `interface` label 至 node network counters；四個目標 Profile 明確不輸出未證實的 Bucket performance metrics；3.7/3.8.0 停用 Flux interval rates | 新增 Profile/manifest/schema/fixture contract cases、Flux range guard、Host Header 與 version matrix；產品 tests 仍待 Go 實作 | 尚未正式核准；責任角色為 Project Maintainer |
| 2026-07-25 | ECS-001 | V1.0 全部章節、Requirement Index、Acceptance Criteria 與 API Mapping contract | 將使用者提供的產品與整合附件納入儲存庫，建立可追溯的 greenfield baseline | 新專案，沒有既有 runtime、metric、API、config 或資料需要 migration；未來變更受 Semantic Versioning 約束 | 建立 unit/component/integration/performance/security/deployment test design；目前只有文件與 Harness evidence | 尚未正式核准；責任角色為 Project Maintainer |

## ECS-008 Notes

- Linux amd64/arm64 release archive 包含 dedicated `ecs-exporter` account、
  systemd hardening、外部 secrets、config/Profile preflight、upgrade/verify/uninstall；
  upgrade restart 失敗時還原前一版 binary、Profiles 與 systemd unit。
- Docker build base image 與 GitHub actions 以 immutable digest/full SHA 固定；
  release workflow 產生 multi-arch image、archives、Helm、checksums、SBOM、
  keyless Sigstore signature 與 GitHub attestations。
- Helm NetworkPolicy 可設定 ingress selectors 與 DNS/ECS egress；production example
  使用文件用 `192.0.2.0/24` fail closed，必須由 operator 替換；預設維持單副本，
  避免在未做負載驗證前倍增 ECS API polling。Chart-managed ConfigMap 使用 checksum
  rollout，Secret volume 預設 `0440`。
- `release-check.sh` 串接 deterministic/race、scanner、container、Kubernetes schema、
  synthetic/deployed target-scale、live ECS 與 deployed E2E；缺 prerequisite exit 3。
- Protected release 同時要求正式設備 exact `3.8.1.4` 為 `UP`，以及測試 ECS CE
  exact `3.8.0.3` 通過 Management suite；CE `DEGRADED` 僅允許
  `node-resources` collector error。
- Production Runbook、Prometheus alerts、release checklist 與 artifact verification
  commands 已建立。真實 ECS 3.8.1.4、Docker daemon、live Kubernetes、target-scale、
  external vulnerability DB、Git remote/tag/release 及具名 reviewer evidence 尚未產生，
  因此目前 lifecycle 仍是 pre-release。

## ECS-007 Notes

- ECS CE exact build `3.8.0.3.138685.3a0a9b6bf3a` 的 follow-up 建立三個 Bucket，
  四個物件合計 10,000,000 bytes；configured quota 為 soft 1 GB/hard 2 GB。
- Bucket quota 與單筆 billing 使用 top-level fields；runtime 同時保留 inherited
  nested envelope，雙 envelope 視為 ambiguous error。
- Namespace batch billing 對 no entity、`{}`、`[]`、`null` 分別回 HTTP 415、
  HTTP 200 + `bucket_billing_infos: []`、HTTP 400/code1013、HTTP 500/code999。
  內建 `BucketListParam.class` 與 live probe 確認 non-empty body 是
  `{"id":[bucket names...]}`，並回三筆 plural `bucket_billing_infos`。
- Fallback 僅適用 HTTP 404/405/501 或 batch 缺少 requested item；
  generic 500 與 500/code999 不 fallback。
- Bucket billing 為 `6835.9375`、`2929.6875`、`0 KB`，Namespace 總計
  `9765.625 KB`；乘 1024 精確得到 10,000,000 bytes。
- Billing MB/GB/TB 及 capacity/quota GB 沒有新的 multiplier evidence，維持既有
  decimal mapping。Profile `tested_builds=[]`、`sandbox_certified=false` 不變。
- 修復後 Exporter batch HTTP 200，Bucket collector 至少連續成功兩次；Inventory、
  Cluster 與 Namespace aggregate 符合三 Bucket／四 object／10,000,000 bytes。
- Race suite 通過；Harness 的 lint/format/typecheck/test/84.5% coverage/build/CI policy
  與獨立 deploy check 通過，但 required `govulncheck` 因外部資料庫連線未獲執行政策
  授權而未完成。Profile 仍未認證。

## ECS-006 Notes

- Performance Collector 現在解析並原子快取 VDC/Namespace Flux values，公開
  throughput、latency 與 request/status-window Gauge；Bucket scope 仍 unavailable。
- Conditional capability 需 per-cluster 明列，mixed interval rates 與 unavailable
  capability 不可覆寫。Recovery 不再繞過 guard。
- Node disk 使用 filesystem allowlist；network 受 allowlist/max/bond policy 限制；
  service/process 只解析既有 Node health response。
- Bucket billing 優先 Namespace batch POST，unsupported/missing item fallback 單筆；
  billing sample time 不再覆蓋 inventory timestamps，支援 decimal TB。
- Per-cluster rate limiter、initial jitter、API response-size histogram、`maxStale`
  domain cutoff 與 per-domain refresh generation 已實作。
- Profile schema 可表達 future sandbox certification，但現有四個 Profile 仍未認證。

## ECS-005 Notes

- 隔離 ECS CE 回報 exact build `3.8.0.3.138685.3a0a9b6bf3a`，Exporter 正確選擇
  `ecs-3.8.0` 且拒絕把部分 CE evidence 升級為 `tested_builds`。
- Live mapping 顯示 Capacity/Namespace quota 為 top-level、Node health 使用
  `_embedded._instances`，Bucket list 必須帶 `namespace`。
- Runtime 同時保留既有 nested envelope，雙 envelope 視為 ambiguous error；Bucket
  先列 Namespace，再逐 Namespace 執行 bounded pagination。
- Node Management 與 Flux Resources 分離。CE Flux HTTP 503 保留 API/collector
  error、readiness 為 HTTP 200 DEGRADED，Node inventory/health 與其他 domain 可用。
- 去識別化 evidence 位於
  `docs/ecs-api/validation/ecs-ce-3.8.0.3-2026-07-25.md`；raw responses、endpoint、
  credential 與 token 未進入 repository。

## ECS-004 Notes

- Runtime 使用 per-cluster token/TLS client、first-401 reauthentication、bounded retry/
  concurrency/circuit 與 cross-origin redirect rejection；logs 不保存 URL、credential
  或 response body。
- 所有 collectors 只在完整成功後替換 deep-copy cache；Bucket marker page 或任一
  quota/billing enrichment 失敗時保留上一份快照。
- Prometheus 與 Inventory 只讀 cache；Inventory 預設 Bearer token，並支援受信任
  proxy 模式與 token file rotation。
- ECS 3.7/3.8.0 interval rate、未驗證的 3.8.1 conditional performance、Bucket
  performance 與沒有 allowlist 的 node disk 維持停用。
- Parser 拒絕 unit conversion/pending-byte overflow 與重複 health/Flux 欄位；
  recovery metric 保留 source/target VDC，避免多 link label 衝突。
- README 擴充為本機、Docker、Helm 的端到端安裝/設定/驗證/維運指南；
  `config.example.yaml` 改用 Git-ignored secret files，CLI help 正常輸出後以成功狀態結束。
- Helm 改為掛載完整 config 與既有 Secret；GitHub Actions 使用 read-only permission
  及 full-SHA actions。Release SBOM/provenance/signing 仍是首次發布前工作。

## ECS-003 Notes

- Go language baseline 固定為 1.26.0，toolchain/build 1.26.5；
  `govulncheck` 固定為 v1.6.0。
- `internal/profile` 嚴格拒絕未知 JSON 欄位、重複/重疊 Profile、無效四段版本、
  range 外 documented/tested build 與未知 capability。
- mixed-version resolution 使用保守交集，並無條件把 `flux_interval_rates` 降為
  `unavailable`；3.8.2 upper boundary 仍拒絕。
- 最小 HTTP 僅提供 liveness、明確 unavailable readiness、build/Profile evidence 與
  bootstrap self metrics，不包含 ECS client 或 collector。
- Dockerfile/Helm Chart 是開發骨架；container startup、base image digest、
  Kubernetes schema/live apply 與 image scan 仍待後續 evidence。

## ECS-002 Notes

- 使用者選定驗證目標為 ECS 3.6.x、3.7.x、3.8.0.x、3.8.1.x。
- Profile、mapping 與 fixtures 是 documentation/component contracts，不是 sandbox
  certification；四個 Profile 的 `tested_builds` 均保持空白。
- ECS 3.6 API/Monitoring 官方文件已檢視。ECS 3.7/3.8 REST API ZIP 需要 Dell
  Support 登入，因此相關 Management mapping 明確標為 `candidate-inherited`。
- Dell KB 000211906 的 Flux range 問題與 KB 000205031 的 Host Header 行為已納入
  capability 及 failure fixtures。
- Node network counters 改由 Flux `net.bytes_recv/bytes_sent` 取得並保留
  `interface` label；不再依賴 ECS 3.6 已移除的 Dashboard performance fields。

## ECS-001 Notes

- 使用者於 2026-07-25 確認採 Go、GitHub Actions、Docker、Kubernetes/Helm 與角色式
  ownership 的建議方向。
- 目標 Dell ECS version 仍未確定；真實 URI/schema mapping 與 adapter 實作被明確
  阻擋。
- 此 changelog 不將方向確認記錄成完整 specification 或 implementation approval；
  Gate 狀態以 `DEVELOPMENT_PLAN.md` 為準。
