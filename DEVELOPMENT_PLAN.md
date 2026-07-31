# Development Plan

## ECS-012 v1.0.0-rc.1 release

### Scope and Decision

以 `v1.0.0-rc.1` 結束 repository-owned V1 開發並發布 GitHub Pre-release。RC 保留
deterministic、race、container、Kubernetes schema、合成規模、dependency/license、
雙架構 image scan、SBOM、signing 與 OCI provenance；private RC 缺少 GitHub-native
attestation 支援時發布 signed boundary asset，stable 仍要求 native attestation。只有帶 prerelease suffix 的 tag
可略過正式 ECS/CE、deployed E2E 與 deployed performance。Stable tag 仍須通過全部
production gates。

### Implementation Plan

1. 新增 fail-closed tag classifier 與 workflow RC/stable 分流。
2. 建立版本化 release notes、RC checklist、changelog 與安裝/status 文件。
3. 完成 Harness、race、release policy、secret 與 full-diff review。
4. 提交、建立 `v1.0.0-rc.1` tag、push 並觀察 release workflow。
5. 只有 signed artifacts、GitHub Pre-release 與 GHCR publication 可觀察時才回報發布完成。

### Approval Record

- Gate 2 Status: user explicitly selected and authorized `v1.0.0-rc.1` publication
- RC publication authorization: granted on 2026-08-01
- Stable Gate 4 Status: not approved; stable external evidence and named reviewers remain pending

### Verification Status

- Local Harness self-test/doctor/verify, 84.7% coverage, `govulncheck`, fresh race,
  synthetic target-scale, actionlint, RC archive/Helm build, checksum and secret/diff checks
  passed. Commit/tag and GitHub publication evidence remain pending; see `plans/ECS-012.md`.

## ECS-011 Cross-version shared feature validation

### Scope and Decision

依使用者決策，同一 Exporter 功能只要在任何一個目標 ECS 版本用 production path
完成真實驗證，就視為四個 Profile 都已驗證。已知版本差異、`unavailable` 與從未在
任何版本取得 live 欄位的功能維持個別狀態；exact-build execution 與完整 Profile
certification 不偽造、不外推。

### Implementation Plan

1. 在 Profile evidence 新增共享驗證 policy 與 capability 清單。
2. Loader 拒絕不存在或 `unavailable` 的共享 capability。
3. 建立中央功能矩陣並更新四版本 mapping、README、規格與 release policy。
4. 保留 Flux interval、Host Header 與 unsupported scope 的版本差異。
5. 執行 targeted、race 與完整 Harness。

### Approval Record

- Gate 2 Status: user explicitly directed cross-version feature validation
- Gate 4 Status: not approved; delivery and named reviewer gates remain pending

### Verification Status

- Four Profile shared lists, negative policy/unavailable tests, release evidence expression,
  fresh race and full Harness passed. Coverage is 84.7%; no known Go vulnerabilities were found.

## ECS-010 Portable read-only Flux compatibility validation

### Scope and Decision

ECS-010 在只有 ECS 3.8.1.1 實體設備、其他版本只有 ECS CE 的限制下，採用三層
證據：官方文件 contract reconciliation、四 Profile fixture replay、以及可交付給
exact-build appliance 管理者執行的去識別化唯讀 Probe。CE/fixture 不替代 appliance
Flux；Profile certification 狀態不因本 change 改變。

### Implementation Plan

1. 比對 Dell 3.6 Flux/Performance 文件與 3.7/3.8 documentation index/known issue。
2. 建立 `internal/fluxprobe` 與 `cmd/ecs-flux-probe`，重用 production client、Profile、
   collector、parser 與 cache mapping。
3. 限制 report 只含 exact build、Profile/policy、safe error/HTTP status 與 series count；
   省略所有 identity、secret、endpoint、raw response/value。
4. Replay 3.6、3.7、3.8.0、3.8.1 fixtures，加入 empty window、503/redaction 與 Disk
   allowlist regression。
5. 將 Probe 加入一般/release build並同步支援矩陣、mapping、test 與操作文件。
6. 執行 deterministic/race/Harness checks；原規劃的 3.6/3.7/3.8.0 report requirement
   已由 ECS-011 改為 optional version-specific regression evidence。

