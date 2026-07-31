# Project Context

本文件是 `AGENTS.md` 共用基線的專案專屬補充。ECS-004 已完成可由 mock/fixture
驗證的 V1 runtime，ECS-005 新增 ECS CE 3.8.0.3 部分 Management API evidence，
ECS-006 補完十項 runtime、mapping 與 telemetry 缺口，ECS-007 修正非空 Bucket
實測揭露的 envelope、fallback 與 billing KB 相容性，ECS-008 新增 Bare Metal 與
fail-closed production delivery gates；ECS CE 3.8.1.4 另完成 Management-backed
partial-live 驗證。ECS-009 再依實體 ECS 3.8.1.1 修正 split Node/Performance
Flux mapping，並允許企業自簽憑證明確停用 TLS verification 且輸出 WARN。
ECS-010 新增重用 production path 的去識別化唯讀 Flux Probe 與四 Profile replay，
供沒有本地實體版本時向客戶、Dell 或合作夥伴收集 exact-build 候選證據。
ECS-011 改採 `shared-live-any-target-version`，把任一目標版本已真實驗證的共用功能
標示為四個 Profile 全部驗證完成，同時保留版本差異與 exact-build 執行事實。
ECS-012 將第一個可公開評估版本固定為 `v1.0.0-rc.1`，新增 RC／stable workflow
分流與明確 Pre-release notes；完整 3.8.1.x range/multi-VDC、deployed target-scale、
exact-build E2E 與具名 reviewer evidence 仍是 stable `v1.0.0` 的門檻。
不得把 fixture/component、CE 部分 evidence 或 workflow 定義誤報為 production
approval 或整個 Profile 真機認證。

## Identity

- Project name: `Dell ECS Metrics Exporter`
- Purpose: 定期讀取一或多個 Dell ECS Cluster 的 Management／Monitoring API，
  將資料轉換為 Prometheus Metrics，並提供唯讀 Inventory REST API、Health Check
  與 Exporter 自我監控能力；歷史資料與視覺化由 Prometheus、Grafana 與
  Alertmanager 負責。
- Lifecycle: `release-candidate`（target `v1.0.0-rc.1`; stable production certification
  pending）
- Owners: `Project Maintainer` 對產品與技術決策負責；`Security Reviewer` 對認證、
  機密資料、CI 與供應鏈變更負責。尚未指定個人姓名。
- Shared baseline version: `1.1.0`
- Shared baseline source/owner: 本儲存庫的 `AGENTS.md` 與 `HARNESS/`；
  維護角色為 `Project Maintainer`。
- License: Apache License 2.0，詳見 `LICENSE`。

## Sources of Truth

- Product behavior: `SPECIFICATION.md`
- Architecture: `SPECIFICATION.md` 第 4、5、8、14、19、20 節；架構決策紀錄預定放在
  `docs/adr/`，該路徑會在第一個 ADR 建立時加入。
- API/data contracts: `SPECIFICATION.md` 第 9、11、12、13、16、22、23 節、
  `DELL_ECS_API_MAPPING.md`、`profiles/`、`docs/ecs-api/` 與 `testdata/ecs/`
- Deployment and operations: `SPECIFICATION.md` 第 17 至 20 節、`Dockerfile`、
  `charts/dell-ecs-metrics-exporter/`、`deploy/bare-metal/`、`deploy/local/`、
  `docs/PRODUCTION_RUNBOOK.md` 與 `docs/RELEASE_CHECKLIST.md`。
- Design system and content rules: 不適用；V1.0 不包含圖形使用者介面或 Grafana
  Dashboard。

發生衝突時，已核准的 `SPECIFICATION.md` 與相容性政策優先於實作；特定 ECS
版本的已驗證 API 行為以 `DELL_ECS_API_MAPPING.md` 為準。測試是行為證據，不得
自行改寫產品契約。若實作或測試揭露規格錯誤，先以新的變更 ID 修訂規格與
traceability，再修改行為。Dell 官方目標版本 API 文件優先於未驗證的推測或範例。

## Component Map

下列路徑列出 ECS-004 建立、ECS-005/ECS-007 live evidence 修正並由 ECS-006 補完的
V1.0 元件。

