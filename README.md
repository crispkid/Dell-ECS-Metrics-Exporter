# Dell ECS Metrics Exporter

Dell ECS Metrics Exporter 是以 Go 實作的唯讀、多叢集 Prometheus Exporter。它會依排程
登入 Dell ECS Management／Monitoring API，將完整且驗證成功的資料寫入記憶體快取，
再由 `/metrics` 與 `/api/v1` Inventory API 提供資料。Prometheus scrape 與 Inventory
查詢只會讀取快取，不會直接觸發 ECS API request。

## 快速導覽

- [End-to-End 上線導覽](#end-to-end-上線導覽)
- [選擇安裝方式](#1-選擇安裝方式)
- [本機快速開始](#3-本機快速開始)
- [完整設定說明](#5-完整設定說明)
- [Prometheus 與 Inventory API](#6-prometheus-與-inventory-api)
- [本機 Docker Prometheus 與 Grafana 測試](#本機-docker-prometheus-與-grafana-測試)
- [Docker 安裝](#7-docker-安裝)
- [Helm 安裝](#8-helm-安裝)
- [Bare Metal/systemd 安裝](#9-bare-metalsystemd-安裝)
- [維運與安全](#10-維運與安全)
- [故障排除](#11-故障排除)
- [開發、品質與發布](#12-開發品質與發布)
- [相容性與已知限制](#13-相容性與已知限制)
- [使用 Flux Probe 驗證實體 ECS](#使用-flux-probe-驗證實體-ecs)

## 目前狀態

目前版本目標為 **v1.0.0-rc.1 Release Candidate**。Runtime、測試、部署範本與
簽署發佈流程已具備，可供功能評估與整合測試；RC 不等於 `v1.0.0` 正式環境認證。

| 項目 | 狀態 | 說明 |
|---|---|---|
| Exporter runtime | 已完成 | 多 Cluster 收集、快取、Prometheus metrics、Inventory API、health/readiness 與安全控制已實作 |
| 本機品質驗證 | 已通過 | ECS-012 的完整 Harness（lint、format、typecheck、test、84.7% coverage、build、CI policy、security、deployment）、fresh race、actionlint、RC build/checksums 與 10/100/10,000 合成規模測試已通過；tag workflow 與 stable external gates 仍分開驗證 |
| ECS CE 3.6.2.0 | 部分實測 | Exact build 自動選擇 `ecs-3.6`；Management、known-size Bucket/quota/billing、Inventory/metrics 已通過；Flux `node-resources` 與 `performance` 回 HTTP 503，因此既有 live-certification gate 未通過 |
| ECS CE 3.7.0.0 | 部分實測 | Exact build 自動選擇 `ecs-3.7`；Management、known-size Bucket、正值 quota、billing、Inventory/metrics 已通過；Flux `node-resources` 回 HTTP 503，readiness 為 `DEGRADED` |
| ECS CE 3.8.0.3 | 部分實測 | Management、Bucket inventory/quota/billing 已以 non-empty data 驗證；Flux `node-resources` 仍受 CE 回 HTTP 503 限制 |
| ECS CE 3.8.1.4 | 部分實測 | Exact build 自動選擇 `ecs-3.8.1`；Management、known-size Bucket/quota/billing、Inventory/metrics 已通過；Flux 仍回 HTTP 503 |
| ECS 3.8.1.1 實體設備 | 部分實測 | Exact build 自動選擇 `ecs-3.8.1`；修正後 Exporter readiness 為 `UP`，五個 Node 的 CPU、RAM 與 Network metrics 已通過；Performance split query/空資料處理已驗證，TLS identity 與正式認證仍待完成 |
| ECS 3.8.1.4 正式設備 | 待認證 | 正式發布前仍須以同一個 release candidate commit 完成 live API、fixtures、mapping 與 Profile evidence 審查 |
| `v1.0.0-rc.1` | Release Candidate | Tag workflow 必須完成 deterministic/race、container、Kubernetes schema、合成規模、dependency/license、雙架構 image scan、SBOM、簽章與 OCI provenance，並標示為 GitHub Pre-release；private repository 的 GitHub-native attestation 限制會以 signed boundary asset 揭露 |
| Stable `v1.0.0` | 尚未放行 | 還需正式 ECS 3.8.1.4、exact CE 3.8.0.3、tagged deployed E2E、實際效能與具名 maintainer/security reviewer gate |

若要安裝並進行功能評估，請先閱讀
[End-to-End 上線導覽](#end-to-end-上線導覽)，再從
[選擇安裝方式](#1-選擇安裝方式)進入對應程序；若要建立正式 release，必須逐項完成
[Production Release Checklist](docs/RELEASE_CHECKLIST.md)。
RC 的必要條件與正式版保留門檻見
[Release Candidate Checklist](docs/RC_RELEASE_CHECKLIST.md)。
完整差距、證據與決策記錄於
[ECS-012 v1.0.0-rc.1 Release Plan](plans/ECS-012.md)、
[ECS-011 Cross-Version Feature Validation](docs/ecs-api/feature-validation.md)、
[ECS-010 Layered Compatibility Validation Plan](plans/ECS-010.md)、
[ECS-009 Compatibility Correction Plan](plans/ECS-009.md) 與
[ECS-008 Production Readiness Plan](plans/ECS-008.md)。

目前可辨識的版本範圍：

| Profile | ECS 版本範圍 | 目前證據 |
|---|---|---|
| `ecs-3.6` | `>= 3.6.0.0`、`< 3.7.0.0` | 共用功能 `validated-shared`；3.6 Dashboard 與 interval/native 規則維持版本別處理 |
| `ecs-3.7` | `>= 3.7.0.0`、`< 3.8.0.0` | 共用功能 `validated-shared`；Flux interval rate 維持 unavailable |
| `ecs-3.8.0` | `>= 3.8.0.0`、`< 3.8.1.0` | 共用功能 `validated-shared`；Host Header 與 Flux interval policy 維持版本別處理 |
| `ecs-3.8.1` | `>= 3.8.1.0`、`< 3.8.2.0` | 共用功能 `validated-shared`；Flux interval rate 維持 conditional |

功能驗證採 `shared-live-any-target-version`：任一目標版本以 production path 完成
真實驗證後，該功能即視為四個版本皆已驗證。完整清單與尚未在任何版本驗證的功能見
[Cross-Version Feature Validation](docs/ecs-api/feature-validation.md)。`tested_builds` 與
`sandbox_certified` 只保留 exact-build／完整 Profile 執行事實，不限制共享功能狀態。
ECS CE 實測範圍與限制記錄於
[ECS CE 3.8.0.3 Live Validation Record](docs/ecs-api/validation/ecs-ce-3.8.0.3-2026-07-25.md)
與
[ECS CE 3.8.1.4 Live Validation Record](docs/ecs-api/validation/ecs-ce-3.8.1.4-2026-07-26.md)，
實體設備結果另見
[ECS Appliance 3.8.1.1 Live Validation Record](docs/ecs-api/validation/ecs-appliance-3.8.1.1-2026-07-30.md)。
3.6.2.0 與 3.7.0.0 的本次結果目前只有本機、已去識別化且被 Git 排除的
`test-results/` evidence，尚未轉換成受審查的 committed fixtures 或 Profile
certification evidence。

## 使用 Flux Probe 驗證實體 ECS

無法在本機取得 ECS 3.6、3.7 或 3.8.0 實體設備時，可把同一 commit 建置出的
`ecs-flux-probe` 提供給客戶、Dell Support 或合作夥伴，在其 exact-build appliance
執行。Probe 使用與 Exporter 相同的 client、Profile、collector、query 與 parser，
但不會啟動服務或修改 ECS。

先依本 README 建立 `config.yaml` 與外部 secret files，再執行：

```bash
./scripts/build.sh
umask 077
./dist/ecs-flux-probe \
  -config config.yaml \
  -profiles-dir profiles \
  -cluster primary-ecs \
  -performance=true \
  -disk=false \
  -timeout 2m \
  > ecs-flux-probe.json
```

只有一個 Cluster 時可以省略 `-cluster`。Performance 預設開啟；Disk 預設關閉，
要開啟必須先在 `nodeResources.filesystems` 設定明確 allowlist。Exit code `0` 表示
所有啟用的 latest-snapshot 檢查完成；exit code `1` 時仍會產生 redacted JSON，供
判讀 setup/bootstrap 或個別 API 失敗。

Report 只包含完整 ECS build、選到的 Profile、version-specific capability policy、
受控 error type/HTTP status 與各 mapping 的 series 數量。它不包含 endpoint、IP/DNS、
帳號、密碼、Token、Node/VDC/Namespace/Bucket/filesystem identity、原始 response 或
sample value。詳細欄位、分享方式與證據限制見
[Flux Compatibility Probe Guide](docs/ecs-api/flux-probe.md)。

Probe `passed` 可補充 exact-build regression evidence。共用功能狀態依
`shared-live-any-target-version` 繼承；`tested_builds` 與 `sandbox_certified` 仍只記錄
實際執行與完整 Profile certification，不得填入未執行的 build。

## End-to-End 上線導覽

本節把後續的安裝、設定、Prometheus 整合、驗收與維運章節串成一條可執行路徑。
第一次導入時，建議先在隔離或測試環境完成本機驗證，再把相同的 ECS endpoint、
CA、credential reference 與 collector policy 移到正式部署方式。

### 資料流程

```text
Dell ECS Management / Monitoring API
                  │  Exporter 依 collector interval 主動輪詢
                  ▼
          Exporter 記憶體快取
             │            │
             │            └── /api/v1/*  唯讀 Inventory API
             └── /metrics
                    │  Prometheus 依 scrape interval 讀取
                    ▼
             Prometheus TSDB
                │       │
                ▼       ▼
             Grafana  Alertmanager
```

Prometheus scrape 與 Inventory API 都只讀取 Exporter 記憶體快取，不會因為查詢而
即時觸發 ECS API request。Prometheus 的 scrape interval 可以短於 collector
interval；在下一次 collector 成功前，Prometheus 會取得相同的快取值。

### 上線階段與完成條件

| 階段 | 執行內容 | 完成條件 |
|---|---|---|
| 1. 準備 ECS | 建立監控帳號、確認 DNS/TCP 4443、取得 CA | Exporter host/Pod 能解析並連線 ECS endpoint |
| 2. 準備產物 | 從 source 建置評估版本，或驗證核准 release 的 checksum、簽章與 digest | Binary/image 與預定 commit、版本一致 |
| 3. 建立 secrets | 建立 ECS username/password 與 Inventory token | Secret 不在 Git、log、image 或一般設定檔內 |
| 4. 建立設定 | 填入 Cluster identity、endpoint、TLS、collector 與 capability policy | `-validate-config` 成功 |
| 5. 部署 | 選擇本機、Docker、Helm 或 systemd | Process/Pod 穩定執行，無重啟循環 |
| 6. Runtime 驗收 | 驗證 liveness、readiness、version、metrics 與 Inventory auth | 正式環境 readiness 為 `UP` |
| 7. Prometheus 整合 | 建立 scrape target、查詢指標、載入告警 | `up{job="dell-ecs-metrics-exporter"} == 1` |
| 8. 正式放行 | 完成相容性、容量、安全、rollback 與 reviewer gate | Release Checklist 全部完成，無 blocked/skipped gate |

### 階段 1：準備 ECS 與網路

每個 ECS Cluster 至少準備：

1. Origin-only HTTPS Management endpoint，例如
   `https://ecs-management.example.com`；設定中不可包含 path、query、fragment 或帳密。
2. 建議具備 `SYSTEM_MONITOR` role 的唯讀帳號；`SYSTEM_ADMIN` 也可通過角色檢查，
   但不符合最小權限原則。
3. Exporter 執行環境到 ECS endpoint 的 DNS、TCP 4443 與 TLS 路徑；endpoint
   明列其他 port 時才使用其他 port。
4. 簽發 ECS server certificate 的 CA chain。若企業 ECS 使用無法建立 trust
   chain 或 hostname/SAN 不匹配的自簽憑證，可明確設定 `tls.verify: false`；
   這會停用憑證身分驗證並在啟動時產生 WARN，仍建議優先使用正確 CA。
5. 若要收集 replication/recovery，準備明確的 replication group/link ID。

Exporter 啟動 collector 前會讀取 ECS 版本並選擇相容性 Profile，再以
`/user/whoami` 驗證角色。未知版本會 fail closed，不會自動套用最接近的 Profile。

### 階段 2：選擇並驗證安裝產物

`v1.0.0-rc.1` 的可下載檔案、checksums 與簽章 bundle 會發布於
[GitHub Release](https://github.com/crispkid/Dell-ECS-Metrics-Exporter/releases/tag/v1.0.0-rc.1)。
OCI image 的版本 tag 為
`ghcr.io/crispkid/dell-ecs-metrics-exporter:1.0.0-rc.1`；部署時應在驗證後改 pin
Release 頁面記錄的 immutable digest。若尚未取得發布產物，也可從 source 建置：

```bash
./scripts/check-toolchain.sh
./scripts/build.sh
```

建置結果包含 `dist/ecs-exporter` 與唯讀相容性驗證工具
`dist/ecs-flux-probe`；正式 release archive 也包含兩個同 commit binary。

正式上線應使用內部核准或 release workflow 產生的不可變產物：

- Linux 使用已驗證 checksum、Sigstore bundle 與 provenance 的 release archive。
- Docker/Kubernetes 使用 immutable OCI image digest，不使用 `latest` 或可變 tag。
- Helm 使用與 application image 相同 release/commit 的 Chart package。

驗證 release 產物的命令見[發布產物驗證](#發布產物驗證)。RC 與從 source 建置都不
代表已獲准正式上線；正式環境仍須完成 Production Release Checklist。

### 階段 3：建立 secrets

本機評估可建立不會提交到 Git 的檔案：

```bash
umask 077
mkdir -p .local-secrets
printf '%s\n' 'ecs-monitor-user' > .local-secrets/username
openssl rand -hex 32 > .local-secrets/inventory-token
bash -c 'umask 077; read -r -s -p "ECS password: " ecs_password; printf "%s" "$ecs_password" > .local-secrets/password; printf "\n"'
chmod 600 .local-secrets/*
```

三個值的用途不同：

| Secret | 使用者 | 用途 |
|---|---|---|
| ECS username | Exporter | 登入 ECS |
| ECS password | Exporter | 登入 ECS |
| Inventory token | Inventory client；metrics 受保護時也供 Prometheus 使用 | 呼叫 Exporter API |

Production 請使用 Kubernetes Secret、企業 Secret Manager 或 systemd 受控檔案。
不要把 password/token 放進 Git、Helm values、container image、shell argument 或
未遮罩的 CI artifact。

### 階段 4：建立與驗證設定

若先採用內建預設值，最小設定可以保持簡短：

```yaml
security:
  inventoryApi:
    tokenFile: .local-secrets/inventory-token

ecs:
  clusters:
    - name: primary-ecs
      site: primary-dc
      environment: production
      endpoint: https://ecs-management.example.com
      usernameFile: .local-secrets/username
      passwordFile: .local-secrets/password
      tls:
        verify: true
        caFile: ""
```

這會沿用 `:8080`、`/metrics`、15 分鐘 stale tolerance、1 小時 max stale、
預設 retry/rate limit 及 collector intervals。使用 private CA 時將 `caFile`
指向 process、container 或 Pod 內實際可讀的檔案。完整範例與所有調整項目見
[完整設定說明](#5-完整設定說明)。

啟動前執行離線驗證：

```bash
./dist/ecs-exporter \
  -config config.yaml \
  -profiles-dir profiles \
  -validate-config
```

這一步驗證 YAML、secret/CA file、Profile 與欄位約束，但不連線 ECS。成功後仍須
透過 runtime readiness 與 live certification 證明真實連線及 response contract。

### 階段 5：部署 Exporter

依執行環境選擇一種方式，不需要全部執行：

| 環境 | 建議方式 | 詳細步驟 |
|---|---|---|
| 開發、相容性評估 | 本機 binary | [本機快速開始](#3-本機快速開始) |
| 本機視覺化與 scrape 測試 | Docker Prometheus + Grafana | [本機 Docker Prometheus 與 Grafana 測試](#本機-docker-prometheus-與-grafana-測試) |
| 已有 Docker/OCI runtime | Read-only container | [Docker 安裝](#7-docker-安裝) |
| Kubernetes | Helm Chart + existing Secret | [Helm 安裝](#8-helm-安裝) |
| Linux VM/實體機 | Hardened systemd service | [Bare Metal/systemd 安裝](#9-bare-metalsystemd-安裝) |

Production 預設維持單一 replica。每個 replica 都會獨立輪詢 ECS；在 target-scale
測試與 ECS API 負載未核准前，不要只為高可用而增加 replica 數。

### 階段 6：Runtime 驗收

部署後依序檢查：

```bash
export ECS_EXPORTER_URL=http://127.0.0.1:8080

curl --fail --silent --show-error "${ECS_EXPORTER_URL}/health"
curl --silent --show-error "${ECS_EXPORTER_URL}/api/v1/version"
curl --silent --show-error "${ECS_EXPORTER_URL}/api/v1/health"
curl --fail --silent --show-error "${ECS_EXPORTER_URL}/metrics" |
  rg '^(ecs_exporter_build_info|ecs_exporter_cache_age_seconds|ecs_cluster_health)'
```

再驗證 Inventory authentication：

```bash
inventory_token="$(tr -d '\r\n' < .local-secrets/inventory-token)"
curl --fail --silent --show-error \
  -H "Authorization: Bearer ${inventory_token}" \
  "${ECS_EXPORTER_URL}/api/v1/clusters"
unset inventory_token
unset ECS_EXPORTER_URL
```

驗收判讀：

- `/health` HTTP 200 只代表 process 存活。
- 啟動初期 `/api/v1/health` 回 HTTP 503／`DOWN` 是正常的，因必要快取尚未初始化。
- 所有必要 collector 成功後，正式環境必須為 HTTP 200／`UP`。
- `DEGRADED` 仍能提供舊快取，但代表 collector error 或 stale data，需要調查；
  除非是文件明列的測試例外，不可作為 production 放行結果。
- `/api/v1/version` 的版本、commit 與 Profile 必須和核准的部署紀錄一致。
- 未帶 token 或帶錯 token 的 Inventory request 必須回 HTTP 401。

目標 ECS build 的唯讀 live smoke procedure 見
[使用真實 ECS 執行唯讀相容性檢查](#使用真實-ecs-執行唯讀相容性檢查)。

### 階段 7：接入 Prometheus 與告警

非 Kubernetes 或未使用 Prometheus Operator 時，在 `prometheus.yml` 加入：

```yaml
scrape_configs:
  - job_name: dell-ecs-metrics-exporter
    scrape_interval: 30s
    scrape_timeout: 10s
    metrics_path: /metrics
    static_configs:
      - targets:
          - ecs-exporter.example.com:8080
```

先用 `promtool check config` 驗證 Prometheus 設定，再依既有部署方式 reload/restart。
在 Prometheus Targets 頁面或 API 確認 target 為 `UP`，並執行：

```promql
up{job="dell-ecs-metrics-exporter"}
```

```promql
max by (cluster, collector) (ecs_exporter_cache_age_seconds)
```

```promql
increase(ecs_exporter_collector_errors_total[15m])
```

若設定 `prometheus.protected: true`，Prometheus 必須使用 Inventory token 或經過受信任
reverse proxy；token 應由 Prometheus 的 secret file 機制提供，不要直接寫入
`prometheus.yml`。目前 Helm Chart 的 ServiceMonitor 不會注入 Bearer token，
因此 Kubernetes 預設採未受 HTTP authentication 保護的 `/metrics`，並以
NetworkPolicy/內部網路限制來源；若安全政策要求 HTTP authentication，需改用受控的
額外 scrape 設定、reverse proxy，或先擴充 Chart 契約。

Prometheus Operator 環境將 `serviceMonitor.enabled` 設為 `true`，並以
`serviceMonitor.additionalLabels` 符合 Prometheus 的 `serviceMonitorSelector`。
Operator 產生的 `job` label 可能不是 `dell-ecs-metrics-exporter`；請以實際 label
同步調整告警 selector，避免 Exporter 正常但 `DellECSExporterDown` 誤報。

載入告警範例：

```yaml
rule_files:
  - /etc/prometheus/rules/dell-ecs-metrics-exporter.yaml
```

將 `deploy/prometheus/alerts.yaml` 放到上述路徑，執行：

```bash
promtool check rules deploy/prometheus/alerts.yaml
```

再 reload Prometheus，確認規則群組 `dell-ecs-metrics-exporter` 已載入。告警門檻以
預設 `staleTolerance: 15m`、`maxStale: 1h` 為基礎；修改 cache policy 時必須同步
調整規則。

### 階段 8：正式放行與持續使用

正式上線前至少確認：

- [ ] 使用 immutable digest 或已驗證 checksum/signature 的 release archive。
- [ ] 實際 ECS exact build 已依政策完成 Profile 與 API contract 驗證。
- [ ] ECS TLS verification 已開啟並驗證 CA/hostname/SAN/Host/SNI/proxy，或
  `tls.verify: false` 自簽憑證例外已獲核准、記錄風險並限制管理網路路徑。
- [ ] ECS 帳號採最小權限，所有 secret 都由核准的 secret mechanism 提供。
- [ ] `/health`、`/api/v1/health`、`/api/v1/version`、`/metrics` 與 Inventory auth 通過。
- [ ] Prometheus target 為 `UP`，cache age、collector/API error 與 ECS domain metrics 可查詢。
- [ ] 告警已載入並測試通知路徑，job label 與實際 scrape labels 一致。
- [ ] 資源限制、cardinality、scrape size、ECS API 負載與 target-scale 結果可接受。
- [ ] 設定、CA、alert rules 與 secret reference 已納入備份；記憶體 cache 不需備份。
- [ ] Credential/CA rotation、restart、rollback 與事故處理已演練。
- [ ] [Production Release Checklist](docs/RELEASE_CHECKLIST.md) 無未完成、
      blocked 或 skipped gate，並取得必要 reviewer 核准。

上線後以 `ecs_exporter_cache_age_seconds`、`ecs_exporter_collector_errors_total`、
`ecs_exporter_api_errors_total` 與 `up` 監控 Exporter 本身，再使用 `ecs_cluster_*`、
`ecs_node_*`、`ecs_namespace_*`、`ecs_bucket_*` 等指標監控 ECS。事故處理、
安全 restart/rollback、credential/CA rotation 與災難復原見
[Production Runbook](docs/PRODUCTION_RUNBOOK.md)。

## 功能摘要

- 每個 ECS Cluster 使用獨立 credential、token、TLS transport、timeout、retry、
  circuit breaker、collector state 與快取。
- 收集 Cluster、Node、Namespace、Bucket、VDC/Namespace Performance、Replication
  與 Recovery 資料；conditional capability 必須由每個 Cluster 明確啟用。
- 支援 ECS login、token header/cookie、首次 HTTP 401 自動重新登入及 logout。
- 支援 HTTP 429、500、502、503、504 retry、`Retry-After`、exponential backoff、
  initial schedule jitter、request timeout、per-cluster request rate/burst 與並行限制。
- Bucket inventory 先列出 Namespace，再以 Namespace scope 執行 marker pagination
  與有限並行 quota enrichment；billing 優先使用 Namespace batch POST，明確不支援
  或缺項時 fallback 單筆 GET。任何必要 request 失敗時保留上一份完整快取。
- 提供 Prometheus domain metrics 與 `ecs_exporter_*` 自我監控 metrics。
- 提供 Bearer token 或 trusted reverse proxy 保護的唯讀 Inventory API。
- 提供 process liveness、cache readiness、`UP`／`DEGRADED`／`DOWN` 狀態。
- 提供 non-root scratch image、Helm Chart、NetworkPolicy、PDB、optional
  ServiceMonitor，以及 hardened Linux systemd Bare Metal 部署。

四個 Profile 都沒有經確認的 Bucket-scope performance mapping，因此不會輸出
Bucket throughput、latency、request 或 HTTP status metrics。Node disk metric 也會在
未設定且驗證 filesystem/device allowlist 的情況下保持缺席。

ECS CE 3.8.0.3 的 non-empty follow-up 證實 Bucket quota 與單筆 billing 可能使用
top-level fields；Exporter 同時保留既有 nested envelope，且拒絕同時出現兩種
envelope 的模糊 response。Billing API 的 `KB` 依已知 10,000,000-byte dataset
驗證為 1024 bytes。此結果不會外推到 billing MB/GB/TB 或 capacity/quota GB；
修正後 live rerun 已取得三個 Bucket、四個 objects 與 10,000,000 bytes 的正確
Inventory/metrics。這仍不代表整個 `ecs-3.8.0` Profile 已認證。

同一 follow-up 顯示 batch billing 對 no JSON entity、`{}`、`[]`、`null` 分別回
HTTP 415、HTTP 200 + plural empty envelope、HTTP 400/code1013、HTTP 500/code999。
ECS CE 內建 class 與 non-empty live probe 已確認 request body 是
`{"id":[bucket names...]}`，response 使用 plural `bucket_billing_infos`。
Fallback 僅適用 HTTP 404/405/501 或 batch 缺少 requested Bucket；generic 500
與 500/code999 不 fallback。

ECS CE 3.8.1.4 的 known-size follow-up 也驗證相同 top-level quota/plural batch
contract：三個 Buckets 合計四個 objects、10,485,760 bytes，Billing 回
`10240 KB`；Exporter Inventory 與 metrics 精確一致。Exact build
`3.8.1.4.140200.8103892f11b` 正確選到 `ecs-3.8.1`，但 CE 的 Flux external query
仍回 HTTP 503，因此 CPU/memory/network、VDC/Namespace performance 與 range
boundary gate 尚未通過，不能視為正式設備 Profile 認證。

ECS CE 3.7.0.0（exact build `3.7.0.0.137697.a664876f8ed`）與 3.6.2.0
（exact build `3.6.2.0.127497.982f3bd4450`）也以相同的 known-size dataset
完成 follow-up：四個 objects 合計 10,485,760 bytes，Bucket 分布為
7,340,032 bytes／3 objects、3,145,728 bytes／1 object 與空 Bucket；
1/2 GB 的正值 soft/hard quota 亦正確映射。3.7.0.0 的 Flux
`node-resources` 回 HTTP 503；3.6.2.0 的 `node-resources` 與 `performance`
都回 HTTP 503。這些結果證明兩個 build 的 Management-backed metrics 可用，
但不證明 Flux-backed metrics、整個 Profile 或正式設備已認證。

## 1. 選擇安裝方式

| 使用情境 | 建議方式 | 章節 |
|---|---|---|
| 開發、驗證或單機執行 | 本機編譯 binary | [本機快速開始](#3-本機快速開始) |
| 已有 Docker／OCI runtime | Docker image | [Docker 安裝](#7-docker-安裝) |
| Kubernetes production-like 環境 | Helm Chart | [Helm 安裝](#8-helm-安裝) |
| Linux 實體機或 VM | Bare Metal/systemd | [Bare Metal 安裝](#9-bare-metalsystemd-安裝) |

第一次使用建議先完成本機快速開始，確認 ECS endpoint、帳號權限、TLS CA 與版本
Profile 都正確，再轉成 Docker 或 Helm 部署。

## 2. 前置條件

### 2.1 ECS 端

請先準備：

1. 一個隔離或已授權監控的 Dell ECS Management endpoint。
2. 可解析該 endpoint hostname 的 DNS。
3. 到 TCP 4443 的網路連線；若 endpoint 已明列其他 port，Exporter 會使用該 port。
4. 每個 Cluster 一組唯讀帳號，建議使用 `SYSTEM_MONITOR` role。
5. ECS HTTPS certificate 的 CA certificate；若已由作業系統信任，可不另設 `caFile`。
6. 若要收集 replication/recovery，準備要查詢的 replication group/link ID。

Exporter 啟動 collector 前會呼叫 `/vdc/nodes` 選擇 Profile，再呼叫
`/user/whoami` 確認帳號具有 `SYSTEM_MONITOR` 或 `SYSTEM_ADMIN`。未知版本會被拒絕，
不會自動退回相近版本。

### 2.2 本機工具

從 source 編譯需要：

- Go 1.26.5
- Git
- Bash 3.2+
- `curl`
- `openssl`（產生 Inventory API token）
- Helm 3+（只有 Helm 安裝或 deployment check 需要）
- Docker 或相容 OCI builder（只有 container 安裝需要）
- `kubectl` 與可用的 Kubernetes cluster（只有 Helm 安裝需要）
- systemd-based Linux amd64/arm64（只有 Bare Metal 安裝需要）

確認 repository 的 Go/Helm 環境：

```bash
./scripts/check-toolchain.sh
```

## 3. 本機快速開始

以下命令都在 repository 根目錄執行。

### 步驟 1：建立不會提交到 Git 的 secret files

```bash
umask 077
mkdir -p .local-secrets
printf '%s\n' 'ecs-monitor-user' > .local-secrets/username
openssl rand -hex 32 > .local-secrets/inventory-token
bash -c 'umask 077; read -r -s -p "ECS password: " ecs_password; printf "%s" "$ecs_password" > .local-secrets/password; printf "\n"'
chmod 600 .local-secrets/*
```

將 `ecs-monitor-user` 改成實際 ECS 帳號。`.local-secrets/` 已列入 `.gitignore`。
Inventory token 是使用者呼叫 Exporter Inventory API 時使用的 token，並不是 ECS
回傳的 authentication token。

### 步驟 2：建立設定檔

```bash
cp config.example.yaml config.yaml
```

編輯 `config.yaml`，至少修改：

```yaml
ecs:
  clusters:
    - name: primary-ecs
      site: primary-dc
      environment: production
      endpoint: https://ecs-management.example.com
      usernameFile: .local-secrets/username
      passwordFile: .local-secrets/password
      tls:
        verify: true
        caFile: ""
```

- `name` 必須在所有 Cluster 中唯一。
- `endpoint` 必須是 origin-only HTTPS URL，不可包含 path、query、fragment 或帳密。
- 未指定 port 時自動使用 `4443`。
- ECS certificate 已由系統 trust store 信任時，`caFile` 保持空字串。
- 使用 private CA 時，將 CA 複製到已忽略的 `certs/` 目錄並設定路徑：

```bash
mkdir -p certs
cp /absolute/path/to/ecs-ca.pem certs/ecs-ca.pem
chmod 644 certs/ecs-ca.pem
```

```yaml
tls:
  verify: true
  caFile: certs/ecs-ca.pem
```

企業自簽憑證無法透過 `caFile` 驗證時，可明確設定：

```yaml
tls:
  verify: false
  caFile: ""
```

此設定在 `environment: production` 也允許，但會同時停用憑證鏈與
hostname/SAN 驗證。Exporter 啟動時會以 cluster/environment 記錄 WARN，且不記錄
endpoint。請限制 Exporter 到 ECS 的網路路徑並將此選擇納入風險審查；預設與建議值
仍是 `verify: true`。

### 步驟 3：選擇 replication/recovery 目標

不需要 replication/recovery 時保留空陣列：

```yaml
replication:
  groups: []
  links: []
```

需要時填入 ECS 實際 ID：

```yaml
replication:
  groups:
    - replication-group-id
  links:
    - replication-link-id
```

目前沒有經真機確認的安全 list endpoint，因此 Exporter 只查詢明列的 ID，不會自行
猜測 discovery URI。

### 步驟 4：編譯

```bash
./scripts/build.sh
```

成功時會建立 `dist/ecs-exporter`，並輸出：

```json
{"profiles":"ecs-3.6,ecs-3.7,ecs-3.8.0,ecs-3.8.1","status":"valid"}
```

第一次執行可能需要連線下載 `go.mod` 中固定版本的 dependencies。

### 步驟 5：啟動前驗證設定

```bash
./dist/ecs-exporter \
  -config config.yaml \
  -profiles-dir profiles \
  -validate-config
```

成功範例：

```json
{"clusters":1,"profiles":"ecs-3.6,ecs-3.7,ecs-3.8.0,ecs-3.8.1","status":"valid"}
```

此命令只驗證設定、secret files 與 Profile，不會連線 ECS，也不會啟動 HTTP server。

### 步驟 6：啟動 Exporter

```bash
./dist/ecs-exporter \
  -config config.yaml \
  -profiles-dir profiles
```

Exporter 會在 stderr 輸出 JSON logs。保持此 terminal 執行，另外開一個 terminal
進行下列確認。

### 步驟 7：確認可以使用

Process liveness：

```bash
curl -i http://127.0.0.1:8080/health
```

應回傳 HTTP 200：

```json
{"component":"process","status":"UP"}
```

版本與 Profile：

```bash
curl -sS http://127.0.0.1:8080/api/v1/version
```

Readiness：

```bash
curl -i http://127.0.0.1:8080/api/v1/health
```

啟動初期在 Cluster、Node、Namespace、Bucket 快取尚未全部初始化前，HTTP 503／
`DOWN` 是預期行為。全部必要 collector 成功後應變成 HTTP 200／`UP`；仍有可服務
快取但資料過期或 collector 發生錯誤時會是 HTTP 200／`DEGRADED`。

Prometheus metrics：

```bash
curl -sS http://127.0.0.1:8080/metrics
```

Inventory API：

```bash
inventory_token="$(tr -d '\r\n' < .local-secrets/inventory-token)"
curl -sS \
  -H "Authorization: Bearer ${inventory_token}" \
  http://127.0.0.1:8080/api/v1/clusters
unset inventory_token
```

到這裡即完成本機安裝、設定、啟動與基本使用驗證。

### 使用真實 ECS 執行唯讀相容性檢查

完成快速開始並確認 Exporter 可連線 ECS 後，可用內建 smoke gate 重跑登入、
版本選擇、Inventory、quota 語意與必要 metrics。此腳本只讀取 ECS，不會建立或
刪除 Namespace、Bucket 或 object；需要 `curl`、`jq`、Go、Git，以及乾淨且已完整
提交的工作樹，讓證據能綁定確切的 commit。

先確認本機設定與 secrets 都被 Git 忽略：

```bash
git check-ignore -v \
  config.local.yaml \
  .local-secrets/username \
  .local-secrets/password \
  .local-secrets/inventory-token
```

再執行目標版本檢查：

```bash
ECS_CERT_CONFIG=config.local.yaml \
ECS_CERT_INVENTORY_TOKEN_FILE=.local-secrets/inventory-token \
ECS_CERT_EXPECTED_VERSION=3.8.1.4 \
ECS_CERT_BASE_URL=http://127.0.0.1:8080 \
./scripts/live-certification.sh
```

`ECS_CERT_BASE_URL` 必須是 loopback URL，port 要與
`config.local.yaml` 的 `server.listenAddress` 一致；上例沿用快速開始的 8080。

成功時會輸出：

```text
read-only live certification smoke passed; evidence is partial-live, not formal Profile certification
```

去識別化摘要會寫入 `test-results/certification/result.json`；`test-results/` 已被
`.gitignore` 排除。原始 ECS response、endpoint、Inventory 名稱與 credential
不會寫入該摘要。

ECS CE 3.7.0.0、3.8.0.3 與 3.8.1.4 的 Flux external query 在目前實測環境回
HTTP 503。如果確認唯一的正值 collector error 是 `node-resources`，可在
**CE 相容性測試**明確加入：

```bash
ECS_CERT_CONFIG=config.local.yaml \
ECS_CERT_INVENTORY_TOKEN_FILE=.local-secrets/inventory-token \
ECS_CERT_EXPECTED_VERSION=3.8.1.4 \
ECS_CERT_BASE_URL=http://127.0.0.1:8080 \
ECS_CERT_ALLOW_DEGRADED=true \
./scripts/live-certification.sh
```

`ECS_CERT_ALLOW_DEGRADED=true` 不可用於正式設備認證；正式 ECS 3.8.1.4 必須達到
`UP`，並另外完成 Flux、multi-VDC、proxy/Host/SNI、token renewal、failure
injection 與 target-scale gates。詳細證據邊界見
[ECS CE 3.8.1.4 Live Validation Record](docs/ecs-api/validation/ecs-ce-3.8.1.4-2026-07-26.md)。

ECS CE 3.6.2.0 實測同時出現 `node-resources` 與 `performance` collector error；
現有 gate 只允許前者，因此即使設定 `ECS_CERT_ALLOW_DEGRADED=true` 仍會失敗。
這個失敗不影響已個別通過的 Management-backed metrics，但也不可擴大解讀為完整
3.6 Profile 相容。正式變更 allowlist 或 capability 前，必須先取得官方／實體設備
Flux evidence，不能只依 CE 503 放寬認證。

## 4. CLI 參數與環境變數

### CLI

| 參數 | 預設值 | 用途 |
|---|---|---|
| `-config` | `config.yaml` | YAML 設定檔 |
| `-profiles-dir` | `profiles` | Compatibility Profile 目錄 |
| `-validate-config` | `false` | 驗證設定與 Profile 後結束 |
| `-validate-profiles` | `false` | 只驗證 Profile 後結束 |

顯示內建 flag help：

```bash
./dist/ecs-exporter -help
```

### 環境變數

| 變數 | 用途 |
|---|---|
| `ECS_EXPORTER_CONFIG` | 覆寫 `-config` 預設值 |
| `ECS_EXPORTER_PROFILES_DIR` | 覆寫 `-profiles-dir` 預設值 |
| `ECS_EXPORTER_LISTEN_ADDRESS` | 覆寫 `server.listenAddress` |
| `ECS_EXPORTER_INVENTORY_TOKEN_FILE` | 覆寫 Inventory Bearer token file |
| `ECS_EXPORTER_MAX_PAGE_SIZE` | 覆寫 Inventory API 最大 page size |

YAML 也支援 `${ENV_NAME}` 展開。被引用的環境變數不存在時，設定驗證會直接失敗。
Credential 建議使用 file reference；若使用環境變數，`username`／`usernameFile` 及
`password`／`passwordFile` 各自只能設定其中一個：

```yaml
username: ${ECS_USERNAME}
usernameFile: ""
password: ${ECS_PASSWORD}
passwordFile: ""
```

## 5. 完整設定說明

### Server 與 Prometheus

```yaml
server:
  listenAddress: ":8080"
  tls:
    certFile: ""
    keyFile: ""

prometheus:
  path: /metrics
  protected: false
```

- `server.tls` 是 Exporter 對 client 提供 HTTPS 的 certificate/key，與連線 ECS 使用的
  `ecs.clusters[].tls` 不同。
- `certFile` 與 `keyFile` 必須同時設定。
- `prometheus.path` 必須是未與 `/health`、`/api/v1` 衝突的絕對路徑。
- `prometheus.protected: true` 會讓 metrics 使用與 Inventory API 相同的認證。

### Cache

```yaml
cache:
  staleTolerance: 15m
  maxStale: 1h
```

- 超過 `staleTolerance` 的必要快取會使 readiness 變成 `DEGRADED`。
- 超過 `maxStale` 會使該 Cluster 變成 `DOWN`。
- 超過 `maxStale` 的 collector domain 不再出現在 Prometheus domain metrics；Exporter
  build/Profile、cache age、last-success 與 cached-resource metrics 仍會保留。
- `maxStale` 不得小於 `staleTolerance`。

### Collector

```yaml
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
```

- `maxAttempts` 可設定 1～10。
- `requestsPerSecond` 是每個 Cluster 共用於 login/logout 與 Management/Flux request
  的 token-bucket rate；`burst` 是可立即使用的 token 數。
- `jitterRatio` 可設定 0～0.5，首次排程與後續 interval 都會套用。
- `maxConcurrentRequestsPerCluster` 可設定 1～64。
- `bucketPageSize` 可設定 1～10,000；實際 pagination contract 仍需在目標 ECS build
  驗證。

每個 Cluster 可覆寫 timeout 與 interval：

```yaml
timeouts:
  connect: 10s
  read: 30s
  overall: 30s
intervals:
  node: 90s
  bucket: 10m
```

`connect` 與 `read` 不得超過 `overall`。未設定的 per-cluster 欄位會沿用全域預設。

每個 Cluster 可明確啟用已在 Profile 標為 `conditional` 的能力：

```yaml
capabilities:
  enabledConditional:
    - vdc_performance
    - namespace_performance
    - node_service_process
    - node_disk_capacity
    - recovery_progress
nodeResources:
  filesystems:
    - /data
  networkInterfaces:
    - bond0
  maxNetworkInterfaces: 16
  preferBondInterfaces: true
```

可用名稱為 `vdc_performance`、`namespace_performance`、`node_service_process`、
`node_disk_capacity`、`recovery_progress`。只有 Profile
本身為 `conditional` 時設定才有效，`unavailable` 永遠不會被覆寫；mixed-version
仍停用 interval-derived rates。`flux_interval_rates` 是 Profile 內部相容性 guard，
V1 沒有可由設定啟用的 interval-derived public metric。啟用 `node_disk_capacity`
時 `filesystems` 不可空。
`networkInterfaces` 留空表示接受最多 `maxNetworkInterfaces` 個介面；若同時存在
bond 與常見 member interface，預設只保留 bond，避免重複計數。

### Inventory API 安全設定

Bearer token 模式：

```yaml
security:
  inventoryApi:
    enabled: true
    authentication: token
    tokenFile: .local-secrets/inventory-token
    proxyHeader: X-Remote-User
    trustedProxyCIDRs: []
    maxPageSize: 500
```

Token file 每個 request 都會重新讀取，因此可以原子替換檔案完成 token rotation，
不必重新啟動 Exporter。Token 必須包含 16～4096 個非空白字元。

Trusted reverse proxy 模式：

```yaml
security:
  inventoryApi:
    enabled: true
    authentication: proxy
    tokenFile: ""
    proxyHeader: X-Remote-User
    trustedProxyCIDRs:
      - 192.0.2.0/24
      - 2001:db8:100::/64
    maxPageSize: 500
```

只有 request 的直接來源 IP 位於 `trustedProxyCIDRs`，且 `proxyHeader` 非空時才會通過。
Reverse proxy 必須移除 client 自行傳入的同名 header，再寫入已驗證 identity。不要在
不受信任網路使用 `enabled: false`。

### 多 Cluster

在 `ecs.clusters` 加入多筆設定即可：

```yaml
ecs:
  clusters:
    - name: primary-ecs
      site: taipei
      environment: production
      endpoint: https://ecs-a.example.com
      usernameFile: .local-secrets/ecs-a-username
      passwordFile: .local-secrets/ecs-a-password
      tls:
        verify: true
        caFile: certs/ecs-ca.pem
      replication:
        groups: []
        links: []

    - name: dr-ecs
      site: kaohsiung
      environment: production
      endpoint: https://ecs-b.example.com:4443
      usernameFile: .local-secrets/ecs-b-username
      passwordFile: .local-secrets/ecs-b-password
      tls:
        verify: true
        caFile: certs/ecs-ca.pem
      replication:
        groups: []
        links: []
```

每個 Cluster 的 name、credential、token、TLS client、retry/circuit、cache 與
collector state 都互相隔離。

## 6. Prometheus 與 Inventory API

### HTTP endpoints

| Method | Path | 預設認證 | 說明 |
|---|---|---|---|
| GET | `/health` | 無 | Process liveness |
| GET | `/api/v1/health` | 無 | Cache/readiness |
| GET | `/api/v1/version` | 無 | Build、Profile 與 certification 狀態 |
| GET | `/metrics` | 無 | Prometheus exposition；可設為受保護 |
| GET | `/api/v1/clusters` | Inventory auth | Cluster 清單 |
| GET | `/api/v1/clusters/{cluster}` | Inventory auth | 單一 Cluster |
| GET | `/api/v1/nodes` | Inventory auth | Node 清單 |
| GET | `/api/v1/namespaces` | Inventory auth | Namespace 清單 |
| GET | `/api/v1/buckets` | Inventory auth | Bucket 清單 |
| GET | `/api/v1/replications` | Inventory auth | Replication/recovery 清單 |

Collection endpoints 支援：

- `page`：從 0 開始的 page index。
- `size`：1 到 `security.inventoryApi.maxPageSize`。
- `sort`：`name`、`cluster`、`namespace`，前置 `-` 表示 descending。
- `cluster`、`namespace`、`name`：依 endpoint 適用的精確比對。

範例：

```bash
inventory_token="$(tr -d '\r\n' < .local-secrets/inventory-token)"

curl -sS \
  -H "Authorization: Bearer ${inventory_token}" \
  "http://127.0.0.1:8080/api/v1/buckets?cluster=primary-ecs&namespace=finance&page=0&size=100&sort=name"

unset inventory_token
```

Collection response envelope：

```json
{
  "items": [],
  "page": 0,
  "size": 100,
  "totalElements": 0,
  "totalPages": 0,
  "collectedAt": null
}
```

無來源資料的 optional 欄位會是 `null`。未設定 quota 時 quota metric 不會出現，
Inventory 則回傳 `null` 並將對應 `*QuotaConfigured` 設為 `false`。

### Prometheus scrape

預設 metrics 未受保護：

```yaml
scrape_configs:
  - job_name: dell-ecs-metrics-exporter
    scrape_interval: 30s
    scrape_timeout: 10s
    metrics_path: /metrics
    static_configs:
      - targets:
          - ecs-exporter.example.com:8080
```

若設定 `prometheus.protected: true`，Prometheus 必須送出相同的 Bearer token，或經過
已設定的 trusted reverse proxy。`deploy/prometheus/alerts.yaml` 的 Exporter down
規則預設比對 `job="dell-ecs-metrics-exporter"`；若使用其他 job name 或
Prometheus Operator 產生不同 label，必須同步調整告警 selector。

### 主要 metrics

- Cluster：`ecs_cluster_health`、`ecs_cluster_capacity_*_bytes`、
  `ecs_cluster_buckets`、`ecs_cluster_namespaces`、`ecs_cluster_objects`
- Node：`ecs_node_health`、`ecs_node_cpu_usage_ratio`、`ecs_node_memory_*_bytes`、
  `ecs_node_disk_*_bytes`、`ecs_node_service_health`、
  `ecs_node_network_receive_bytes_total`、`ecs_node_network_transmit_bytes_total`
- Namespace：`ecs_namespace_capacity_used_bytes`、`ecs_namespace_quota_bytes`、
  `ecs_namespace_buckets`、`ecs_namespace_objects`
- Bucket：`ecs_bucket_used_bytes`、`ecs_bucket_soft_quota_bytes`、
  `ecs_bucket_hard_quota_bytes`、`ecs_bucket_objects`
- Performance：VDC throughput/latency/request-rate Gauge，以及
  `ecs_namespace_requests` request-rate Gauge；ECS 文件與 3.8.1.1 實測都沒有
  Namespace throughput/latency mapping，也不提供 Bucket-scope performance
- Replication/recovery：`ecs_replication_status`、`ecs_replication_lag_seconds`、
  `ecs_recovery_progress_ratio`
- Exporter：`ecs_exporter_api_*`、`ecs_exporter_collector_*`、
  `ecs_exporter_cache_*`、`ecs_exporter_last_success_timestamp_seconds`、
  `ecs_exporter_api_response_size_bytes`、`ecs_exporter_build_info`、
  `ecs_exporter_profile_contract_info`

完整名稱、型別、unit 與 label contract 請參閱
[SPECIFICATION.md](SPECIFICATION.md) 第 9 節。

### 本機 Docker Prometheus 與 Grafana 測試

Repository 提供獨立的本機觀測 stack，讓 Prometheus 擷取 host port 8080 上的
Exporter，並讓 Grafana 自動使用該 Prometheus datasource。Stack 不包含 ECS
credential，也不會部署 Exporter；Exporter 可使用本機 binary，或另一個已將
container port 8080 發布到 host port 8080 的 container。

先確認 Exporter：

```bash
curl --fail --silent --show-error http://127.0.0.1:8080/health
curl --fail --silent --show-error http://127.0.0.1:8080/metrics |
  rg '^ecs_exporter_build_info'
```

再從 repository 根目錄啟動：

```bash
docker compose -f deploy/local/compose.yaml config --quiet
docker compose -f deploy/local/compose.yaml pull
docker compose -f deploy/local/compose.yaml up -d
docker compose -f deploy/local/compose.yaml ps
```

預設服務：

| Service | URL | 說明 |
|---|---|---|
| Prometheus | <http://127.0.0.1:9090> | 保留七天本機測試資料 |
| Prometheus targets | <http://127.0.0.1:9090/targets> | Exporter target 應為 `UP` |
| Grafana | <http://127.0.0.1:3000> | 匿名 Viewer；可暫時使用 Explore、不可儲存；只綁定 loopback |

Prometheus 每 15 秒抓取
`host.docker.internal:8080/metrics`，Compose 同時建立 Linux `host-gateway`
mapping。Grafana 已 provision `http://prometheus:9090` 為預設 datasource；開啟
**Explore** 即可執行本章的 PromQL。V1 仍不交付 Grafana Dashboard JSON 或
Dashboard provisioning。

驗證整合：

```bash
curl --fail --silent --show-error --get \
  --data-urlencode 'query=up{job="dell-ecs-metrics-exporter"}' \
  http://127.0.0.1:9090/api/v1/query |
  jq .
```

結果 value 應為 `1`。停止並保留資料：

```bash
docker compose -f deploy/local/compose.yaml down
```

完整的設定驗證、自訂 ports、Grafana Explore 查詢、故障排除與資料清除說明見
[Local Prometheus and Grafana test stack](deploy/local/README.md)。

## 7. Docker 安裝

Release Candidate 可先拉取版本化 image：

```bash
docker pull ghcr.io/crispkid/dell-ecs-metrics-exporter:1.0.0-rc.1
```

若 registry visibility 尚未對目前帳號開放，或要驗證未發布的 source commit，才自行
建置：

```bash
docker build -t dell-ecs-metrics-exporter:dev .
```

建立 container 專用設定：

```bash
cp config.example.yaml config.container.yaml
```

將其中的 file paths 改成 container 內路徑：

```yaml
security:
  inventoryApi:
    tokenFile: /var/run/secrets/ecs-exporter/inventory-token

ecs:
  clusters:
    - name: primary-ecs
      site: primary-dc
      environment: production
      endpoint: https://ecs-management.example.com
      usernameFile: /var/run/secrets/ecs-exporter/username
      passwordFile: /var/run/secrets/ecs-exporter/password
      tls:
        verify: true
        caFile: ""
```

若使用 private CA，另設 `caFile: /etc/ecs-exporter/certs/ecs-ca.pem` 並加入 CA mount。

執行：

```bash
docker run --rm \
  --name dell-ecs-metrics-exporter \
  --read-only \
  --user "$(id -u):$(id -g)" \
  -p 8080:8080 \
  --mount type=bind,src="$PWD/config.container.yaml",dst=/etc/ecs-exporter/config.yaml,readonly \
  --mount type=bind,src="$PWD/.local-secrets",dst=/var/run/secrets/ecs-exporter,readonly \
  ghcr.io/crispkid/dell-ecs-metrics-exporter:1.0.0-rc.1
```

Private CA 額外加入：

```text
--mount type=bind,src="$PWD/certs",dst=/etc/ecs-exporter/certs,readonly
```

Image 內建 `/profiles`，預設 command 會使用：

```text
-config=/etc/ecs-exporter/config.yaml
-profiles-dir=/profiles
```

使用本機的 [步驟 7](#步驟-7確認可以使用) 驗證 port 8080。

## 8. Helm 安裝

### 步驟 1：準備可由 Kubernetes 拉取的 image

Release Candidate image：

```bash
docker pull ghcr.io/crispkid/dell-ecs-metrics-exporter:1.0.0-rc.1
```

正式發布建議改用 immutable digest。

### 步驟 2：建立 Namespace 與既有 Secret

Chart 永遠不會建立 credential-bearing Secret：

```bash
kubectl create namespace monitoring --dry-run=client -o yaml |
  kubectl apply -f -

kubectl -n monitoring create secret generic ecs-exporter-credentials \
  --from-file=inventory-token=.local-secrets/inventory-token \
  --from-file=username=.local-secrets/username \
  --from-file=password=.local-secrets/password \
  --dry-run=client -o yaml |
  kubectl apply -f -
```

若 Secret 已存在，請使用組織的 Secret Manager／GitOps 流程更新，不要將明文 secret
寫入 values file。

### 步驟 3：建立 values override

建立 `/tmp/ecs-exporter-values.yaml`：

```yaml
image:
  repository: ghcr.io/crispkid/dell-ecs-metrics-exporter
  tag: 1.0.0-rc.1
  pullPolicy: IfNotPresent

credentials:
  existingSecret: ecs-exporter-credentials

config:
  content: |-
    server:
      listenAddress: ":8080"
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
        - name: primary-ecs
          site: primary-dc
          environment: production
          endpoint: https://ecs-management.example.com
          usernameFile: /var/run/secrets/ecs-exporter/username
          passwordFile: /var/run/secrets/ecs-exporter/password
          tls:
            verify: true
            caFile: ""
          replication:
            groups: []
            links: []

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: "1"
    memory: 512Mi

serviceMonitor:
  enabled: false
```

### 步驟 4：需要時掛入 private CA

```bash
kubectl -n monitoring create configmap ecs-exporter-ca \
  --from-file=ecs-ca.pem=/absolute/path/to/ecs-ca.pem \
  --dry-run=client -o yaml |
  kubectl apply -f -
```

在 values 加入：

```yaml
extraVolumes:
  - name: ecs-ca
    configMap:
      name: ecs-exporter-ca

extraVolumeMounts:
  - name: ecs-ca
    mountPath: /etc/ecs-exporter/certs
    readOnly: true
```

並將 `config.content` 中的 `caFile` 改成：

```yaml
caFile: /etc/ecs-exporter/certs/ecs-ca.pem
```

### 步驟 5：先 lint/render，再安裝

若不是從 source checkout 執行，先下載 RC 的 OCI Chart，並把下列命令中的
`charts/dell-ecs-metrics-exporter` 改成 `/tmp/ecs-exporter-chart/dell-ecs-metrics-exporter`：

```bash
helm pull oci://ghcr.io/crispkid/charts/dell-ecs-metrics-exporter \
  --version 1.0.0-rc.1 \
  --untar \
  --untardir /tmp/ecs-exporter-chart
```

```bash
helm lint charts/dell-ecs-metrics-exporter \
  --values /tmp/ecs-exporter-values.yaml

helm template ecs-exporter charts/dell-ecs-metrics-exporter \
  --namespace monitoring \
  --values /tmp/ecs-exporter-values.yaml

helm upgrade --install ecs-exporter charts/dell-ecs-metrics-exporter \
  --namespace monitoring \
  --values /tmp/ecs-exporter-values.yaml \
  --wait \
  --timeout 5m
```

### 步驟 6：確認 Deployment

```bash
kubectl -n monitoring rollout status deployment/ecs-exporter-dell-ecs-metrics-exporter
kubectl -n monitoring get pods,service
kubectl -n monitoring logs deployment/ecs-exporter-dell-ecs-metrics-exporter
```

本機 port-forward：

```bash
kubectl -n monitoring port-forward service/ecs-exporter-dell-ecs-metrics-exporter 8080:8080
```

再依 [步驟 7](#步驟-7確認可以使用) 呼叫 health、version、metrics 與 Inventory API。

若已安裝 Prometheus Operator，可將 `serviceMonitor.enabled` 改為 `true`。Chart 的
`networkPolicy.ingressFrom` 與 `networkPolicy.egress` 可限制 Prometheus 來源及 DNS／
ECS 目的地。Production 請從
`charts/dell-ecs-metrics-exporter/values-production.example.yaml` 複製設定，替換
image digest、Prometheus selector、DNS labels 及 ECS management CIDR。範例中的
`192.0.2.0/24` 是不會路由的文件用網段，未替換時會刻意阻止 ECS 通訊。

### 升級與移除

```bash
helm upgrade ecs-exporter charts/dell-ecs-metrics-exporter \
  --namespace monitoring \
  --values /tmp/ecs-exporter-values.yaml \
  --wait \
  --timeout 5m
```

Chart 內管理的 ConfigMap 內容變更會透過 checksum annotation 觸發 rollout。
`config.existingConfigMap` 或外部 Secret 更新後，請依變更內容執行
`kubectl -n monitoring rollout restart deployment/ecs-exporter-dell-ecs-metrics-exporter`
並重新驗證；ECS username/password 只在 process 啟動時載入。

```bash
helm uninstall ecs-exporter --namespace monitoring
```

外部建立的 Secret 與 CA ConfigMap 不會被 Helm 刪除。確認不再使用後才手動移除：

```bash
kubectl -n monitoring delete secret ecs-exporter-credentials
kubectl -n monitoring delete configmap ecs-exporter-ca
```

## 9. Bare Metal/systemd 安裝

此方式支援 systemd-based Linux amd64/arm64。Release archive 內含安裝器、systemd
unit、sysusers/tmpfiles 設定與 profiles；服務使用獨立的 `ecs-exporter` 無登入帳號，
設定放在 `/etc/dell-ecs-metrics-exporter`，binary 與 profiles 為 root-owned。
主機需有 systemd、`systemctl`、`curl`、`tar` 與標準 Linux
`install`/`find`/`useradd` 工具，安裝與移除需有 root/sudo 權限。

### 步驟 1：驗證並解壓 release

先依[發布產物驗證](#發布產物驗證)確認 checksum、簽章、provenance 與來源，再解壓
符合主機架構的 Linux archive。不要以 root 執行來路不明或未驗證的安裝器。

### 步驟 2：安裝但先不啟動

在解壓目錄執行：

```bash
sudo ./deploy/bare-metal/install.sh \
  --binary ./ecs-exporter \
  --profiles ./profiles \
  --no-start
```

安裝器會在升級時驗證新 Profiles，原子替換 profiles 目錄，並保留既有
`config.yaml` 與 `secrets/`。

### 步驟 3：安裝 secrets 與 CA

```bash
sudo install -m 0640 -o root -g ecs-exporter /secure/path/username \
  /etc/dell-ecs-metrics-exporter/secrets/username
sudo install -m 0640 -o root -g ecs-exporter /secure/path/password \
  /etc/dell-ecs-metrics-exporter/secrets/password
sudo install -m 0640 -o root -g ecs-exporter /secure/path/inventory-token \
  /etc/dell-ecs-metrics-exporter/secrets/inventory-token
```

Private CA 可放在同一目錄並設為 `0644 root:root`。不要把 private key 當成 CA
certificate。

### 步驟 4：設定並啟動

```bash
sudoedit /etc/dell-ecs-metrics-exporter/config.yaml
sudo -u ecs-exporter /usr/local/bin/ecs-exporter \
  -config=/etc/dell-ecs-metrics-exporter/config.yaml \
  -profiles-dir=/usr/share/dell-ecs-metrics-exporter/profiles \
  -validate-config
sudo systemctl enable --now dell-ecs-metrics-exporter
sudo ./deploy/bare-metal/verify.sh
```

預設只監聽 `127.0.0.1:8080`，適用同機 Prometheus agent 或 reverse proxy。若改成
主機介面，需用 host firewall 限制來源，並保護 Inventory API。服務使用 read-only
filesystem、NoNewPrivileges、private devices/tmp、kernel/system protection 及限制的
system-call/address-family policy。

### 升級與移除

使用新 release archive 再執行 `install.sh` 即可升級。先用 `--no-start` 驗證，再由
維護窗口執行 `sudo systemctl restart dell-ecs-metrics-exporter` 與
`sudo ./deploy/bare-metal/verify.sh`。不加 `--no-start` 時，安裝器會重啟服務；若
新版本啟動失敗，會還原前一版 binary、profiles 與 systemd unit。移除程式但保留
設定與 secrets：

```bash
sudo ./deploy/bare-metal/uninstall.sh
```

只有確認不再需要且已完成必要備份時，才可加上 `--purge-config`。完整細節見
[Bare Metal deployment](deploy/bare-metal/README.md)。

## 10. 維運與安全

### Logs

Exporter 將 access、ECS logical API 與 collector logs 以 JSON 寫至 stderr，包括：

- Cluster logical name
- logical API/collector name
- HTTP status
- duration
- retry count
- result/error type
- correlation ID

Logs 不會記錄 password、token、Authorization/Cookie、完整 ECS URL/query 或 raw
response body。請不要把 production `config.yaml` 或未遮罩 Inventory response 上傳到
公開 issue。

### Graceful shutdown

收到 `SIGINT` 或 `SIGTERM` 後，Exporter 會：

1. 停止排程。
2. 關閉 HTTP server。
3. 等待執行中的 collector 結束。
4. 嘗試向各 ECS Cluster logout。

### Token rotation

以新的隨機值原子替換 Inventory token file，下一個 request 即會使用新值。Kubernetes
Secret volume 更新可能有同步延遲；輪替期間請依平台行為保留短暫重疊或安排 client
切換。

### 本機檔案與 Git 安全

`.gitignore` 已排除本機 config overrides、`.local-secrets/`、CA/secret 目錄、
test/coverage/build output、scanner cache、kubeconfig、臨時 SSH/ESXi 設定，以及
放在 repository 根目錄的 OVA/OVF/VMDK 等虛擬設備映像。大型 ECS CE 安裝檔建議
仍下載到 repository 外；忽略規則只是最後一道防線，不是 secret manager。

在提交前至少執行：

```bash
git status --short
git diff --check
./HARNESS/harness.sh doctor
./HARNESS/harness.sh security
```

若 `security` 無法連線 Go vulnerability database，該 gate 是 **blocked/failed**，
不可當成成功。不要使用 `git add -f` 加入設定、secret、OVA、原始 ECS response
或未遮罩的 Inventory/metrics evidence。

### Prometheus cardinality

Bucket、Namespace、Node 與 interface 是受控但可能大量的 labels。導入前應量測實際
Bucket/Node 數量、scrape response size 與 Prometheus storage impact。Label 會移除
control characters、限制長度並加入穩定 hash 避免碰撞。

Production 告警範例位於 `deploy/prometheus/alerts.yaml`，事故處理、rollback、
credential/CA rotation 與災難復原步驟見
[Production Runbook](docs/PRODUCTION_RUNBOOK.md)。

## 11. 故障排除

### `load configuration` 或 `decode config` 失敗

先執行：

```bash
./dist/ecs-exporter -config config.yaml -profiles-dir profiles -validate-config
```

常見原因：

- YAML 欄位拼錯；loader 會拒絕未知欄位。
- 同時設定 `username` 與 `usernameFile`，或兩者都沒設定。
- Secret/CA file 不存在或 process 無讀取權限。
- `connect`／`read` 大於 `overall`。
- Cluster name 或 replication ID 重複。
- YAML 包含多份 document。

### `x509: certificate signed by unknown authority`

- 確認 `caFile` 是簽發 ECS server certificate 的 CA chain，不是 server private key。
- 確認 container/Pod 內的路徑與 mount path 完全一致。
- 確認 endpoint hostname 存在於 certificate SAN。
- 若企業自簽憑證確實無法建立上述 trust/SAN，可明確使用 `verify: false`；確認啟動
  WARN、限制管理網路，並記錄停用伺服器身分驗證的風險。

### Authentication 失敗或 HTTP 401

- 確認 username/password 檔案內容與權限。
- 確認帳號沒有停用或鎖定。
- 確認 `/user/whoami` 回傳 `SYSTEM_MONITOR` 或 `SYSTEM_ADMIN`。
- ECS token 過期後 Exporter 只會自動重新登入一次；持續 401 會保留舊快取並回報錯誤。

### ECS 3.8.x 經 proxy/load balancer 回 HTTP 403

確認設定的 endpoint hostname、TLS SNI、HTTP Host 與 ECS accepted server names 一致。
不要以改用 IP address 規避 Host Header policy。

### `/api/v1/health` 長時間是 `DOWN`

檢查 JSON logs 中 `collector execution` 的 `collector`、`result` 與 `error_type`：

- `authentication`：帳密、role 或 token 問題。
- `transport`：DNS、routing、firewall、TLS 或 timeout。
- `http_429`／`http_5xx`：ECS rate limit 或服務狀態。
- `mapping`／`invalid_response`：目標 ECS build schema 可能與 candidate mapping 不同。
- `cache_not_initialized`：Cluster/Node/Namespace/Bucket 至少一個尚未成功。

### Quota metric 不存在

這通常是正確行為。ECS 回傳未設定 quota（例如 `-1`）時，Exporter 不會輸出 quota
metric，也不會用 Cluster capacity 或 0 代替。請查詢 Inventory 的
`softQuotaConfigured`／`hardQuotaConfigured`。

### Performance 或 node disk metric 不存在

- ECS 3.7/3.8.0 會停用受已知 Flux range 問題影響的 interval-derived rates。
- Profile 為 `conditional` 時，該功能若已列於 shared validation，可直接依部署需求在
  `capabilities.enabledConditional` 明列 `vdc_performance`／
  `namespace_performance`；版本特定且未列入 shared validation 的 capability 仍須個別
  驗證，預設保持停用。
- ECS `monitoring_vdc` 只提供 VDC throughput/latency 與 Namespace
  transaction/error request rates；Namespace throughput/latency 不會輸出。
- Bucket-scope performance 在四個 Profile 都是 unavailable。
- Node disk 必須同時啟用 `node_disk_capacity` 並提供經驗證的
  `nodeResources.filesystems` allowlist。
- Node Management 與 Flux Resources 是獨立 collector。Flux unavailable 時
  Node inventory/health 仍可用，readiness 會以 HTTP 200 `DEGRADED` /
  `collector_error` 呈現，CPU/Memory/Network 值保持缺席。

### `unsupported ECS version`

Exporter 不會使用 nearest-version fallback。請記錄 `/vdc/nodes` 回傳的完整版本，
依 [DELL_ECS_API_MAPPING.md](DELL_ECS_API_MAPPING.md) 建立或擴充 Profile、mapping、
fixture 與 sandbox evidence，再進行程式變更。

## 12. 開發、品質與發布

標準交付命令：

```bash
./HARNESS/harness.sh selftest
./HARNESS/harness.sh doctor
./HARNESS/harness.sh verify
```

個別 stage：

```bash
./HARNESS/harness.sh lint
./HARNESS/harness.sh format:check
./HARNESS/harness.sh typecheck
./HARNESS/harness.sh test
./HARNESS/harness.sh coverage
./HARNESS/harness.sh build
./HARNESS/harness.sh security
./HARNESS/harness.sh deploy:check
./HARNESS/harness.sh ci:policy
```

Go race test：

```bash
bash -lc 'source scripts/go-env.sh; go test -race -timeout=3m ./...'
```

目前 coverage gate 為 80%。`security` stage 需要連線 Go 官方 vulnerability database；
最新 ECS-010 驗證結果以本次 handoff 與
[ECS-010 plan](plans/ECS-010.md)為準。先前可連線的完整驗證不會替代目前工作樹的
required security stage。Deployment check 在本機未安裝
`systemd-analyze`、`promtool` 與 `kubeconform`，只完成對應的 strict static checks；
這不取代真實 systemd host、Prometheus、container 或 Kubernetes runtime evidence。

Production 發布的 fail-closed 總入口：

```bash
RELEASE_VERSION=v1.0.0 ./scripts/release-check.sh
```

`v1.0.0-rc.1` 是評估用 Pre-release。Tag workflow 對 RC 執行 deterministic/race、
container、Kubernetes schema、合成 target-scale、dependency/license、雙架構 image
scan、SBOM、簽章、provenance 與 artifact publication；正式 ECS、deployed E2E 與
deployed performance protected jobs 只可由帶 prerelease suffix 的版本略過。沒有 suffix
的 `v1.0.0` 仍必須全部通過，詳細分流見
[Release Candidate Checklist](docs/RC_RELEASE_CHECKLIST.md)。

Release host 另外需要 Docker daemon、`kubeconform` v0.7.0、Syft v1.44.0、
Grype v0.112.0、OSV-Scanner v2.3.8、Cosign 與可連線的 vulnerability/license
databases；CI workflow 對 action 與 scanner 版本均有固定。OSV license gate 只允許
`Apache-2.0`、`MIT`、`BSD-2-Clause`、`BSD-3-Clause`、`ISC`，例外需依
[Security Policy](SECURITY.md) 記錄。

它會執行 deterministic/race、安全供應鏈、container、Kubernetes schema、合成與
真實部署效能、live ECS integration 及 deployed E2E gate。exit code 3 表示工具、
Git remote、Docker/Kubernetes、真實 ECS 或受保護 credential 等前置條件不足，仍是
發布失敗，不是 skip。逐項人工與自動條件見
[Production Release Checklist](docs/RELEASE_CHECKLIST.md)。

Stable GitHub tag release 另外使用 `ecs-certification`、`ecs-ce-compatibility`、
`ecs-e2e`、`ecs-performance` 與 `release` 五個 protected environments。CE gate
固定驗證 exact `3.8.0.3`；只有 readiness reason 為 `collector_error` 且唯一正值
collector error 是已知的 `node-resources` 時才允許 `DEGRADED`。建立 tag 前，需先
將同一個 full Git commit 的候選版本部署到 E2E 與 10/100/10,000 target-scale
環境；workflow 會比對 `/api/v1/version` 的 commit，任何舊 deployment 都會阻擋
publish。

建立 production tag 前，release policy 會確認必要功能都列於四個 Profile 的
`shared_validated_capabilities`。Tag workflow 仍會在正式 3.8.1.4 執行同一 commit 的
部署 smoke/E2E，這是環境與 release candidate 驗證，不再重複要求每個版本對相同功能
各自完成認證。

### 發布產物驗證

Release workflow 會發布 checksums、Sigstore bundle、各平台
(`linux/amd64`、`linux/arm64`) SBOM、OCI provenance、Helm package，以及以 digest
識別的 multi-architecture OCI image。Public 或 GitHub Enterprise Cloud repository
另外產生 GitHub-native attestations；不具 Enterprise Cloud 的 private RC 會發布已簽署
`github-attestation-boundary.txt`，stable 仍要求 native attestation 成功。將範例中的
owner、repository、version 與 digest 換成實際 release：

```bash
sha256sum --check SHA256SUMS

cosign verify-blob \
  --bundle SHA256SUMS.bundle \
  --certificate-identity \
  "https://github.com/OWNER/REPOSITORY/.github/workflows/release.yml@refs/tags/vVERSION" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  SHA256SUMS

cosign verify \
  --certificate-identity \
  "https://github.com/OWNER/REPOSITORY/.github/workflows/release.yml@refs/tags/vVERSION" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "ghcr.io/owner/repository@sha256:DIGEST"

cosign verify \
  --certificate-identity \
  "https://github.com/OWNER/REPOSITORY/.github/workflows/release.yml@refs/tags/vVERSION" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "ghcr.io/owner/charts/dell-ecs-metrics-exporter@sha256:HELM_DIGEST"

gh attestation verify ./dell-ecs-metrics-exporter_VERSION_linux_amd64.tar.gz \
  --repo OWNER/REPOSITORY
```

macOS 可用 `shasum -a 256 -c SHA256SUMS`。驗證成功後仍須確認
`release-metadata.json` 的 full commit、版本與核准紀錄一致。最後一個 `gh attestation`
命令只適用於 release 產生 GitHub-native attestation 的 repository。

## 13. 相容性與已知限制

- Profile、mapping 與 fixtures 位於 `profiles/`、`docs/ecs-api/`、
  `testdata/ecs/`。
- ECS 3.7/3.8.0 REST API 文件仍可補強版本差異，但已由其他目標版本真實驗證的共用
  功能狀態為 `validated-shared`。
- `ecs-flux-probe` 可補充 exact-build regression evidence；3.6/3.7/3.8.0 reports 不再是
  共用功能驗證的必要條件。
- Production gate、release workflow、SBOM/provenance/signing、container、
  Kubernetes schema、live/E2E 與 10 Cluster/10,000 Bucket 效能入口已實作；目前仍
  缺正式 ECS 3.8.1.4；ECS CE 3.8.1.4 只有 partial-live Management evidence，不能
  取代正式設備認證。ECS 3.8.1.1 實體設備已部分證實 Node/Performance Flux
  contract，但尚未涵蓋 non-zero workload、range/failure、verified TLS 與 multi-VDC；
  另缺測試 ECS CE 3.8.0.3 protected rerun、target-scale deployment、Docker daemon、
  live Kubernetes、外部 vulnerability database 與實際 Git release 的通過證據。
- 使用者必須在實際環境驗證 schema、unit、pagination、token lifecycle、Host/SNI、
  response size 與 API latency。
- 正式部署請使用 immutable image digest、外部 Secret Manager、受控 NetworkPolicy
  egress 及組織核准的 release pipeline。

更完整的契約與驗證邊界：

- [SPECIFICATION.md](SPECIFICATION.md)
- [DELL_ECS_API_MAPPING.md](DELL_ECS_API_MAPPING.md)
- [TEST_PLAN.md](TEST_PLAN.md)
- [TRACEABILITY.md](TRACEABILITY.md)
- [PROJECT.md](PROJECT.md)

## License

本專案採用 Apache License 2.0，詳見 [LICENSE](LICENSE)。