### Approval Record

- Gate 2 Status: user accepted the layered validation and portable probe recommendation
- Gate 4 Status: not approved; exact-build appliance evidence and named reviewers remain pending

### Verification Status

- Four-profile fixture replay passed expected Profile selection, version-specific interval
  policy, split Node/Performance counts, all-null empty handling, 503 redaction and Disk guard.
- Local synthetic HTTPS calibration passed real client login/query/logout and emitted only the
  redacted report contract. Fresh race and the complete required Harness passed at 84.7%
  coverage; the security stage found no Go vulnerabilities.
- Deterministic release build completed with both `ecs-exporter` and `ecs-flux-probe` in the
  Linux archive. Deployment checks used documented strict static fallbacks where local tools
  were unavailable.
- Exact ECS 3.6/3.7/3.8.0 appliance Probe reports are not available. ECS-011 later promoted
  qualifying shared functions without inventing exact-build Profile certification.

## ECS-009 ECS 3.8.1.1 Flux and TLS compatibility correction

### Scope and Decision

ECS-009 依授權實體 ECS 3.8.1.1 唯讀結果修正 Node resources 與 Performance
Flux contract，並依使用者決策允許所有 environment 明確設定
`tls.verify: false`。預設驗證與 `caFile` 支援不變；停用驗證必須 WARN。

### Implementation Plan

1. 移除 production TLS false validation prohibition，加入不含 endpoint 的 startup WARN。
2. 將 CPU、Memory、Network 與 conditional Disk 拆成 ECS 支援的精確
   `filter`/`keep`/`last` queries，全部成功後原子合併。
3. 將 VDC core、latency、Namespace request rates 分開查詢；只接受官方與實體
   共同證實的 measurement/field/tag。
4. 移除未證實的 Namespace throughput/latency public families，修正 rate/unit/status。
5. 補 fixtures、parser/collector/metrics/mock tests、相容性文件與 partial-live record。
6. 執行 Harness、race 與 corrected Exporter appliance rerun；保留 certification gate。

### Approval Record

- Gate 2 Status: user explicitly approved TLS verification as optional and required all other fixes
- Gate 4 Status: not approved; formal certification and named reviewers remain pending

### Verification Status

- Corrected Exporter selected `ecs-3.8.1` for exact appliance build
  `3.8.1.1.140118.8d698782e5d`; readiness was `UP`.
- Five CPU, five memory-used, five memory-total, ten network-receive and ten
  network-transmit series were exported; 32 current metric families passed
  `metricscheck`.
- Performance and Node split collectors completed repeatedly. The final
  zero-load window returned no VDC core or Namespace rows; the ECS all-null
  placeholder was accepted as an empty result.
- Race, lint, format, typecheck, tests, 84.6% coverage, build, CI policy and
  deployment checks passed. The required security refresh stopped because the
  external vulnerability database could not be resolved, so the full Harness
  handoff did not pass.
- The redacted appliance record is
  `docs/ecs-api/validation/ecs-appliance-3.8.1.1-2026-07-30.md`. Formal
  certification and Gate 4 remain pending.

## ECS-008 Production readiness and Bare Metal delivery

### Scope and Decision

ECS-008 完成首次 production release 所需的可執行 delivery controls，並新增
systemd-based Linux Bare Metal 部署。此 change 不修改 ECS mapping 或把任何
Profile 升級為 certified；真實 ECS、target-scale、scanner/runtime、Git remote
及 reviewer prerequisites 缺少時，release gate 必須 blocked。

### Approval Record

- Gate 2 Status: user explicitly requested production-readiness completion and Bare Metal
- Gate 4 Status: not approved; external evidence and named reviewers remain pending
- Approved By: no named Project Maintainer or Security Reviewer has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. 建立 Linux amd64/arm64 systemd/sysusers/tmpfiles deployment，支援安全安裝、升級、
   config/Profile preflight、verify、rollback 與保留設定的 uninstall。
2. 將 OCI base image/actions 固定至 digest/SHA，建立 reproducible release archive、
   checksums、SBOM、High/Critical scan、keyless signing、provenance 與 protected publish。