| Component | Path | Responsibility | Owner | Runtime |
|---|---|---|---|---|
| Exporter entry point | `cmd/ecs-exporter/` | Config/Profile validation、runtime wiring、HTTP/scheduler lifecycle 與 graceful logout | Project Maintainer | Go 1.26.5 |
| Flux compatibility probe | `cmd/ecs-flux-probe/`、`internal/fluxprobe/` | Read-only exact-build mapping probe and redacted count/policy report | Project Maintainer | Go 1.26.5 |
| ECS compatibility contracts | `profiles/`、`docs/ecs-api/`、`testdata/ecs/` | 版本選擇、capability、API mapping 與合成 fixtures | Project Maintainer | JSON、Markdown |
| ECS profile runtime | `internal/profile/` | 四段版本解析、Profile 嚴格載入、range/mixed capability resolution | Project Maintainer | Go 1.26.5 |
| Runtime configuration | `internal/config/`、`config.example.yaml` | Strict YAML/env/secret/TLS/interval/rate-limit/capability/resource-policy validation | Project Maintainer | Go、YAML |
| ECS adapters | `internal/ecs/` | Per-cluster auth/TLS HTTP、rate limit/retry/circuit、safe errors 與 Management/Flux parsing | Project Maintainer | Go 1.26.5 |
| Collectors and scheduler | `internal/collector/` | 各領域排程、initial jitter、capability guard、pagination/batch enrichment 與 single-flight refresh | Project Maintainer | Go 1.26.5 |
| Cache and domain model | `internal/cache/`、`internal/model/` | Deep-copy thread-safe snapshots、per-domain generation/state 與統一資料模型 | Project Maintainer | Go 1.26.5 |
| Prometheus mapping | `internal/metrics/` | Domain/self metric family、type、unit、bounded labels 與 exposition | Project Maintainer | Go + Prometheus Go client |
| Inventory and health API | `internal/httpapi/` | Liveness/readiness/version、authenticated Inventory、pagination/filter/sort 與 RFC 9457 | Project Maintainer | Go 1.26.5 |
| Deployment | `Dockerfile`、Helm Chart、`deploy/bare-metal/`、`deploy/local/` | Digest-pinned image、Kubernetes policy、hardened systemd install/upgrade/verify，以及本機 Prometheus/Grafana integration test stack | Project Maintainer | OCI、Kubernetes、Helm、systemd、Docker Compose |
| Delivery and operations | release/performance/live scripts、`.github/workflows/`、`docs/`、`deploy/prometheus/` | Release/SBOM/sign/provenance、exact-build/E2E/scale gates、alerts/runbook | Project Maintainer + Security Reviewer | Bash、Go、GitHub Actions、Prometheus |
| CI | `.github/workflows/ci.yml`、`scripts/ci-policy-check.sh` | SHA-pinned read-only PR verification、Harness 與 OCI build | Project Maintainer | GitHub Actions |
| Portable harness | `HARNESS/` | 專案檢查、證據與治理契約 | Project Maintainer | Bash 3.2+ |

預定的產生檔、測試輸出與建置產物分別位於 `test-results/`、`coverage/` 與
`dist/`，不得提交。V1.0 不規劃 vendored dependencies 或資料庫 migration。
目前文件衍生的合成 fixture 位於 `testdata/ecs/`；ECS-007 的去識別化 partial-live
fixture 位於 `testdata/ecs/ecs-3.8.0.3-live/`，ECS CE 3.8.1.4 的 partial-live fixture
位於 `testdata/ecs/ecs-3.8.1.4-live/`。Partial-live fixture 只證明該 exact build
的已列 response shape/unit，不代表整個 Profile 或修正後 Exporter 已認證。
未來 package-specific fixture 可放在各 package 的 `testdata/`，但必須引用相同
mapping ID 與 evidence classification。

## Supported Environments and Toolchain

- Developer operating systems: macOS 與 Linux；Windows 僅支援 WSL2。
- Runtime versions: language baseline 為 Go 1.26.0，開發與建置 toolchain 固定為
  Go 1.26.5；`go.mod`、Dockerfile、Harness environment check 與本文件同步。
- Package managers: Go Modules；Helm dependency management 僅在 Chart 引入依賴
  時使用。版本來源必須是提交的 module、toolchain 與 Chart metadata。
- Required local tools: Bash 3.2+、Git、Go 1.26.5、Helm 3+、ripgrep。Docker/相容
  OCI builder 與 Kubernetes schema scanner 在執行 image/live deployment gate 時
  需要；目前安全 scanner `govulncheck` 由 `go.mod` tool dependency 固定。
