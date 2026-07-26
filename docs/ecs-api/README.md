# Versioned Dell ECS API Mapping

本目錄把 Exporter internal model 與 Dell ECS API 契約拆成：

- `common-contract.md`：四個 Profile 共用的候選 logical mappings、單位與安全規則。
- `ecs-3.6.md`：ECS 3.6.x 的官方文件驗證結果。
- `ecs-3.7.md`：ECS 3.7.x 的差異、已知 Flux 問題與待驗證項目。
- `ecs-3.8.0.md`：ECS 3.8.0.x 的 Flux 與 Host Header 差異。
- `ecs-3.8.1.md`：ECS 3.8.1.x 的 Flux 修正與保留風險。
- `validation/`：去識別化的真實環境驗證紀錄；每份紀錄必須明列通過範圍與未覆蓋
  的 certification gates。

Mapping 狀態：

- `documented`：可追溯至已讀取的 Dell 官方文件。
- `candidate-inherited`：從較早版本繼承的候選契約，尚未用該版本 REST ZIP 或真機證實。
- `CE-live-observed`：已在 exact ECS CE build 觀察到 response 與 Exporter 行為；
  只適用明列範圍，不代表正式設備或完整 Profile 認證。
- `CE-blocked`：CE 環境未提供或無法執行該來源；必須在正式設備另行驗證。
- `unsupported`：官方來源沒有該 scope/語意，禁止輸出。
- `sandbox-pending`：文件 mapping 已建立，但不能宣稱真機相容。

Committed fixtures 分成 `synthetic-document-derived` 與
`redacted-sandbox-derived`。前者適合 parser/contract tests；後者只保留 exact
build partial-live response 的去識別化必要欄位。兩者都不會單獨構成整個 Profile
certification，也不能取代修正後 Exporter live rerun、正式設備與 reviewer gates。
真實 raw response 不直接提交；`validation/` 只保留經去識別化的結果、結構差異與
evidence boundary。

目前 partial-live records：

- [ECS CE 3.8.0.3](validation/ecs-ce-3.8.0.3-2026-07-25.md)
- [ECS CE 3.8.1.4](validation/ecs-ce-3.8.1.4-2026-07-26.md)
