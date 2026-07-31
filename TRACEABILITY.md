# Traceability

Active Change: ECS-012

ECS-004 已完成 mock/fixture 可驗證的產品 runtime，ECS-005 加入隔離 ECS CE
3.8.0.3 部分 Management API evidence，ECS-006 補完十項 runtime/component 缺口，
ECS-007 修正非空 Bucket live follow-up 揭露的 envelope 與 billing KB，並補正
batch request-body mapping。ECS-008 新增 Bare Metal、release/supply-chain、
NetworkPolicy、operations、live/E2E 與 target-scale fail-closed gates。
ECS-009 依實體 ECS 3.8.1.1 修正 split Flux query、Performance schema 與 TLS
explicit-disable contract。
ECS-010 新增官方文件 reconciliation、四 Profile production-path fixture replay 與
去識別化 exact-build read-only Probe；這些證據仍不自動提升 Profile certification。
ECS-011 依使用者決策，將任一目標版本 qualifying live evidence 繼承為四個 Profile
的共用功能驗證；exact-build facts 與版本差異仍分開管理。
ECS-012 以 `v1.0.0-rc.1` 建立明確 RC/stable 發布分流、版本化 release notes 與
Pre-release publication；stable external gates 不因 RC 而降低。

| Requirement ID | Change ID | Implementation area | Test evidence | Harness | Status |
|---|---|---|---|---|---|
| REQ-001 | ECS-002/ECS-004/ECS-005/ECS-006/ECS-011 | `internal/config`、`internal/ecs/client.go`、`cmd/ecs-exporter` | component auth lifecycle、per-cluster rate cancellation + CE live SYSTEM_MONITOR login/whoami/logout | `test`、`security`、live record | Login/token/logout validated-shared; expiry/reauth operation gate pending |
| REQ-002 | ECS-004/ECS-005/ECS-006 | `internal/collector`、`internal/cache`、`internal/model` | initial jitter、single-flight、deep-copy/per-domain generation、atomic snapshots | `test`、`coverage`、live record | Implemented |
| REQ-003 | ECS-002/ECS-004/ECS-005/ECS-006/ECS-009/ECS-011 | `internal/ecs/parser.go`、split node collectors、`internal/metrics` | disk allowlist、service enum、network bound/reset、split-query atomic tests + ECS 3.8.1.1 CPU/Memory/Network live evidence | `test`、live record | Inventory/health/CPU/Memory/Network validated-shared; Disk/service pending |
| REQ-004 | ECS-002/ECS-004/ECS-005/ECS-006/ECS-007/ECS-011 | namespace/bucket collectors、pagination、nested/top-level quota/billing parser | envelope/ambiguity、known-size KB + corrected CE three-Bucket/four-object live refresh | `test`、live record | Namespace/Bucket inventory/quota/billing validated-shared |
| REQ-005 | ECS-002/ECS-004/ECS-005/ECS-006/ECS-009/ECS-011 | Profile guard、split Performance parser/cache/metrics | official-shape VDC/Namespace fixture mapping、atomic failure、conditional enable/deny + ECS 3.8.1.1 live evidence | `test`、live record | Supported VDC/Namespace Performance validated-shared; interval range remains version-specific |
| REQ-006 | ECS-002/ECS-004/ECS-006 | replication/recovery parser/collector/cache/metrics | guard + known/unknown status、lag、progress、多 link 維度 | `test` | Implemented for configured IDs; sandbox pending |
| REQ-007 | ECS-004/ECS-005 | `internal/httpapi/server.go` | component auth/query tests + CE-backed five Inventory collection smoke | `test`、live record | Implemented; reverse proxy live pending |
| REQ-008 | ECS-004/ECS-005/ECS-006 | `internal/health`、`internal/metrics`、HTTP health/version | maxStale cutoff/self preservation、response-size/skipped refresh + CE DEGRADED | `test`、`coverage`、live record | Implemented |
| REQ-009 | ECS-004/ECS-005/ECS-006/ECS-007 | ECS client、scheduler、pagination/enrichment | `{"id":[...]}` batch、plural envelope、404/405/501/missing fallback、500 negative tests + live success | `test`、live record | Partial-live batch passed；controlled 429/timeout and external vulnerability gate pending |
| REQ-010 | ECS-004/ECS-005/ECS-006/ECS-008/ECS-009/ECS-010/ECS-012 | strict config/Profile evidence、safe observer、TLS warning、redacted Flux Probe、security/release scripts、Inventory auth | production TLS-disable warning、Probe serialization/setup/error redaction、secret/release/RC policy tests | `test`、`security`、`supply-chain` | Implementation complete; TLS-disabled deployments retain identity-verification risk |
| REQ-011 | ECS-003/ECS-004/ECS-008/ECS-012 | digest-pinned `Dockerfile`、Helm、Bare Metal、RC/stable release workflow/scripts | deterministic archive、Helm/NetworkPolicy static、tag classification、CI/release policy、synthetic scale | `build`、`deploy:check`、`ci:policy`、tag workflow | RC mechanism complete pending publication; stable container/live K8s/deployed scale evidence remains |
| REQ-012 | ECS-002/ECS-003/ECS-004/ECS-005/ECS-006/ECS-007/ECS-008/ECS-009/ECS-010/ECS-011 | `internal/profile`、profiles/docs/fixtures、`internal/fluxprobe`、bootstrap/live certification | shared capability evidence + four-profile replay/redaction + CE/appliance live records | `test`、`verify`、`integration`、`e2e` | Common functions validated-shared; version-specific differences and delivery gates remain |