3. 擴充 Helm ingress selectors/DNS-ECS egress 及 fail-closed production values，
   加入 kubeconform production gate。
4. 建立 exact-build live integration、deployed E2E、container smoke、synthetic 與
   deployed target-scale performance scripts；缺少 prerequisite 時 exit 3。
5. 建立 Prometheus alert rules、Production Runbook、Release Checklist 與 artifact
   verification instructions，同步所有 governed contracts。

### Verification Status

- Repository-controlled implementation is complete. Unit/component and fresh
  race suites, 84.3% coverage, build/Profile validation, governance, CI policy,
  Helm/Bare Metal static checks, deterministic two-build archive comparison,
  checksums, and synthetic 10/100/10,000 performance passed.
- Full Harness verification reached the security stage after deterministic
  stages passed, then failed because `vuln.go.dev` could not be resolved. This
  is a required external-data failure, not a skip or product pass.
- Release policy, supply-chain, container, Kubernetes schema, live
  certification, deployed E2E, and deployed performance entry points all
  returned blocking status when their Git/tool/runtime/credential prerequisites
  were absent.
- Exact production ECS 3.8.1.4, applicable ECS CE 3.8.0.3 rerun, deployed
  target-scale, container runtime, live Kubernetes/systemd, external
  vulnerability/SBOM scanners, Git remote/tag, signed published artifacts, and
  named reviewer approvals are external release gates and are not claimed by
  this change. The login-gated ECS 3.8.1 REST reference must also be downloaded,
  reviewed, and reconciled before Profile certification can be recorded.

## ECS-007 Correct ECS 3.8.0.3 Bucket compatibility

### Scope and Decision

ECS CE 3.8.0.3 非空 Bucket follow-up 證實 quota 與單筆 billing 使用 top-level
response，且 billing `9765.625 KB` 對應已知 10,000,000 bytes。Batch billing
probes 另證明 request body shape 會產生 415、200/plural empty envelope、
400/code1013、500/code999。此 change 保留 inherited nested response，新增
top-level/plural mapping、`{"id":[bucket names...]}` BucketListParam contract，
並修正 billing KB=1024。

證據只涵蓋 exact CE build 的部分 Management API。Billing MB/GB/TB、capacity/quota
GB、正式版 Flux、正式設備 3.8.1.4 與整個版本 range 均不在此變更的認證範圍；
`tested_builds=[]`、`sandbox_certified=false` 不變。

### Approval Record

- Gate 2 Status: user explicitly requested correction of the observed incompatibilities
- Gate 4 Status: not approved; required external vulnerability scan and formal certification
  remain pending
- Approved By: no named Project Maintainer or Security Reviewer has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. Bucket quota/single billing parser 同時接受 nested 或 top-level envelope，拒絕雙
   envelope、missing 與 malformed response。
2. ECS client 安全解析 normalized numeric error code，不保存或輸出 raw error body。
3. Batch billing 使用 `{"id":[bucket names...]}`，支援 plural envelope；只有
   404/405/501 或 missing requested item fallback，500/code999 不 fallback。
4. Billing KB 改乘 1024；其他 unit 與 capacity/quota GB 保持既有 decimal。
5. 新增 partial-live fixtures、unit/component/race tests，同步規格、mapping 與
   evidence boundary。
6. 執行完整 Harness，之後使用保留的三 Bucket/four-object dataset live rerun。

### Verification Status

- Pre-fix live API observation completed: three Buckets, four objects, 10,000,000 bytes,
  soft/hard quota 1/2 GB, Bucket billing `6835.9375`/`2929.6875`/`0 KB`, Namespace
  `9765.625 KB`, and the batch no entity/`{}`/`[]`/`null` response matrix.
- Built-in class and non-empty live probe confirmed `BucketListParam.id` and a three-item
  plural response.
- Corrected Exporter live rerun passed: batch HTTP 200, Bucket collector succeeded at least
  twice, Inventory returned three correct Bucket records, and Cluster/Namespace aggregates
  reported four objects and 10,000,000 bytes where applicable.
- Readiness was HTTP 200 `DEGRADED`; the only collector error was the known CE Flux
  node-resources 503. The selected Profile was `ecs-3.8.0` and certified Profiles remained
  empty.
