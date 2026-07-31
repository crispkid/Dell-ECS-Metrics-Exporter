# Cross-Version Feature Validation

- Policy ID: `ECS-011`
- Machine value: `shared-live-any-target-version`
- Applies to: `ecs-3.6`, `ecs-3.7`, `ecs-3.8.0`, `ecs-3.8.1`

## Decision

功能只要在任一目標 ECS 版本，透過 Exporter production client、collector、parser 與
metric/inventory path 完成一次真實 ECS CE 或 appliance 驗證，就視為四個目標版本的
共用功能均已驗證。狀態使用 `validated-shared`，不再要求每個版本重複執行相同功能
才能更新支援矩陣。

以下情況不使用跨版本繼承，仍依 Profile 個別判定：

- Dell 文件或實測已證明行為不同，例如 Flux interval range、Host Header policy。
- 某 Profile 將 capability 設為 `unavailable`。
- 功能從未在任何真實 ECS 環境產生所需欄位或成功結果。
- 部署、TLS/權限、效能、故障注入及 release approval；這些是環境／交付 gate，
  不是 API 功能狀態。

`version.tested_builds` 只記錄實際執行過的 exact build，`sandbox_certified` 只表示完整
Profile certification。兩者不再作為 `validated-shared` 的必要條件，也不得因共享功能
狀態而填入未執行的版本。

## Shared Validation Matrix

下列狀態同時套用至四個 Profile。

| Feature/capability | Shared status | Qualifying live evidence | Boundary |
|---|---|---|---|
| Authentication login/token/logout | `validated-shared` | ECS CE 3.8.0.3、CE 3.8.1.4、appliance 3.8.1.1 | Token expiry/renewal failure injection另列為 operation gate |
| Version discovery/Profile selection | `validated-shared` | `/vdc/nodes` exact-build selection on 3.8.0.3、3.8.1.4、3.8.1.1 | Mixed-version rollout remains an operation gate |
| Cluster health/capacity | `validated-shared` | CE 3.8.0.3、CE 3.8.1.4、appliance 3.8.1.1 | Known-size capacity GB multiplier is not independently calibrated |
| Node inventory/health | `validated-shared` | CE 3.8.0.3 HAL health、CE 3.8.1.4、five-node appliance 3.8.1.1 | Service/process detail has no live field evidence |
| Node CPU/memory | `validated-shared` | Appliance 3.8.1.1 split Flux queries and exported series | Latest snapshot only |
| Node network counters | `validated-shared` | Appliance 3.8.1.1 preserved interface and exported receive/transmit series | Counter reset/range is an operation gate |
| Namespace inventory/capacity/quota | `validated-shared` | CE 3.8.0.3、CE 3.8.1.4 and appliance 3.8.1.1 | Additional unit calibration is not required for feature status |
| Namespace billing | `validated-shared` | CE known-size object totals and exact KB conversion | Billing MB/GB/TB were not separately calibrated |
| Bucket inventory/capacity/quota | `validated-shared` | CE 3.8.0.3 and CE 3.8.1.4 non-empty Bucket runs | Pagination at target scale is an operation gate |
| Bucket batch/single billing | `validated-shared` | CE 3.8.0.3 `{"id":[...]}` batch and single fallback shapes; CE 3.8.1.4 batch | Generic failure injection remains an operation gate |
| Flux latest snapshot | `validated-shared` | Appliance 3.8.1.1 CPU/memory/network and Performance query completion | CE Flux 503 does not invalidate appliance evidence |
| VDC performance | `validated-shared` | Appliance 3.8.1.1 VDC query/latency rows through production parser | Non-zero throughput/request load is an operation gate |
| Namespace request performance | `validated-shared` | Appliance 3.8.1.1 Namespace query contract and production parser | No Namespace throughput/latency mapping exists |
| Inventory API authentication/envelopes | `validated-shared` | CE 3.8.0.3 and appliance 3.8.1.1 Exporter runs | Reverse-proxy topology is a deployment gate |
| Prometheus exposition/readiness | `validated-shared` | CE/appliance metricscheck and health results | External Prometheus deployment is a deployment gate |
| Explicit `tls.verify: false` | `validated-shared` | Appliance 3.8.1.1 self-signed certificate run | It intentionally does not verify server identity |

## Not Yet Validated on Any Target Version

下列項目沒有 qualifying live evidence，因此不列入四個 Profile 的
`shared_validated_capabilities`：

- `node_service_process`
- `node_disk_capacity`
- `flux_interval_rates`
- `replication_status`
- `replication_lag`
- `recovery_progress`

`bucket_performance` 是 `unavailable`，屬於明確不支援而不是待驗證。

## Evidence Records

- [ECS CE 3.8.0.3](validation/ecs-ce-3.8.0.3-2026-07-25.md)
- [ECS CE 3.8.1.4](validation/ecs-ce-3.8.1.4-2026-07-26.md)
- [ECS appliance 3.8.1.1](validation/ecs-appliance-3.8.1.1-2026-07-30.md)
- [Four-profile fixture replay](validation/fixture-replay-2026-08-01.md)