- Supported clients/browsers/devices: Prometheus-compatible scraper、HTTP/JSON client
  與 Kubernetes；不提供瀏覽器 UI。

## Harness Contract

- Required stages: ECS-008 deterministic handoff 沿用
  `lint,format,typecheck,test,coverage,build,security,deploy,ci-policy`。具備隔離測試 ECS
  與 deployment prerequisites 後，production `release-check.sh` 另外要求
  `supply-chain`、`integration`、`e2e`、container、Kubernetes schema 與效能 gate。
- Canonical narrow commands: `./HARNESS/harness.sh <lint|format:check|typecheck|test|coverage|build|security|deploy:check|ci:policy>`；
  實際命令由 `HARNESS/config.env` 指向 `scripts/` 中的固定入口。
- Deterministic handoff command: `./HARNESS/harness.sh verify`
- Coverage threshold and scope: 最低 80%，衡量應用程式 Go packages；產生檔、
  僅含 wiring 的 entry point 與測試 fixture 可依 coverage 設定排除，不得藉排除
  隱藏核心邏輯。
- Integration prerequisites: 去識別化 Mock ECS API 用於 component tests；宣稱真實
  integration 通過時必須使用隔離、非 production 的目標 ECS 版本環境、測試帳號與
  可清理資料。Mock-only 測試不得標示為真實 integration。
- E2E prerequisites: 已部署的測試 Exporter、隔離 ECS、Prometheus scraper、唯讀
  Inventory API credential、固定測試資料與完成後清理程序。
- Security gate: 執行 credential-pattern scan、module checksum 與 `govulncheck v1.6.0`。
  Container/image、license 與 deployment policy scanner 仍是 release 前阻擋項目；
  Critical/High 弱點預設阻擋，例外必須有風險、到期日與 Security Reviewer 核准。
- CI policy gate: `scripts/ci-policy-check.sh` 驗證 read-only permissions、禁用
  `pull_request_target`、完整 commit SHA pinning 與不可信 event interpolation。
- Supply-chain gate: `scripts/supply-chain-check.sh` 產生 deterministic release、
  checksums、SPDX/CycloneDX SBOM 並以 Grype 阻擋 High/Critical；protected release
  workflow 另產生 keyless Sigstore signature 與 GitHub provenance/attestation。
  缺工具或實際 publish evidence 時不得宣稱通過。
- Deployment gate: 驗證 Helm lint/template、必要 resource kinds、probes、
  resource limits、non-root/read-only security context、existing Secret reference、
  ingress/egress NetworkPolicy，且 Chart 不產生 Secret；production gate 要求
  kubeconform。Bare Metal 另提供 systemd preflight/verify。Digest 已固定；container
  startup、live Kubernetes/systemd host evidence 仍待外部 runtime。
- Evidence directory and retention: `test-results/harness/`；本機輸出不提交。CI 預設
  保留 30 天，security/SBOM/provenance evidence 預設保留 90 天，並先執行 redaction。

以上規格與 `HARNESS/config.env` 同步；required stage 不得以 skip 取代成功證據。

## Agent Evaluation Contract

- Evaluation dataset: 尚未採用；未來位置為 `HARNESS/evals/`。
- Regression runner: 尚未配置；建立穩定的 agent workflow 後才可設定。
- Capability runner: 尚未配置，且不是 V1.0 release gate。
- Trials and reliability metrics: 採用時預設每 case 3 次，回報 pass@1 與一致性；
  正式門檻必須由 Project Maintainer 明確核准。
- Graders: 優先使用 deterministic repository state 與 tests；主觀文件品質才使用
  human-calibrated rubric。
- Transcript/outcome retention: 採用時存於受控 CI artifact，先遮罩 endpoint、
  username、token、response body 與個人資料，預設保留 30 天。
- Independent evaluator requirement: Security、authentication、metric 相容性或
  deployment 權限的高風險變更，需由未實作該變更的 Project Maintainer 或
  Security Reviewer 複核；一般變更不要求獨立 agent evaluator。

## Environments and External Dependencies