- The race suite passed. Harness lint/format/typecheck/test/84.5% coverage/build/CI policy
  and the separate deployment check passed. Complete `verify` remains failed because the
  execution policy did not authorize `govulncheck` to fetch its external vulnerability
  database. The exact-build partial-live pass must not be reported as whole-Profile
  certification.

## ECS-006 Complete ten runtime and mapping gaps

### Scope and Decision

使用者要求先完成規格稽核中十項「尚未實作或實作不完整」項目。本 change 補完
Performance、conditional capability、Node resource/service、Inventory 欄位、stale
cutoff、rate/jitter、network cardinality、batch billing、unit/response telemetry 與
cache-refresh correctness。所有新增 API 行為預設保守：conditional 預設關閉，
filesystem 必須 allowlist，任何部分 enrichment 不覆蓋完整 cache。

本 change 只建立 synthetic fixture/component evidence；不宣稱 ECS CE 3.8.0.3 Flux
或正式設備 3.8.1.4 已通過，也不修改四個 Profile 的 `tested_builds=[]` 與
`sandbox_certified=false`。

### Approval Record

- Gate 2 Status: user explicitly requested implementation of the ten audited gaps
- Gate 4 Status: not approved; live ECS, scale, supply-chain and reviewer evidence remain
- Approved By: no named Project Maintainer or Security Reviewer has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. 擴充 strict config/Profile schema，加入 rate limit、conditional enable 與
   filesystem/interface policy，並允許完整 future sandbox evidence。
2. 擴充 model/cache/parser，加入 VDC/Namespace performance、Node service/process、
   usage sample time、Inventory 欄位、TB 與 response validation。
3. 更新 collectors：capability/recovery guard、Node policy、batch billing/fallback、
   Performance atomic replacement 與 per-domain cache generation。
4. 更新 ECS client/scheduler/metrics：per-cluster token bucket、initial jitter、
   bounded network Counter、response-size histogram 與 `maxStale` domain cutoff。
5. 更新 fixtures、README、mapping/spec/governance documents，執行 unit/component、
   race、coverage、security、build、deployment 與 CI policy gates。

### Verification Results

- `go test ./...` 已通過，涵蓋 Performance mapping、conditional enable/deny、
  Node disk/service、Inventory/billing sample、batch fallback、TB、rate cancellation、
  initial jitter、network cardinality/reset、stale cutoff、response size 與 skipped refresh。
- `go test -race -timeout=3m ./...` 已通過所有 package，未偵測到 data race。
- `./HARNESS/harness.sh verify` 已通過九個 required stages；coverage 84.4%，高於
  80% threshold。
- `go mod verify` 與 `govulncheck` 通過，未發現已知 Go vulnerability；credential
  pattern scan 未發現敏感資料。
- 四個 Profile build validation、Helm lint/template 與 GitHub Actions policy checks
  均通過。Helm 只有非阻擋的 Chart icon 建議。
- 未執行真實 ECS Flux、正式設備 3.8.1.4、target-scale performance、container
  startup、live Kubernetes、image scan、SBOM/provenance/signing。

## ECS-005 Validate ECS CE 3.8.0.3 Management compatibility

### Scope and Decision

使用者授權在隔離 ESXi 部署 ECS CE 3.8.0.3 並繼續完成 API 相容性驗證。本 change
只把真實觀察到的 Management API envelope、Bucket scope 與 partial failure behavior
納入 runtime；不把 CE 當成正式設備，不宣稱整個 3.8.0 range 或正式環境 3.8.1.4
已認證。

### Approval Record

- Gate 2 Status: user authorized the isolated ECS CE deployment and compatibility work
- Gate 4 Status: not approved; production Flux, failure, scale and reviewer evidence remain
- Approved By: no named Project Maintainer or Security Reviewer has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. 完成 ECS CE storage pool、VDC、RG 與 least-privilege management user 初始化。
2. 建立空白、可清理的測試 Namespace 並執行真實 Exporter smoke。
3. 修正 top-level capacity/quota、HAL Node health 與 Namespace-scoped Bucket mapping。
4. 將 Node Management 與 Flux Resources 分離，保留 partial data 並呈現 DEGRADED。
5. 新增 regression/race tests、redacted evidence 與 governance traceability。
6. 執行完整 Harness、security、race 與 final diff/secret review。

