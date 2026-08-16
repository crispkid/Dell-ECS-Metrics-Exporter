# Dell ECS Metrics Exporter

Dell ECS Metrics Exporter 是以 Go 開發的唯讀監控程式，會定期向一或多套 Dell ECS
讀取容量、健康狀態、效能、Namespace、Bucket、Replication 與 Recovery 資料，並轉換成
Prometheus 可收集的 Metrics。

Exporter 只會讀取 ECS API，不會建立、修改或刪除 ECS 上的資料。每次成功收集的完整結果
會保存在記憶體快取中；Prometheus 與 Inventory API 都只讀取快取，不會因查詢而立即呼叫
ECS API。

目前版本為
[`v1.0.0-rc.1`](https://github.com/crispkid/Dell-ECS-Metrics-Exporter/releases/tag/v1.0.0-rc.1)
Release Candidate（RC，候選版本）。Repository、Release、Container Image 與 Helm Chart
目前皆為 Private，必須使用具有存取權限的 GitHub 帳號。RC 可用於功能評估與整合測試，
但不代表所有 ECS 實體設備都已完成正式認證。

本專案採用 Apache License 2.0，詳見 [LICENSE](LICENSE)。

## 專案簡介

### 運作方式

```text
Dell ECS Management / Monitoring API
                  │
                  │ Exporter 定期收集
                  ▼
             記憶體快取
              │      │
              │      └── 唯讀 Inventory API
              ▼
           /metrics
              │
              ▼
          Prometheus ── Grafana / Alertmanager
```

主要功能如下：

- 同時監控多套 ECS，每套 ECS 使用獨立的帳號、連線、重試機制與快取。
- 自動辨識 ECS 版本，並選用對應的 Compatibility Profile（相容性設定檔）。
- 提供 Prometheus Metrics、健康檢查及唯讀 Inventory API。
- ECS API 暫時失敗時保留上一份完整快取，避免輸出不完整資料。
- 支援 ECS Token 重新登入、逾時、流量限制、退避重試及 Graceful Shutdown（平順關閉）。
- 支援 Binary、Docker、Kubernetes／Helm 及 Linux systemd 安裝。

### 服務端點

| 路徑 | 用途 | 預設是否需要驗證 |
|---|---|---|
| `/health` | Exporter 程序存活檢查（Liveness） | 否 |
| `/api/v1/health` | ECS 資料與快取就緒狀態（Readiness） | 否 |
| `/api/v1/version` | Exporter 版本、Commit 與載入的 Profile | 否 |
| `/metrics` | Prometheus Metrics | 否，可設定為需要 Bearer Token |
| `/api/v1/clusters` | Cluster 清單 | 是 |
| `/api/v1/nodes` | Node 清單 | 是 |
| `/api/v1/namespaces` | Namespace 清單 | 是 |
| `/api/v1/buckets` | Bucket 清單 | 是 |
| `/api/v1/replications` | Replication 與 Recovery 清單 | 是 |

Inventory API 預設使用 Bearer Token。它的用途是查詢目前快取中的資源資訊，不是用來
修改 ECS。

## 可收集的 Metrics

所有 Metrics 都使用 `ecs_` 開頭。只有 ECS API 實際提供、功能已啟用且快取仍在有效期限
內的資料才會輸出。Quota 未設定、API 回傳空值或功能不適用時，對應的 Metric 會保持缺席，
不會以 `0` 代替未知值。

容量統一使用 `bytes`，效能流量使用 `bytes/second`，時間使用 `seconds`，比例值使用
`0`～`1` 的 `ratio`。需要顯示百分比時可在 PromQL 或 Grafana 將 ratio 乘以 `100`；
需要顯示 GiB 時可將 bytes 除以 `1024^3`。

### Cluster

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_cluster_health` | Gauge | 無單位（`0`／`1`） | Cluster 健康狀態，正常為 `1`、異常為 `0` |
| `ecs_cluster_capacity_total_bytes` | Gauge | bytes | Cluster 總容量 |
| `ecs_cluster_capacity_used_bytes` | Gauge | bytes | Cluster 已使用容量 |
| `ecs_cluster_capacity_available_bytes` | Gauge | bytes | Cluster 可用容量 |
| `ecs_cluster_buckets` | Gauge | count | Bucket 數量 |
| `ecs_cluster_namespaces` | Gauge | count | Namespace 數量 |
| `ecs_cluster_objects` | Gauge | count | Object 數量 |

### Node

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_node_health` | Gauge | 無單位（`0`／`1`） | Node 健康狀態，正常為 `1`、異常為 `0` |
| `ecs_node_service_health` | Gauge | 無單位（`0`／`1`） | Node 上服務或程序的健康狀態 |
| `ecs_node_cpu_usage_ratio` | Gauge | ratio（`0`～`1`） | CPU 使用率，範圍為 `0` 到 `1` |
| `ecs_node_memory_used_bytes` | Gauge | bytes | 已使用記憶體 |
| `ecs_node_memory_total_bytes` | Gauge | bytes | 總記憶體 |
| `ecs_node_disk_used_bytes` | Gauge | bytes | 已使用磁碟容量 |
| `ecs_node_disk_total_bytes` | Gauge | bytes | 總磁碟容量 |
| `ecs_node_network_receive_bytes_total` | Counter | bytes（累計） | 網路介面累計接收流量 |
| `ecs_node_network_transmit_bytes_total` | Counter | bytes（累計） | 網路介面累計送出流量 |

Node CPU、記憶體與網路資料來自 ECS Flux API。Node Disk Metrics 預設不輸出；必須先確認
目標 ECS 回傳的檔案系統名稱，再啟用 `node_disk_capacity` 並設定
`nodeResources.filesystems` Allowlist（允許清單）。

### Namespace

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_namespace_capacity_used_bytes` | Gauge | bytes | Namespace 已使用容量 |
| `ecs_namespace_quota_bytes` | Gauge | bytes | Namespace Hard Quota；未設定時不輸出 |
| `ecs_namespace_buckets` | Gauge | count | Namespace 內的 Bucket 數量 |
| `ecs_namespace_objects` | Gauge | count | Namespace 內的 Object 數量；API 未提供時不輸出 |

### Bucket

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_bucket_used_bytes` | Gauge | bytes | Bucket 已使用容量 |
| `ecs_bucket_soft_quota_bytes` | Gauge | bytes | Bucket Soft Quota；未設定時不輸出 |
| `ecs_bucket_hard_quota_bytes` | Gauge | bytes | Bucket Hard Quota；未設定時不輸出 |
| `ecs_bucket_objects` | Gauge | count | Bucket 內的 Object 數量 |

目前支援的 ECS 版本沒有經確認的 Bucket 層級效能 API，因此不輸出 Bucket Throughput、
Latency、Request Rate 或 HTTP Status Metrics。

### VDC 與 Namespace 效能

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_vdc_read_throughput_bytes_per_second` | Gauge | bytes/second | VDC 每秒讀取量 |
| `ecs_vdc_write_throughput_bytes_per_second` | Gauge | bytes/second | VDC 每秒寫入量 |
| `ecs_vdc_request_latency_seconds` | Gauge | seconds | VDC Request Latency，包含 Operation 與 Quantile Label |
| `ecs_vdc_requests` | Gauge | requests/second | VDC 每秒 Request 數，包含 Operation 與 Status Class Label |
| `ecs_namespace_requests` | Gauge | requests/second | Namespace 每秒 Request 數 |

ECS 已驗證的資料模型沒有 Namespace Throughput 或 Namespace Latency，因此不會輸出這兩類
Metrics。效能功能可能需要在該 Cluster 的 `capabilities.enabledConditional` 中明確啟用
`vdc_performance` 或 `namespace_performance`。

### Replication 與 Recovery

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_replication_status` | Gauge | 無單位（`0`／`1`） | Replication 狀態，正常為 `1`、異常為 `0` |
| `ecs_replication_lag_seconds` | Gauge | seconds | Replication 延遲秒數 |
| `ecs_recovery_progress_ratio` | Gauge | ratio（`0`～`1`） | Recovery 進度，範圍為 `0` 到 `1` |

Exporter 只會查詢設定檔中明確列出的 Replication Group 與 Link ID。

### Exporter 自我監控

| Metric | 類型 | 單位 | 說明 |
|---|---|---|---|
| `ecs_exporter_api_requests_total` | Counter | count | 呼叫 ECS API 的次數 |
| `ecs_exporter_api_errors_total` | Counter | count | ECS API 錯誤次數 |
| `ecs_exporter_api_request_duration_seconds` | Histogram | seconds | ECS API 呼叫時間 |
| `ecs_exporter_api_response_size_bytes` | Histogram | bytes | ECS API Response 大小 |
| `ecs_exporter_collector_runs_total` | Counter | count | Collector 執行次數 |
| `ecs_exporter_collector_errors_total` | Counter | count | Collector 錯誤次數 |
| `ecs_exporter_collector_duration_seconds` | Histogram | seconds | Collector 執行時間 |
| `ecs_exporter_cache_refresh_total` | Counter | count | 快取成功更新次數 |
| `ecs_exporter_cache_refresh_errors_total` | Counter | count | 快取更新失敗次數 |
| `ecs_exporter_last_success_timestamp_seconds` | Gauge | Unix timestamp（seconds） | Collector 最近成功時間 |
| `ecs_exporter_cache_age_seconds` | Gauge | seconds | 快取資料年齡 |
| `ecs_exporter_cached_resources` | Gauge | count | 快取中的資源數量 |
| `ecs_exporter_build_info` | Gauge | 無單位（固定為 `1`） | Exporter 版本、Commit 與建置日期 |
| `ecs_exporter_profile_contract_info` | Gauge | 無單位（固定為 `1`） | 已載入 Profile 與驗證狀態 |

常用 Label 包含 `cluster`、`site`、`environment`、`vdc`、`node`、`namespace`、
`bucket`、`interface`、`operation` 與 `status_class`。為避免 Prometheus Cardinality
（時間序列數量）失控，請不要自行把完整 URL、錯誤訊息或其他高變動值加入 Label。

## ECS 相容版本

Exporter 會在啟動時讀取 ECS 完整版本，並依下表選擇 Profile。版本不在支援範圍內時會
直接停止該 Cluster 的收集，不會自動套用最接近的版本。

| Profile | 支援範圍 | 已執行的實際驗證 | 注意事項 |
|---|---|---|---|
| `ecs-3.6` | `3.6.0.0` 以上、未滿 `3.7.0.0` | ECS CE `3.6.2.0` 已驗證 Management、Bucket、Quota、Billing、Inventory 與對應 Metrics | CE 的 Node Resources 與 Performance Flux API 回 HTTP 503 |
| `ecs-3.7` | `3.7.0.0` 以上、未滿 `3.8.0.0` | ECS CE `3.7.0.0` 已驗證 Management、Bucket、Quota、Billing、Inventory 與對應 Metrics | CE 的 Node Resources Flux API 回 HTTP 503 |
| `ecs-3.8.0` | `3.8.0.0` 以上、未滿 `3.8.1.0` | ECS CE `3.8.0.3` 已驗證 Management、Bucket、Quota、Billing、Inventory 與對應 Metrics；使用者另確認實體 ECS `3.8.0.x` 相容性測試通過 | Dell 已知的 Flux 時間範圍問題仍適用，因此停用 interval-derived rate；實機 exact build 與去識別化報告尚未納入 Repository |
| `ecs-3.8.1` | `3.8.1.0` 以上、未滿 `3.8.2.0` | ECS CE `3.8.1.4` 已驗證 Management 類資料；實體 ECS `3.8.1.1` 已驗證 Node CPU、記憶體、網路與部分 Performance 資料；使用者另確認實體 ECS `3.8.1.x` 相容性測試通過 | 實機 exact build 與去識別化報告尚未納入 Repository；interval-derived rate 仍須通過環境別條件檢查 |

### Profile 自動選擇與安裝內容

使用者不需要在 `config.yaml` 指定 Profile。Exporter 連線每套 ECS 後，會從
`/vdc/nodes` 讀取完整版本，再自動選擇上表對應的 Profile。若同一套 ECS 回傳無法相容的
混合版本，或版本不在支援範圍內，該 Cluster 會停止收集，不會套用鄰近版本繼續執行。

`v1.0.0-rc.1` 的正式安裝產物已包含完整的 Runtime Profiles：

```text
profiles/
├── ecs-3.6.json
├── ecs-3.7.json
├── ecs-3.8.0.json
├── ecs-3.8.1.json
└── profile.schema.json
```

| 安裝方式 | Profile 位置 | 是否需要另外下載 |
|---|---|---|
| Release Binary | 解壓目錄中的 `profiles/` | 否 |
| Docker | Image 內的 `/profiles` | 否 |
| Kubernetes／Helm | Pod 使用的 Image 內 `/profiles` | 否 |
| Bare Metal／systemd | `/usr/share/dell-ecs-metrics-exporter/profiles` | 否，安裝程式會自動複製 |

只有自行單獨複製 `ecs-exporter` Binary 時，才需要另外保留同一版本的 `profiles/` 目錄；
Profile 與 Binary 版本不應混用。啟動後可從 `/api/v1/version` 確認實際選用的 Profile。

Profile 自動選擇不代表所有選用功能都會自動開啟。`vdc_performance`、
`namespace_performance`、`node_disk_capacity` 等 Conditional Capability（條件式功能）
仍須依實際環境在該 Cluster 的 `capabilities.enabledConditional` 中明確啟用；Node Disk
另外需要設定 `nodeResources.filesystems` Allowlist（允許清單）。

本專案採用共用功能驗證原則：同一個 API 功能只要在任一目標版本以正式程式路徑完成真實
驗證，就視為四個 Profile 的共用功能已驗證；版本特有的差異仍由各 Profile 分別處理。
這項原則不代表每個 ECS Build 都已完成實機認證。

上表的「使用者確認」是依使用者提供的實機測試結果更新，不等同 GitHub Release
Workflow 所產生的可重現 exact-build 證據。在完整四段 ECS 版本與去識別化測試報告納入
受控驗證流程前，Profile 的 `version.tested_builds` 仍保持空白，也不會標示為完整
Profile 認證。

ECS Community Edition（CE）不一定提供可用的 Flux External Query。CE 回 HTTP 503 時，
Management 類 Metrics 仍可使用，但 CPU、記憶體、網路或 Performance Metrics 可能缺席，
`/api/v1/health` 也可能顯示 `DEGRADED`。這不等於實體 ECS 一定不支援 Flux。

## 安裝與設定

### 選擇安裝方式

| 環境 | 建議方式 |
|---|---|
| Linux 或 macOS 單機測試 | Release Binary |
| 已有 Docker 或相容 OCI Runtime | Docker |
| Kubernetes | Helm |
| Linux 實體機或 VM | Bare Metal／systemd |

不論使用哪一種方式，都必須先準備 ECS 帳號及設定檔。

### 安裝前準備

ECS 端需要：

1. ECS Management HTTPS Endpoint，例如 `https://ecs-management.example.com`。
2. Exporter 到 ECS 的 DNS 與 TCP 4443 連線；Endpoint 指定其他 Port 時則使用該 Port。
3. 建議使用具有 `SYSTEM_MONITOR` Role 的唯讀帳號。`SYSTEM_ADMIN` 可以使用，但權限較大。
4. ECS Server Certificate 的 CA 憑證；若系統已信任該 CA，可不另外指定。

建立本機 Secret 檔案：

```bash
umask 077
mkdir -p .local-secrets
printf '%s\n' 'ecs-monitor-user' > .local-secrets/username
openssl rand -hex 32 > .local-secrets/inventory-token
bash -c 'read -r -s -p "ECS password: " password; printf "\n"; printf "%s" "$password" > .local-secrets/password'
chmod 600 .local-secrets/*
```

請將 `ecs-monitor-user` 改成實際帳號。Inventory Token 是用來保護 Exporter Inventory API，
不是 ECS 登入後回傳的 Token。

建立 `config.yaml`：

```yaml
security:
  inventoryApi:
    enabled: true
    authentication: token
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

設定注意事項：

- `name` 必須在設定檔中唯一。
- `endpoint` 只能填 HTTPS Origin，不可包含 API Path、Query、帳號或密碼。
- 使用企業 Private CA 時，把 `caFile` 指向 CA 檔案。
- 自簽憑證無法驗證時可設定 `verify: false`，但這會同時停用憑證鏈與主機名稱驗證，
  Exporter 會輸出警告。正式環境應優先匯入正確的 CA。
- 多套 ECS 可在 `clusters` 下新增項目，每套 ECS 使用各自的帳號與 TLS 設定。

完整設定範例位於 [config.example.yaml](config.example.yaml)。

### 方式一：Release Binary

從
[`v1.0.0-rc.1` Release](https://github.com/crispkid/Dell-ECS-Metrics-Exporter/releases/tag/v1.0.0-rc.1)
下載符合作業系統與 CPU 架構的檔案，並核對 Release 頁面顯示的 SHA-256 Digest。

```bash
tar -xzf dell-ecs-metrics-exporter_1.0.0-rc.1_linux_amd64.tar.gz
cd dell-ecs-metrics-exporter_1.0.0-rc.1_linux_amd64
```

將前一節建立的 `config.yaml` 與 `.local-secrets` 放在目前目錄，再驗證設定：

```bash
./ecs-exporter \
  -config config.yaml \
  -profiles-dir profiles \
  -validate-config
```

驗證成功後啟動：

```bash
./ecs-exporter -config config.yaml -profiles-dir profiles
```

若要從原始碼建置，需先安裝 Go 1.26.6，再於 Repository 根目錄執行：

```bash
./scripts/build.sh
./dist/ecs-exporter -config config.yaml -profiles-dir profiles
```

### 方式二：Docker

目前 Image 為 Private，請先從 GitHub Release 下載 `image-digest.txt`，再登入 GHCR。
此檔案記錄本次發布的不可變 Image Digest：

```bash
docker login ghcr.io
IMAGE_REFERENCE="$(tr -d '\r\n' < image-digest.txt)"
docker pull "$IMAGE_REFERENCE"
```

將 `config.yaml` 中的 Secret 路徑改成 Container 內路徑：

```yaml
tokenFile: /var/run/secrets/ecs-exporter/inventory-token
usernameFile: /var/run/secrets/ecs-exporter/username
passwordFile: /var/run/secrets/ecs-exporter/password
```

啟動 Container：

```bash
docker run -d \
  --name dell-ecs-metrics-exporter \
  --restart unless-stopped \
  --read-only \
  --user "$(id -u):$(id -g)" \
  -p 8080:8080 \
  --mount type=bind,src="$PWD/config.yaml",dst=/etc/ecs-exporter/config.yaml,readonly \
  --mount type=bind,src="$PWD/.local-secrets",dst=/var/run/secrets/ecs-exporter,readonly \
  "$IMAGE_REFERENCE"
```

若使用 Private CA，再把 CA 目錄掛載到 Container，並將 `caFile` 設為 Container 內的
完整路徑。

### 方式三：Kubernetes／Helm

先建立 Namespace、GHCR Pull Secret 與 ECS Secret。下列 GHCR Token 必須具有
`read:packages` 權限：

```bash
kubectl create namespace monitoring

read -r -s -p "GHCR token: " GHCR_TOKEN
kubectl -n monitoring create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password="$GHCR_TOKEN"
unset GHCR_TOKEN

kubectl -n monitoring create secret generic ecs-exporter-credentials \
  --from-file=inventory-token=.local-secrets/inventory-token \
  --from-file=username=.local-secrets/username \
  --from-file=password=.local-secrets/password
```

下載 Helm Chart：

```bash
helm registry login ghcr.io
helm pull oci://ghcr.io/crispkid/charts/dell-ecs-metrics-exporter \
  --version 1.0.0-rc.1 \
  --untar \
  --untardir /tmp/ecs-exporter-chart
```

建立 `/tmp/ecs-exporter-values.yaml`：

```yaml
image:
  repository: ghcr.io/crispkid/dell-ecs-metrics-exporter
  digest: ""
  tag: ""

imagePullSecrets:
  - name: ghcr-pull

credentials:
  existingSecret: ecs-exporter-credentials

config:
  content: |-
    server:
      listenAddress: ":8080"
    prometheus:
      path: /metrics
      protected: false
    security:
      inventoryApi:
        enabled: true
        authentication: token
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

安裝並確認 Pod 狀態：

```bash
IMAGE_REFERENCE="$(tr -d '\r\n' < image-digest.txt)"
IMAGE_DIGEST="${IMAGE_REFERENCE##*@}"

helm upgrade --install ecs-exporter \
  /tmp/ecs-exporter-chart/dell-ecs-metrics-exporter \
  --namespace monitoring \
  --values /tmp/ecs-exporter-values.yaml \
  --set-string image.digest="$IMAGE_DIGEST" \
  --wait \
  --timeout 5m

kubectl -n monitoring rollout status \
  deployment/ecs-exporter-dell-ecs-metrics-exporter
kubectl -n monitoring get pods,service
```

正式環境請依
[Production Values 範例](charts/dell-ecs-metrics-exporter/values-production.example.yaml)
限制 NetworkPolicy、設定資源上限，並依 Prometheus Operator 環境決定是否啟用
`serviceMonitor.enabled`。

### 方式四：Bare Metal／systemd

此方式支援使用 systemd 的 Linux amd64 與 arm64。請下載並解壓對應的 Linux Release
Archive，然後執行：

```bash
sudo ./deploy/bare-metal/install.sh \
  --binary ./ecs-exporter \
  --profiles ./profiles \
  --no-start
```

安裝 Secret：

```bash
sudo install -m 0640 -o root -g ecs-exporter /secure/path/username \
  /etc/dell-ecs-metrics-exporter/secrets/username
sudo install -m 0640 -o root -g ecs-exporter /secure/path/password \
  /etc/dell-ecs-metrics-exporter/secrets/password
sudo install -m 0640 -o root -g ecs-exporter /secure/path/inventory-token \
  /etc/dell-ecs-metrics-exporter/secrets/inventory-token
```

編輯 `/etc/dell-ecs-metrics-exporter/config.yaml`，再驗證並啟動：

```bash
sudo -u ecs-exporter /usr/local/bin/ecs-exporter \
  -config=/etc/dell-ecs-metrics-exporter/config.yaml \
  -profiles-dir=/usr/share/dell-ecs-metrics-exporter/profiles \
  -validate-config

sudo systemctl enable --now dell-ecs-metrics-exporter
sudo ./deploy/bare-metal/verify.sh
```

預設只監聽 `127.0.0.1:8080`。若要讓遠端 Prometheus 連線，請調整
`server.listenAddress`，並用主機防火牆限制只有 Prometheus 可以存取。

### 確認安裝結果

無論使用哪一種方式，都可以依序檢查：

```bash
curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/api/v1/version
curl -sS http://127.0.0.1:8080/api/v1/health
curl -sS http://127.0.0.1:8080/metrics | head
```

正常情況下：

- `/health` 回傳 HTTP 200 與 `UP`。
- `/api/v1/version` 顯示正確版本及選用的 ECS Profile。
- Collector 完成第一次收集後，`/api/v1/health` 應為 `UP`；部分可選功能失敗時可能為
  `DEGRADED`。
- `/metrics` 可以看到 `ecs_exporter_build_info` 及 ECS 資料。

Prometheus Scrape 設定範例：

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

## 日常維運與安全

### 建議監控項目

除了 ECS 本身的 Metrics，至少要監控下列 Exporter 狀態：

| 項目 | 建議判斷方式 |
|---|---|
| Exporter 是否可連線 | Prometheus 的 `up{job="dell-ecs-metrics-exporter"}` 應為 `1` |
| Collector 是否持續失敗 | `ecs_exporter_collector_errors_total` 是否持續增加 |
| ECS API 是否異常 | `ecs_exporter_api_errors_total` 是否持續增加 |
| 資料是否過舊 | `ecs_exporter_cache_age_seconds` 是否超過設定的收集週期 |
| 最近是否成功收集 | `ecs_exporter_last_success_timestamp_seconds` |
| API 是否變慢 | `ecs_exporter_api_request_duration_seconds` |

告警範例位於 [deploy/prometheus/alerts.yaml](deploy/prometheus/alerts.yaml)。

### 健康狀態

- `UP`：必要 Collector 已成功，快取可以正常使用。
- `DEGRADED`：仍有可用快取，但部分 Collector 失敗或資料已超過建議時間。
- `DOWN`：必要快取尚未建立，或已超過最大可接受時間。

Exporter 只保存記憶體快取，重啟後會重新向 ECS 收集，不需要備份 Exporter 資料。
歷史資料應由 Prometheus 保存。

### 帳號、Token 與憑證

- ECS 帳號建議使用最小權限的 `SYSTEM_MONITOR` Role。
- 密碼、Inventory Token、Private CA 與正式環境設定不可提交到 Git 或寫入 Container Image。
- ECS 帳號或密碼變更後要重新啟動 Exporter。
- Inventory Token 檔案可以直接替換；新 Request 會讀取新值。
- 建議保持 `tls.verify: true`。使用 `false` 時，必須限制管理網路並留下風險紀錄。
- `/api/v1/*` 應保持 Bearer Token 驗證；若 `/metrics` 會經過不受信任的網路，也應設定
  `prometheus.protected: true` 或使用 Reverse Proxy 保護。

### 網路與部署安全

- ECS Management Port 只開放給 Exporter。
- Exporter 的 8080 Port 只開放給 Prometheus、受信任的管理端或 Reverse Proxy。
- Kubernetes 應使用 NetworkPolicy 限制連入來源、DNS 與 ECS 目的地。
- Container 與 systemd 安裝預設使用非 Root、唯讀檔案系統及權限限制，請勿為了排除問題
  直接改成特權模式。
- 正式部署應使用不可變的 Image Digest，不使用 `latest`。
- 升級前應核對 Release SHA-256、SBOM 與 Cosign 簽章；保留上一版 Digest 供 Rollback。

### Logs 與敏感資料

Exporter 會將 JSON Logs 寫到標準錯誤輸出，內容包含 Cluster 邏輯名稱、Collector、HTTP
狀態、執行時間、重試次數及錯誤類型。正常情況下不會記錄密碼、Token、Cookie、完整
ECS URL 或原始 Response。

查看 Logs：

```bash
# Docker
docker logs --tail 200 dell-ecs-metrics-exporter

# Kubernetes
kubectl -n monitoring logs deployment/ecs-exporter-dell-ecs-metrics-exporter

# systemd
sudo journalctl -u dell-ecs-metrics-exporter -n 200 --no-pager
```

不要把正式環境的 `config.yaml`、未遮罩的 Inventory Response 或 Logs 上傳到公開的
Issue。

### 升級與 Rollback

- Docker：先拉取並驗證新 Digest，再以相同設定重建 Container；需要回復時改回上一個
  Digest。
- Helm：使用新的 Digest 執行 `helm upgrade`；需要回復時執行
  `helm rollback ecs-exporter REVISION --namespace monitoring`。
- systemd：以新 Release Archive 再執行 `install.sh`。安裝器在啟動失敗時會嘗試還原前一版
  Binary、Profiles 與 Unit。

詳細事故處理與輪替程序請參閱 [Production Runbook](docs/PRODUCTION_RUNBOOK.md)。

## 故障排除

### 建議檢查順序

1. 執行設定驗證，確認 YAML、Secret 與 Profile 可以載入。
2. 檢查 `/health`、`/api/v1/version` 及 `/api/v1/health`。
3. 查看 Logs 中的 `collector`、`result` 與 `error_type`。
4. 確認 DNS、TCP Port、TLS、帳號權限及 ECS 版本。
5. 最後才調整 Timeout、重試或 Conditional Capability。

設定驗證命令：

```bash
./ecs-exporter \
  -config config.yaml \
  -profiles-dir profiles \
  -validate-config
```

### 設定檔無法載入

常見原因：

- YAML 縮排或欄位名稱錯誤；Exporter 會拒絕未知欄位。
- 同時設定 `username` 與 `usernameFile`，或兩者都沒有設定。
- Secret 或 CA 檔案不存在，或執行 Exporter 的帳號沒有讀取權限。
- Cluster 名稱重複。
- Connection／Read Timeout 大於 Overall Timeout。

### `x509: certificate signed by unknown authority`

- 確認 `caFile` 是簽發 ECS Server Certificate 的 CA Chain，不是 Server Private Key。
- 確認 Endpoint Hostname 存在於憑證的 SAN。
- Docker 或 Kubernetes 必須確認 CA 已掛載到設定檔指定的相同路徑。
- 只有在組織明確接受風險時才使用 `tls.verify: false`。

### HTTP 401 或登入失敗

- 檢查 Username／Password 檔案是否有多餘空白、換行或權限問題。
- 確認帳號未被停用或鎖定。
- 確認 `/user/whoami` 回傳 `SYSTEM_MONITOR` 或 `SYSTEM_ADMIN`。
- Exporter 在 Token 失效後只會自動重新登入一次；持續 401 時會保留舊快取並記錄錯誤。

### 經過 Proxy 或 Load Balancer 時回 HTTP 403

確認設定的 Endpoint Hostname、TLS SNI、HTTP Host Header 與 ECS 接受的 Server Name
一致。不要直接改用 IP 來繞過 Hostname 檢查。

### `/api/v1/health` 長時間為 `DOWN`

依 Logs 中的 `error_type` 判斷：

| 錯誤類型 | 常見原因 |
|---|---|
| `authentication` | 帳號、密碼、Role 或 ECS Token 問題 |
| `transport` | DNS、Routing、Firewall、TLS 或 Timeout 問題 |
| `http_429` | ECS API 流量限制 |
| `http_5xx` | ECS 服務暫時異常 |
| `mapping`／`invalid_response` | ECS Response 格式與目前 Profile 不一致 |
| `cache_not_initialized` | 至少一個必要 Collector 尚未成功 |

啟動後第一次收集完成前，短暫出現 HTTP 503／`DOWN` 是正常現象。

### ECS CE 顯示 `DEGRADED` 或 Flux 回 HTTP 503

目前測試過的多個 ECS CE 版本會讓 Flux Node Resources 或 Performance Query 回 HTTP 503。
此時 Cluster、Namespace、Bucket、Quota 與 Billing 等 Management 類 Metrics 仍可能正常，
但 CPU、記憶體、網路或 Performance Metrics 會缺席。請先確認這是 CE 限制，不要直接修改
Profile 來忽略實體設備上的同類錯誤。

### Quota Metric 不存在

這通常是正常行為。ECS 回傳未設定 Quota（例如 `-1`）時，Exporter 不會輸出 Quota
Metric，也不會用 `0` 代替。可從 Inventory API 檢查對應的 `*QuotaConfigured` 欄位。

### CPU、記憶體、網路或 Performance Metric 不存在

- 先確認 `/api/v1/version` 選到正確的 Profile。
- 檢查 `node-resources` 或 `performance` Collector 是否成功。
- Conditional 功能必須在 `capabilities.enabledConditional` 明確啟用。
- ECS CE 的 Flux API 可能回 HTTP 503；請在實體 ECS 上再次確認。
- Namespace 只提供 Request Rate，不提供已驗證的 Throughput 或 Latency。
- Bucket 層級 Performance 目前不支援。

### Node Disk Metric 不存在

Node Disk 預設關閉。必須先確認 ECS 回傳的檔案系統名稱，再於對應的 Cluster 下同時設定：

```yaml
ecs:
  clusters:
    - name: primary-ecs
      # 其他 Cluster 設定省略
      capabilities:
        enabledConditional:
          - node_disk_capacity
      nodeResources:
        filesystems:
          - /filesystem/name/from-ecs
```

不要使用過度寬鬆的 Allowlist，避免把暫存或虛擬檔案系統轉成大量時間序列。

### `unsupported ECS version`

目標 ECS 版本不在目前四個 Profile 的範圍內。Exporter 不會自動使用鄰近版本。請記錄
ECS 回傳的完整版本，並在隔離測試環境確認 API、Mapping 與 Fixture 後，再由維護人員新增
或調整 Profile。

### Prometheus 沒有資料

- 開啟 Prometheus Targets 頁面，確認 Exporter Target 為 `UP`。
- 從 Prometheus 主機直接呼叫 Exporter 的 `/metrics`。
- 確認 `metrics_path`、Port、Firewall 與 DNS。
- 若設定 `prometheus.protected: true`，Prometheus 必須送出正確的 Bearer Token。
- 確認 Metric 沒有因資料過期、Quota 未設定或 Conditional 功能未啟用而合理缺席。

### Docker 出現 `permission denied`

確認執行 Container 的 UID／GID 可以讀取 `config.yaml`、Secret 與 CA 檔案。不要用
`--privileged` 解決權限問題；應調整檔案擁有者、權限或 `--user`。

### Kubernetes 出現 `ImagePullBackOff`

- 確認 `ghcr-pull` Secret 中的 GitHub Token 具有 `read:packages` 權限。
- 確認 `imagePullSecrets` 名稱與 Secret 相同，而且位於相同 Namespace。
- 確認 Image Repository 與 Digest 正確。

若仍無法判斷，請保留 Exporter 版本、ECS 完整版本、選用的 Profile、健康狀態及已遮罩的
錯誤類型，再依 [Production Runbook](docs/PRODUCTION_RUNBOOK.md) 進一步調查。
