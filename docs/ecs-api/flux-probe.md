# ECS Flux Compatibility Probe

`ecs-flux-probe` 是提供給 ECS 管理者、Dell Support 或合作夥伴執行的唯讀相容性
探針。它會使用與 Exporter 相同的 client、Profile、query、parser 與 cache mapping，
但只輸出去識別化結果。探針不會建立 Bucket、寫入物件、修改 ECS 設定或啟動
Prometheus HTTP service。

## Evidence Boundary

- 真實 ECS appliance 上的報告分類為 `live-read-only-redacted`，可作為 exact-build
  候選相容性證據；仍需 Project Maintainer 審查，不能單獨視為 Profile certification。
- ECS CE 缺少 production fabric/Flux 能力時，Management 成功或 Flux HTTP 503 只能
  證明該 CE 環境的結果，不能替代相同版本實體設備。
- `testdata/ecs` replay 只證明四個 Profile 共用 mapping 的 deterministic regression，
  不證明真實 endpoint、權限、TLS、資料新鮮度或 range behavior。
- Probe 使用 `last()` snapshot。它不解除 ECS 3.7/3.8.0 interval-rate 禁用，也不會
  自動啟用 ECS 3.8.1 的 conditional interval rates。

## Build

```bash
./scripts/build.sh
./dist/ecs-flux-probe -help
```

一般 build 與 release archive 都包含 `ecs-flux-probe`。必須使用與待驗證 Exporter
相同 commit、Profile 目錄與設定，避免測到不同 mapping。

## Prepare a Read-only Configuration

沿用 Exporter `config.yaml`。建議使用 `SYSTEM_MONITOR` 帳號、`usernameFile` 與
`passwordFile`；不得把密碼放在命令列或報告內。設定有多個 Cluster 時，命令列必須
用 `-cluster` 選擇其中一個。企業自簽憑證可以明確設定 `tls.verify: false`，但報告
通過不代表 TLS server identity 已驗證。

探針預設檢查 VDC/Namespace Performance。Disk 預設關閉；只有設定
`nodeResources.filesystems` allowlist 後才能使用 `-disk=true`，避免無界 filesystem
cardinality。

## Run

```bash
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

單一 Cluster 設定可以省略 `-cluster`。探針只會呼叫下列 read-only contract：

1. Login、`GET /vdc/nodes`、`GET /user/whoami`。
2. Cluster capacity/health 與 Node inventory/health GET。
3. `POST /flux/api/external/v2/query`，body 只含 Exporter 的 Flux read query。
4. Logout。

Exit code `0` 表示所有啟用檢查完成；`1` 表示 setup、bootstrap 或至少一個檢查失敗。
即使 exit code 是 `1`，stdout 仍會保留可分享的 redacted JSON report。

## Report Contract

Report 可包含：

- 完整 ECS build 字串、選到的 Profile、mixed-version 狀態。
- `flux_interval_rates`、VDC/Namespace Performance、Disk 的 Profile policy。
- 每個 stage 的 `pass`、`error` 或 `skipped`，以及受控 error type/HTTP status。
- CPU、Memory、Disk、Network、VDC/Namespace Performance 的 series 數量。

Report 永遠省略：

- ECS endpoint、IP/DNS、帳號、密碼、Token、Cookie 與 Authorization header。
- Cluster、VDC、Node、interface、Namespace、Bucket 或 filesystem identity。
- 原始 response、Flux sample 值與 Prometheus metric 值。
- 底層錯誤文字；transport/API 只保留受控類型與 HTTP status。

`passed` 只表示啟用的 latest-snapshot contract 在該次執行完成。`empty: true` 且 stage
為 `pass` 表示 ECS 回應合法但目前視窗沒有 series，不是 parser failure。`partial`
表示 bootstrap/Profile 成功，但至少一個後續查詢失敗；`failed` 表示無法建立可信的
version/Profile 結果。

## Review and Promotion

分享 report 前仍應人工檢查其 schema 與組織政策。Project Maintainer 應把報告與以下
資料一起審查：exact release candidate commit、官方版本文件、TLS/權限模式、非零負載
結果、range boundary、token expiry、代表性 4xx/5xx/timeout，以及正式 validation
record。只有完成適用 gate 後，才能更新 Profile `tested_builds` 或
`sandbox_certified`；fixture/CE/probe report 本身都不會自動修改這些欄位。