### Verification Results

- ECS CE exact build `3.8.0.3.138685.3a0a9b6bf3a` 選用 `ecs-3.8.0` Profile。
- Login/whoami/logout、Host accepted/rejected、Cluster/Node Management、空 Namespace
  quota/billing、Namespace-scoped 空 Bucket list 均取得 live response 並通過修正後
  Exporter。
- 五個 Inventory endpoint、未授權 401 與 Prometheus exposition smoke 通過。
- CE Flux 持續回 HTTP 503/code 6503；Node inventory/health 保留，readiness 回
  HTTP 200 `DEGRADED` / `collector_error`，resource metrics 不捏造。
- `tested_builds=[]`、`sandbox_certified=false`；正式版 Flux、non-empty resources、
  REST ZIP、fault injection、3.8.1.4、scale/release gates 未執行。
- Final `./HARNESS/harness.sh verify` passed all nine required stages with 86.4% coverage,
  no known Go vulnerabilities, successful Helm/CI policy checks and governance traceability.
- Full `go test -race -timeout=3m ./...` passed every package.

## ECS-004 Complete the mock-verified V1 runtime

### Scope and Decision

使用者要求完成全部開發工作。此 change 實作可在沒有真實 ECS 的情況下完成並驗證的
V1 product runtime：strict configuration、per-cluster ECS client/auth、Profile
bootstrap、collectors/scheduler、atomic cache、Prometheus mapping、Inventory/health
API、structured telemetry、Docker/Helm 與 read-only CI。

使用者已明確表示實際 ECS 測試將另行安排，因此 ECS-004 的完成定義是程式與
mock/component/deployment-static gates 完成，不包含捏造真機、10,000 Bucket、
container runtime 或 production evidence。四個 Profile 的 `tested_builds` 與
`sandbox_certified` 保持未認證。

### Approval Record

- Gate 2 Status: working V1 specification accepted for implementation through the user's request
- Gate 4 Status: not approved for production release; live ECS and release evidence remain outstanding
- Approved By: no named Project Maintainer or Security Reviewer has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. 實作 strict YAML/env/secret/TLS/interval/security configuration 與 validation CLI。
2. 實作 ECS Basic login/token/whoami/logout、401 reauthentication、retry/backoff/jitter、
   timeout、bounded concurrency、circuit breaker、safe error 與 correlation logging。
3. 實作文件 mapping 的 Management/Flux parser、decimal unit、enum/range/null guard。
4. 實作 Cluster/Node/Namespace/Bucket/Performance/Replication/Recovery collector、
   bucket pagination/enrichment、capability guard、single-flight scheduler 與 atomic cache。
5. 實作 domain/self Prometheus metrics、health state、Inventory auth/filter/sort/page
   與 RFC 9457。
6. 完成 hardened image/Helm config/Secret mounts、SHA-pinned GitHub Actions 與
   local CI policy gate。
7. 以 unit/component/race/coverage/security/deployment checks 驗證並同步全部契約文件。

### Verification Results

- Unit/component tests：已涵蓋四版本 fixture mapping、auth reauthentication、
  503 retry、circuit、pagination loop/duplicate/partial failure、scheduler/cache race、
  health、Inventory auth/query/problem 與 Prometheus text parser。
- Go application package coverage：86.1%，高於 80% required threshold。
- Helm lint/template 與 CI policy static checks：已通過。
- `go test -race -timeout=3m ./...`：全部 package 通過，沒有偵測到 data race。
- `govulncheck v1.6.0`、module checksum 與 credential-pattern scan：通過，沒有發現
  已知 Go vulnerability 或 credential-like material。
- Binary build 與 `-validate-profiles`：通過，四個 Profile 均載入成功。
- `./HARNESS/harness.sh selftest`、`doctor`、`governance:doctor`、`verify`：通過；
  lint、format、typecheck、test、coverage、build、security、deploy、ci-policy
  九個 required stages 全部 passed。Selftest 明確提示本機沒有 ShellCheck，但 Bash
  syntax 與 behavioral tests 已執行。
