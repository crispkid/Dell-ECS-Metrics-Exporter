# Traceability

Active Change: ECS-008

ECS-004 已完成 mock/fixture 可驗證的產品 runtime，ECS-005 加入隔離 ECS CE
3.8.0.3 部分 Management API evidence，ECS-006 補完十項 runtime/component 缺口，
ECS-007 修正非空 Bucket live follow-up 揭露的 envelope 與 billing KB，並補正
batch request-body mapping。ECS-008 新增 Bare Metal、release/supply-chain、
NetworkPolicy、operations、live/E2E 與 target-scale fail-closed gates。
含 `pending` 的項目不得解讀為正式設備或整個版本範圍相容認證。

| Requirement ID | Change ID | Implementation area | Test evidence | Harness | Status |
|---|---|---|---|---|---|
| REQ-001 | ECS-002/ECS-004/ECS-005/ECS-006 | `internal/config`、`internal/ecs/client.go`、`cmd/ecs-exporter` | component auth lifecycle、per-cluster rate cancellation + CE live SYSTEM_MONITOR login/whoami/logout | `test`、`security`、live record | Implemented; expiry/reauth live pending |
| REQ-002 | ECS-004/ECS-005/ECS-006 | `internal/collector`、`internal/cache`、`internal/model` | initial jitter、single-flight、deep-copy/per-domain generation、atomic snapshots | `test`、`coverage`、live record | Implemented |
| REQ-003 | ECS-002/ECS-004/ECS-005/ECS-006 | `internal/ecs/parser.go`、node collectors、`internal/metrics` | disk allowlist、service enum、network bound/reset + CE Management live | `test`、live record | Component complete; production Flux/hardware pending |
| REQ-004 | ECS-002/ECS-004/ECS-005/ECS-006/ECS-007 | namespace/bucket collectors、pagination、nested/top-level quota/billing parser | envelope/ambiguity、known-size KB + corrected CE three-Bucket/four-object live refresh | `test`、live record | Partial-live/race passed；external vulnerability gate/formal Profile certification pending |
| REQ-005 | ECS-002/ECS-004/ECS-005/ECS-006 | Profile guard、Performance parser/cache/metrics | VDC/Namespace fixture-to-Prometheus mapping、conditional enable/deny + CE 503 behavior | `test`、live record | Component complete; production Flux sandbox pending |
| REQ-006 | ECS-002/ECS-004/ECS-006 | replication/recovery parser/collector/cache/metrics | guard + known/unknown status、lag、progress、多 link 維度 | `test` | Implemented for configured IDs; sandbox pending |
| REQ-007 | ECS-004/ECS-005 | `internal/httpapi/server.go` | component auth/query tests + CE-backed five Inventory collection smoke | `test`、live record | Implemented; reverse proxy live pending |
| REQ-008 | ECS-004/ECS-005/ECS-006 | `internal/health`、`internal/metrics`、HTTP health/version | maxStale cutoff/self preservation、response-size/skipped refresh + CE DEGRADED | `test`、`coverage`、live record | Implemented |
| REQ-009 | ECS-004/ECS-005/ECS-006/ECS-007 | ECS client、scheduler、pagination/enrichment | `{"id":[...]}` batch、plural envelope、404/405/501/missing fallback、500 negative tests + live success | `test`、live record | Partial-live batch passed；controlled 429/timeout and external vulnerability gate pending |
| REQ-010 | ECS-004/ECS-005/ECS-006/ECS-008 | strict config/Profile evidence、safe observer、security/release scripts、Inventory auth | conditional/resource/rate config、redaction、secret/release policy tests | `test`、`security`、`supply-chain` | Implementation complete; external scan pending |
| REQ-011 | ECS-003/ECS-004/ECS-008 | digest-pinned `Dockerfile`、Helm、Bare Metal、release workflow/scripts | deterministic archive、Helm/NetworkPolicy static、CI/release policy、synthetic scale | `build`、`deploy:check`、`ci:policy`、`release-check` | Mechanisms implemented; container/live K8s/deployed scale evidence pending |
| REQ-012 | ECS-002/ECS-003/ECS-004/ECS-005/ECS-006/ECS-007/ECS-008 | `internal/profile`、profiles/docs/fixtures、bootstrap/live certification | exact CE selection + partial-live 3.8.0.3 fixtures + exact-build fail-closed gate | `test`、`verify`、`integration`、`e2e` | Partial CE evidence；3.8.1.4 and all formal certifications pending |

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
| `profiles/`、`docs/ecs-api/`、`testdata/ecs/` | Four-version contract + redacted partial-live ECS 3.8.0.3 Bucket fixtures | Partial build evidence does not certify a Profile；`tested_builds=[]` |
| `docs/ecs-api/validation/ecs-ce-3.8.0.3-2026-07-25.md` | Redacted exact-build live Management API and corrected non-empty Bucket results | CE Flux、failure/formal appliance gates pending |

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