## ECS-004 Artifact Trace

| Artifact | Contract/evidence | Remaining limitation |
|---|---|---|
| `internal/config/`、`config.example.yaml` | Strict runtime configuration and secret references | Secret-manager integration is deployment-specific |
| `internal/ecs/` | Authenticated resilient client and mapping parsers | Real ECS schema/unit/token lifecycle not yet observed |
| `internal/collector/`、`internal/cache/`、`internal/model/` | Scheduled atomic domain snapshots | Performance at target scale not yet measured |
| `internal/metrics/` | Prometheus-parseable domain/self metrics | Conditional/unavailable fields intentionally omitted |
| `internal/httpapi/`、`internal/health/` | Authenticated inventory, RFC 9457, health/version | Reverse-proxy deployment topology needs environment test |
| `Dockerfile`、Helm Chart、`deploy/bare-metal/` | Non-root OCI/Kubernetes and hardened systemd deployment contracts | Container/live Kubernetes/systemd host verification not run locally |
| `.github/workflows/`、release scripts | SHA-pinned CI, protected exact-build/release, SBOM/sign/provenance contracts | No Git remote/tag/published artifacts or external scanner evidence |
| `profiles/`、`docs/ecs-api/`、`testdata/ecs/` | Four-version contract + machine-readable shared feature evidence | Exact-build/full Profile facts remain separate；`tested_builds=[]` |
| `cmd/ecs-flux-probe/`、`internal/fluxprobe/` | Same-path read-only checks with redacted build/Profile/policy/count report | Exact reports are optional version-specific regression evidence under ECS-011 |
| `docs/ecs-api/validation/ecs-ce-3.8.0.3-2026-07-25.md` | Redacted exact-build live Management API and corrected non-empty Bucket results | CE Flux、failure/formal appliance gates pending |

ECS-010 deterministic verification passed four-profile replay/redaction, local synthetic HTTPS
client calibration, a fresh race suite and every required Harness stage at 84.7% coverage. The
security stage found no Go vulnerabilities. Release build inspection confirmed both binaries in
the Linux archive. Deployment used strict static fallbacks for unavailable local systemd,
promtool and kubeconform; no live runtime claim is made.

## Evidence Boundary

`./HARNESS/harness.sh verify` 已在 ECS-006 最終工作樹通過九個 required stages，
coverage 84.4%，完整 race test、Profile build、`govulncheck`、Helm 與 CI policy
亦通過。ECS-007 已取得 ECS CE 3.8.0.3 top-level quota/single billing、batch
request-body matrix 與 billing KB=1024 的 partial live evidence，且
`testdata/ecs/ecs-3.8.0.3-live/` 只保存去識別化必要欄位。修正後 Exporter batch
HTTP 200、Bucket collector 連續成功、三 Bucket Inventory 與 Cluster/Namespace
aggregate 已通過；race、lint/format/typecheck/test、84.5% coverage、build、
CI policy 與 deploy check 亦通過。完整 Harness 仍因 execution policy 未授權
`govulncheck` 存取外部漏洞資料庫而失敗。這些證據仍不證明
ECS 3.6.x/3.7.x/整個 3.8.0.x/3.8.1.x、正式版 Flux、target-scale performance、
container startup、Kubernetes admission 或 production readiness。

ECS-008 的 repository-controlled gates 已完成：unit/component 與 fresh race、
84.3% coverage、build/Profile、governance、CI policy、Helm/Bare Metal static、
deterministic archives/checksums 與 synthetic 10/100/10,000 performance 通過。
`metricscheck` 使用明確 Prometheus legacy name validation 並有 success/malformed/
missing-family regression，避免 release live/E2E gate 因未初始化 parser 而 panic。
完整 Harness 仍在外部 `vuln.go.dev` 查詢失敗；Git tracking/remote、scanner、
Docker、kubeconform、systemd/live Kubernetes、ECS 3.8.1.4、deployed scale、
published signatures/attestations 與具名 reviewer 皆由 fail-closed gate 阻擋。