| Environment/service | Purpose | Configuration source | Data policy | Local substitute |
|---|---|---|---|---|
| Dell ECS Cluster | Management／Monitoring API 資料來源 | YAML、環境變數與 Secret file reference | Internal/Confidential；不得保存 production response | 合成資料的 Mock ECS API |
| Prometheus | 擷取 `/metrics` 並驗證 exposition | Prometheus scrape config | Internal telemetry | 本機 Prometheus 或 parser |
| Kubernetes | 正式部署、Secret、probe 與 NetworkPolicy | Helm values、ConfigMap、Secret reference | Internal | kind、k3d 或等效隔離叢集 |
| GitHub Actions | PR/push、exact-build 與 protected release gate | `.github/workflows/` | Internal build/redacted evidence | 本機 Harness/scripts |
| OCI registry | Signed multi-arch image 與 Helm OCI 發布 | GitHub OIDC、GHCR、protected release environment | Internal/public release artifacts | 本機 OCI image store |

- Harness network policy: `declared`
- Stages permitted to use network: `integration,e2e,security,deploy,ci-policy,`
  `supply-chain,agent-regression,agent-capability`
- Per-command timeout: Harness 外層 timeout 目前停用，因支援的 macOS 環境未保證
  GNU `timeout`；Go test/typecheck 內建 2 分鐘 timeout，HTTP server 另設
  read-header/read/write/idle/shutdown timeout。其他長時間 stage 必須在各 script
  內加入等效 timeout 後才能成為 release gate。
- Offline/frozen dependency commands: 已提交 `go.mod`/`go.sum`，application 與
  security-tool dependencies 均已固定。CI 必須使用這些 checksum，不得在 release
  job 隱式升級依賴。

## Security and Data

- Data classification: 原始規格與一般程式碼為 internal；credential、token、cluster
  endpoint、未遮罩 inventory/response 與 production logs 為 confidential 或
  restricted。
- Authentication and authorization authority: ECS Authentication Manager 管理每個
  Cluster 的 ECS token；Inventory API 預設由 Bearer Token 或企業 reverse proxy
  驗證；Kubernetes/CI identity 由部署平台管理。
- Secret-management mechanism: Kubernetes Secret、企業 Secret Manager、環境變數或
  Secret file reference；不得提交、寫入 image、log、metric label 或測試證據。
- Logging and redaction rules: 依 `SPECIFICATION.md` 第 7、15、18 節；Password、
  Token、Cookie、Authorization header、敏感 query、完整 URL 與敏感 response body
  必須遮罩。
- Dependency and vulnerability policy: 依必要性、維護狀態、授權與供應鏈風險審查；
  Critical/High 預設阻擋，例外需有 owner、理由、補救措施與到期日。
- Backup, retention, deletion, and recovery: Exporter 不保存歷史資料且 cache 僅在
  memory；不需產品資料備份。設定與部署宣告由 Git/Secret 管理系統恢復，敏感測試
  artifact 依 retention 到期刪除。

## Compatibility and Change Management

- Public interfaces: `/metrics` metric names/types/labels、`/health`、
  `/api/v1/*` JSON schema 與 error model、YAML/environment configuration、
  container/Helm values。
- Compatibility promise: Semantic Versioning；V1.x 不任意移除或重新命名 metric，
  metric deprecation 至少保留一個 minor release；不相容 REST 變更使用新 major
  API path。
- Migration strategy: 無資料庫 migration。設定變更必須提供 validation、預設值與
  migration notes；metric/API breaking change 必須使用新的 governed change。
- Deprecation process: 在 spec、changelog、runtime warning（適用時）與 release notes
  宣告，至少保留規格要求的相容期並提供替代方案。
- Required governance mode: `required`
- Approval roles: Project Maintainer 核准需求、相容性與 Gate 2/Gate 4；涉及 credential、
  authorization、CI write permission、artifact signing 或 production deployment 時，
  Security Reviewer 也必須核准。不得用角色名稱冒充尚未發生的核准。

## Deployment and Operations

- Deployment targets: Docker/OCI、Kubernetes Deployment 與 Helm Chart；V1.0 預設
  單 replica。
- Release mechanism: `.github/workflows/release.yml` 對 prerelease tag 執行 deterministic、
  race、container、schema、synthetic scale、dependency/license、multi-architecture image
  scan、SBOM、Sigstore 與 OCI provenance 後發布；public/Enterprise repository 另產生
  GitHub-native attestation，private RC 以 signed boundary asset 揭露平台限制；stable
  tag 額外要求正式 ECS、CE、deployed E2E、deployed performance 與 native attestation。
- Health/readiness evidence: `/health` 與 `/api/v1/health`；liveness 不因單一 ECS
  失效而失敗，readiness 依可服務 cache 判斷。
- Observability: 結構化 JSON logs、`ecs_exporter_*` self-monitoring metrics、collector
  state、cache age、API duration/error 與 build info；V1.0 不提供 native tracing。