- 真實 ECS、10,000 Bucket performance、container startup、Kubernetes live apply、
  HTTP listener smoke、image scan 與 release supply-chain evidence：未執行，不可
  視為通過。

## ECS-003 Bootstrap the compatibility runtime

### Scope and Decision

使用者於 2026-07-25 要求開始準備開發。本 change 固定 Go 工具鏈，先實作不需要
真實 ECS credential 的最小垂直切片：Profile contract loader、Dell 四段版本解析、
uniform/mixed version resolution、fixture contract tests、bootstrap HTTP server 與
Docker/Helm 開發骨架。

此 change 不實作 ECS authentication、polling、cache 或 domain metric，也不把
synthetic fixtures 升級為 sandbox evidence。正式 Git remote 尚未設定，因此 module
path 暫定為 `dell-ecs-metrics-exporter`，來源位置決定後再做機械式 rename。

### Approval Record

- Gate 2 Status: not complete — existing working specification baseline has no formal approval record
- Gate 4 Status: not started — user requested development preparation, not V1.0 release approval
- Approved By: no formal approver has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. 固定 Go language 1.26.0、toolchain/build 1.26.5 與 `govulncheck` v1.6.0。
2. 實作 strict Profile loader、四段版本 parser、range selection、unknown rejection
   及 mixed-version capability intersection。
3. 直接以 repository 四版 `nodes.json`、mixed/unsupported fixtures 驗證版本選擇，
   並驗證所有 manifest、mapping reference、Flux row/time boundary 與 secret hygiene。
4. 建立只呈現 bootstrap 狀態的 HTTP entrypoint；readiness 在沒有可服務 cache 時
   回傳 HTTP 503／`DOWN`，不產生 domain metrics。
5. 建立 Dockerfile 與 Helm Chart，包含 ConfigMap、existing Secret reference、
   probes、resources、non-root/read-only security context、NetworkPolicy、PDB 與
   optional ServiceMonitor。
6. 將 lint、format、typecheck、test、coverage、build、security、deploy 設為
   Harness required stages，並執行完整 `verify`。
7. 下一個 change 從 multi-cluster config schema 與 ECS bootstrap/auth client 開始；
   真機 integration 由使用者另行安排環境後執行。

### Verification Results

- `./HARNESS/harness.sh selftest`：2026-07-25 通過；本機無 ShellCheck，Harness
  syntax 與 behavioral tests 仍全部通過。
- `./HARNESS/harness.sh doctor`、`governance:doctor`：2026-07-25 通過；Go 1.26.5、
  Helm 4.2.3、required tools/stages 與 ECS-003 traceability 有效。
- Unit/contract tests：2026-07-25 通過；包含四段版本、四 Profile、mixed/unknown、
  30 個 fixture JSON、5 份 manifest、24 筆 fixture record、5 個 Flux fixture 與
  HTTP bootstrap。
- Go application package coverage：84.0%，高於 80% required threshold。
- `govulncheck v1.6.0`：2026-07-25 連線官方 vulnerability database 執行，沒有發現
  已知漏洞；credential-pattern scan 與 module checksum verification 通過。
- Helm 4.2.3 lint/template 與 deployment contract：2026-07-25 通過；包含 required
  resources、probe、resource、non-root/read-only、existing Secret reference 與
  optional ServiceMonitor。
- Built binary loopback smoke：2026-07-25 通過；`/health` 回 200/UP、
  `/api/v1/health` 回 503/DOWN、version 顯示 4 個 supported／0 個 certified
  Profile，`/metrics` 僅含 bootstrap self metrics。
- `./HARNESS/harness.sh verify`：2026-07-25 通過；8 個 required stages 全部 passed，
  evidence 寫入未提交的 `test-results/harness/verify.json`。
- Live ECS integration、container startup、Kubernetes live apply：本 change 不執行，
  不得宣稱通過。

## ECS-002 Add ECS 3.6/3.7/3.8 compatibility contracts

### Scope and Decision

使用者於 2026-07-25 要求建立 ECS 3.6、3.7、3.8.0、3.8.1 四個版本的 Profile、
mapping 與 fixtures。此 change 建立文件與合成 contract assets，不建立 Go runtime，
也不宣稱任何真實 ECS build 已認證。

