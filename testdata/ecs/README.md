# Dell ECS Contract Fixtures

本目錄包含兩類 fixture。`synthetic-document-derived` 使用 RFC 5737 位址、
`.example.invalid` 名稱與合成 UUID，依 Dell 官方文件製作；
`redacted-sandbox-derived` 只保留已授權隔離 sandbox response 的必要欄位與
去識別化值。兩類 fixture 都不含客戶或 production ECS 資料，也不代表整個
Profile 或修正後 Exporter integration 已通過。

## Layout

- `common/`：ECS 3.6 官方 REST API 已記錄、且供後續 Profile 候選繼承的 Management
  API fixtures。
- `ecs-3.6/`、`ecs-3.7/`、`ecs-3.8.0/`、`ecs-3.8.1/`：版本偵測、Flux 與版本特有
  failure fixtures。
- `ecs-3.8.0.3-live/`：只保留 ECS CE exact build response 的去識別化必要欄位，
  用於 top-level quota/billing 與 known-size billing KB regression。
- `ecs-3.8.1.4-live/`：只保留 ECS CE exact build 的版本、top-level quota、
  known-size billing 與單 VDC RG response shape；Flux HTTP 503 不轉成成功 fixture。
- `manifest.schema.json`：每個 fixture manifest 的結構契約。

## Rules

- 不得加入 token、Cookie、Authorization header、password、private endpoint、
  customer name、真實 IP、未遮罩 owner 或 raw production response。
- Dell 文件中的部分 JSON example 本身有缺逗號等格式錯誤；fixture 必須是有效 JSON，
  但保留 number/string 混合型態來驗證 tolerant numeric decoder。
- Fixture 只能證明 parser/mapper 行為。Profile 的 `tested_builds` 在真機 sandbox
  通過前必須保持空陣列。
- 真機 response 若需轉為 fixture，必須先經 owner 核准、去識別化、secret scan 與
  schema review，並記錄精確 ECS build 與 fixture hash。
- `redacted-sandbox-derived` fixture 只證明 manifest 明列的 exact build、logical
  mapping 與 response shape/value；partial live fixture 不等於整個 Profile
  certification，也不能單獨填入 `tested_builds` 或設定 `sandbox_certified=true`。

## Intended Assertions

- `-1` quota 轉為 `configured=false`、Inventory `null` 且不輸出 metric。
- Flux parser 按 `Columns` 對應 `Values`，不可假定欄位順序。
- Flux numeric value 可為字串或 number。
- Node CPU、memory、network 與 conditional disk fixtures 分別模擬 ECS
  `keep()` 後的回應，確保 `cpu`、`interface`、`path` 維度不會因合併查詢遺失。
- 3.7／3.8.0 interval-rate capability 必須停用。
- 3.8.1 的 range fixture 只能打開 contract path，不能取代 live range-boundary test。
- `secretKeys` 永遠不出現在 fixture 或 internal model。
- Performance fixture 保留來源 VDC/Namespace scope；VDC throughput/latency 與
  VDC/Namespace request rates 必須映射 Gauge，不得展開為 Bucket。VDC latency
  以 `id=ttfb_read`／
  `id=ttlb_write` 區分讀寫；Namespace 只使用文件明列帶 `namespace` tag 的
  transaction/error measurements，不捏造 Namespace throughput/latency。
- Bucket billing batch fixture 必須以 Namespace/name 識別 item；sample time 不得
  覆蓋 Bucket inventory 的 last-modified timestamp。
- ECS CE 3.8.0.3 batch request 使用 `{"id":[bucket names...]}`，response envelope
  為 plural `bucket_billing_infos`；500/code999 from `null` 不得 fallback。
- ECS CE 3.8.0.3 live billing `9765.625 KB` 必須映射為 10,000,000 bytes；
  此 KB=1024 assertion 不得改寫 billing MB/GB/TB 或 capacity/quota GB 的既有
  decimal contract。
- ECS CE 3.8.1.4 live billing `10240 KB` 必須映射為 10,485,760 bytes；三個
  Bucket 的 0/3072/7168 KB 必須保留 0/1/3 objects，configured 1/2 GB quota
  仍按 API GB 的既有 decimal contract 映射。
- ECS CE 3.8.1.4 單 VDC RG 沒有 `rglinkStatus` 或 RPO 時，pending bytes 可為 0，
  但不得捏造 `ecs_replication_status` 或 lag。