- Rollback or forward-recovery: 以先前已驗證 image digest 與 Helm release rollback
  回復；設定必須向後相容或附 migration notes。Cache 為記憶體資料，重啟後重新收集。
- Incident and escalation path: Project Maintainer；security/credential 事件升級至
  Security Reviewer；操作與事故處理程序見 `docs/PRODUCTION_RUNBOOK.md`。

## CI and Supply Chain

- CI system and workflow paths: GitHub Actions `.github/workflows/ci.yml`；
  security contract 見 `GITHUB_ACTIONS.md`。
- Default token permissions: repository contents read-only；只有個別發布 job 可取得
  packages write、attestation 或 deployment 權限。
- Third-party action pinning policy: 全部固定到已審查的完整 commit SHA，旁註人類可讀
  release；更新透過受審查 PR。
- Workload authentication: registry/cloud 優先使用 GitHub OIDC/workload identity；
  禁止新增長效 cloud credential，除非 Security Reviewer 記錄無替代方案與輪替流程。
- Dependency update ownership: Project Maintainer 每月至少檢視一次；Critical/High
  advisories 依風險即時處理。
- Artifact provenance, SBOM, checksums, and signing: release 產生 checksums、SPDX SBOM、
  BuildKit OCI provenance/SBOM 與 Sigstore keyless signatures；GitHub-native attestation
  依 repository plan 執行，private RC 以 signed boundary asset 揭露，stable 必須成功。
  OCI image 與 Helm chart 發布至 GHCR，GitHub Release 保存 archive 與驗證資料。

## Known Constraints

- 目標版本族已選定為 ECS 3.6.x、3.7.x、3.8.0.x 與 3.8.1.x。共用功能依 ECS-011
  採跨版本 live evidence，已列於每個 Profile 的 `shared_validated_capabilities`。
  3.7/3.8 REST API ZIP 與 exact-build reports 可補強版本差異，但不阻擋共享功能狀態。
- Dell 已記錄 ECS 3.7/3.8.0 Flux range 問題；這兩個 Profile 禁用 interval-derived
  rate。ECS 3.8.1 只有在 live range-boundary gate 通過後才能啟用；ECS CE 3.8.1.4
  的 Flux external query 回 HTTP 503，不能用來通過此 gate。
- ECS 3.8.0/3.8.1 可能受 accepted server names/Host Header 影響；proxy/load
  balancer 測試是版本認證必要項目。
- Go 已固定為 language 1.26.0／toolchain 1.26.5；Git remote 是
  `crispkid/Dell-ECS-Metrics-Exporter`。`dell-ecs-metrics-exporter` module path 仍是內部
  application module identity，未對外承諾 Go library import compatibility。
- 儲存庫已有完整 fixture/component-verified runtime、unit/component tests、HTTP
  API、Dockerfile、Helm Chart 與 CI workflow，以及 ECS CE 3.8.0.3/3.8.1.4 部分
  Management live evidence與 ECS 3.8.1.1 實體設備 Node/Performance Flux partial
  live evidence；尚無正式 ECS 3.8.1.4、multi-VDC、container startup、live
  Kubernetes、target-scale 或完整 Profile 認證。
- ECS API response size 與 10,000 buckets 的記憶體/效能目標是初始估計，必須以
  目標版本實測。
- V1.0 多副本策略預設為單 replica；若要求多 replica，必須先選擇 leader election
  或核准 ECS API 負載可接受的獨立 polling。

## Project-Specific Agent Rules

- 不得臆造 Dell ECS API URI、欄位、硬體資料、metric scope 或 counter 語意；只有
  在任一目標版本具有 qualifying live evidence 的功能可標示 `validated-shared`。
- Profile 的 `tested_builds` 只能由隔離真機 integration evidence 更新；文件與
  synthetic fixture 不足以填入。
- Prometheus scrape 僅可讀 cache，不得觸發 Dell ECS API。
- 未設定 quota 時不得使用 cluster capacity 代替；quota metric 應省略，Inventory
  回傳 `null` 與 configured flag。
- 單一 page 或 resource 的部分結果不得覆蓋上一份完整成功 cache。
- Owner、description、完整 URL、error message、token 與未受控值不得成為 metric
  label。
- 任何 metric/API/config public contract 變更都必須同步更新 specification、API
  mapping、test plan、traceability 與 changelog。