採用的設計：

1. `profiles/` 使用 machine-readable JSON 記錄版本範圍、transport、capability、
   known issue 與 evidence status。
2. `docs/ecs-api/common-contract.md` 記錄共用 logical mappings，四份版本文件只覆寫
   evidence、差異及 certification gap。
3. `testdata/ecs/common/` 保存共用 Management API fixtures；版本目錄保存 version
   discovery、Flux range 與 Host Header 特有情境。
4. ECS 3.7/3.8.0 依 Dell KB 000211906 停用 Flux interval-derived rates。
5. ECS 3.8.0/3.8.1 依 Dell KB 000205031 保留 hostname、TLS SNI 與 Host 一致性。
6. Fixtures 全部標記 `synthetic-document-derived`；Profile 的 `tested_builds` 保持空白。

### Evidence and Remaining Blockers

- ECS 3.6 REST API Reference 與 3.6.1 Monitoring Guide 已取得並檢視。
- Dell 官方索引確認 ECS 3.7、3.8.0、3.8.1 REST API Reference 存在，但下載需要
  Dell Support 登入；目前 mapping 明確標成 `candidate-inherited`。
- 尚無隔離 ECS sandbox、測試 credential、精確 live build 或 redacted live fixture。
- Management unit multiplier、enum、pagination、Host Header 與 Flux boundary 均仍需
  真機驗證。

### Approval Record

- Gate 2 Status: not complete — user requested the artifacts, but no formal specification gate approval is recorded
- Gate 4 Status: not started — no Go runtime implementation or sandbox certification exists
- Approved By: no formal approver has been recorded
- Approved On: no formal approval date has been recorded

### Implementation Plan

1. 建立 Profile schema 與四個版本 Profile。
2. 建立共同 API contract 與四份版本 mapping。
3. 建立 fixture manifest schema、共同 Management fixtures 與版本專屬 fixtures。
4. 同步更新 specification、mapping index、project context、test plan、traceability
   與 changelog。
5. 驗證 JSON、schema shape、manifest reference、mapping ID、secret hygiene、Markdown
   links 與 Harness。
6. 取得 Dell Support 文件及 sandbox 後，將 candidate mapping 升為 documented/live，
   加入 redacted fixtures 與精確 `tested_builds`。

### Verification Results

- JSON/profile/manifest contract validation：2026-07-25 通過；35 個 JSON、4 個
  不重疊且符合目標版本的 Profile range、5 個 manifest、24 個 fixture records、5 個 Flux fixtures、
  mapping reference 與 credential-string scan 均通過。
- Markdown local link／placeholder inspection：2026-07-25 通過；本 change 沒有失效的
  local Markdown link 或未處理 placeholder。
- `./HARNESS/harness.sh selftest`：2026-07-25 通過；環境沒有 `shellcheck`，但 syntax
  與 behavioral self-tests 均執行成功。
- `./HARNESS/harness.sh doctor`：2026-07-25 通過。
- `./HARNESS/harness.sh governance:doctor`：2026-07-25 通過；ECS-002 可跨治理文件追溯。
- `./HARNESS/harness.sh verify`：2026-07-25 通過；目前是 specification-only repository，
  required stages 為 `none`，因此 lint、format、typecheck、test、coverage、build 未配置
  且由 Harness 明確 skip。這些 skip 不是產品實作成功 evidence。

## ECS-001 Establish the V1.0 specification baseline

### Specification Understanding

本 change 將 Dell ECS Metrics Exporter 定義為 Go-based、read-only、cache-backed 的
多 Cluster Prometheus exporter 與 Inventory API。V1.0 必須涵蓋 authentication、
Cluster/Node/Namespace/Bucket/Performance/Replication/Recovery collectors、
Prometheus mapping、Inventory API、health/self-monitoring、resilience、security
及 Docker/Kubernetes/Helm deployment。`/metrics` 只能讀取 cache；真實 Dell ECS
adapter 必須等目標版本的官方 API mapping 完成後才能實作。

### Discussion and Confirmation Items

已確認：

- 2026-07-25 使用者同意保留 `HARNESS/templates/` 中會被 selftest 使用的共用
  `.md.example`，並在根目錄建立完整專案文件。
- 實作語言採 Go；精確版本在建立 `go.mod` 前由 Project Maintainer 固定。
- CI/交付基線採 GitHub Actions、Docker/OCI、Kubernetes 與 Helm。
- 未指定個人時使用 Project Maintainer、Security Reviewer 角色描述，但角色名稱不
  等同正式核准。

尚未解決且會阻擋相關工作：

- 目標 Dell ECS version/build 與官方 API 文件未提供，阻擋所有真實 URI/schema、
  authentication variant 與 adapter implementation。
- 隔離 ECS sandbox、測試 credential 與 redacted fixtures 尚未提供，阻擋真實
  integration evidence。
- Go 精確版本、OCI registry、artifact signing tool 與 production runbook 尚未選定；
  分別阻擋產品 bootstrap、首次發布與 production deployment。

### Approval Record

- Gate 2 Status: not complete — recommendations were confirmed, but the full specification baseline has not received formal approval
- Gate 4 Status: not started — there is no implementation plan approved for execution
- Approved By: no formal approver has been recorded
- Approved On: no formal approval date has been recorded

`HARNESS_REQUIRE_PLAN_APPROVAL` 在規格整理階段為 `false`，因此 `verify` 只驗證文件
完整性與 traceability，不代表 Gate 2 或 Gate 4 通過。進入產品實作前必須由真實
Project Maintainer 更新並核准這些欄位。

### Implementation Plan

1. 將附件內容納入 `SPECIFICATION.md`，補上 ECS-001、REQ-001 至 REQ-012 與可觀察
   acceptance criteria。
2. 將根目錄 `PROJECT.md.example`、`PLANS.md.example`、
   `CODE_REVIEW.md.example` 專案化並移除 `.example`；保留 HARNESS 共用範本。
3. 建立 `DELL_ECS_API_MAPPING.md`、`SPEC_CHANGELOG.md`、`TEST_PLAN.md`、
   `TRACEABILITY.md` 與 `GITHUB_ACTIONS.md`。
4. 建立 `HARNESS/config.env` 與 `HARNESS/ACTIVE_CHANGE`；規格階段明確使用
   no-code gate，並要求未來 implementation change 開啟完整 stages。
5. 驗證沒有未處理 placeholder、broken local links、敏感值或 project-owned
   `.md.example`，執行 Harness selftest、doctor、governance doctor 與 verify。
6. Project Maintainer 選定 Dell ECS version/build，取得官方文件、sandbox 與去識別化
   fixtures，逐一完成 API mapping 及 review。
7. 選定並固定 Go/tool versions，建立 source、tests、CI、container 與 Helm artifacts，
   同時把 Harness required stages 從 `none` 改為產品交付 gate。
8. 依 `TEST_PLAN.md` 完成 component、真實 integration、performance、security 與
   deployment evidence，經 Gate 4 核准後才可宣告 V1.0 ready。

### Verification Results

- Markdown placeholder scan: 2026-07-25 通過；專案專屬文件沒有未處理的常見
  占位標記或 angle-bracket placeholder。
- `./HARNESS/harness.sh selftest`: 2026-07-25 通過；ShellCheck 在本機不可用，但
  Harness syntax 與 behavioral self-tests 全部通過。
- `./HARNESS/harness.sh doctor`: 2026-07-25 通過；required paths、Bash、Git、
  governance/config 與 network contract 有效。
- `./HARNESS/harness.sh governance:doctor`: 2026-07-25 通過；ECS-001 可跨五份
  governance control files 追溯。
- `./HARNESS/harness.sh verify`: 2026-07-25 通過 specification-only gate，evidence
  寫入 `test-results/harness/verify.json`（不提交）。Lint、format、typecheck、test、
  coverage 與 build 因尚無產品程式碼而明確 skipped；`HARNESS_REQUIRED_STAGES=none`
  已在 `PROJECT.md` 記錄，不能作為未來產品 handoff 的通過證據。
- Product unit/integration/performance/security/deployment checks: 尚無產品程式碼或
  deployment artifact，不能執行且不得宣稱通過。
