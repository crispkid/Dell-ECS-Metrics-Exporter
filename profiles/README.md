# Dell ECS Compatibility Profiles

本目錄提供 Exporter 選擇 Dell ECS adapter 的 machine-readable 契約。Profile 只描述
版本範圍、transport 差異、capability 與已知限制；實際 URI、欄位及 metric mapping
位於 `docs/ecs-api/`。

## Profiles

| Profile | Version range | Mapping | Evidence status |
|---|---|---|---|
| `ecs-3.6` | `>=3.6.0.0, <3.7.0.0` | `docs/ecs-api/ecs-3.6.md` | 官方 API/Monitoring 文件；真機未驗證 |
| `ecs-3.7` | `>=3.7.0.0, <3.8.0.0` | `docs/ecs-api/ecs-3.7.md` | 官方索引與已知問題文件；REST ZIP/真機待驗證 |
| `ecs-3.8.0` | `>=3.8.0.0, <3.8.1.0` | `docs/ecs-api/ecs-3.8.0.md` | 官方索引與已知問題文件；ECS CE 3.8.0.3 Management 與修復後 non-empty Bucket partial-live 通過；REST ZIP/正式版 Flux 仍待驗證 |
| `ecs-3.8.1` | `>=3.8.1.0, <3.8.2.0` | `docs/ecs-api/ecs-3.8.1.md` | 官方索引與修正資訊；ECS CE 3.8.1.4 Management/known-size partial-live 通過；CE Flux HTTP 503；REST ZIP/正式設備待驗證 |

`documentation-verified` 不等同 `sandbox_certified`。`documented_releases` 是供
contract test 使用的官方文件可辨識版本清單，不保證列出每個歷史 patch，也不表示
該 build 已通過 Exporter 測試。所有 Profile 的 `tested_builds` 目前均為空。
ECS CE 3.8.0.3 的部分 Management API evidence 未涵蓋 REST ZIP、正式版 Flux、
replication 與完整故障注入，因此不足以加入 `tested_builds`。
`testdata/ecs/ecs-3.8.0.3-live/` 的 redacted partial-live fixtures
只證明 exact build 已列出的 quota/billing shape、`{"id":[...]}` batch contract 與
billing KB multiplier，不等同整個 Profile certification。
ECS CE 3.8.1.4 也只為 exact build
`3.8.1.4.140200.8103892f11b` 提供 Management、known-size billing/quota 與單 VDC
RG partial-live evidence；`testdata/ecs/ecs-3.8.1.4-live/` 不涵蓋 CE 回 HTTP 503 的
Flux、multi-VDC replication 或正式設備，因此同樣不得加入 `tested_builds`。

## Selection Rules

1. 以 bootstrap client 登入後呼叫 `GET /vdc/nodes`。
2. 解析每個 `node[].version` 的前四個數字，保留其餘 build suffix。
3. 所有節點版本落在同一 Profile 才可選用該 Profile。
4. 滾動升級造成 mixed-version 時，只啟用兩個 Profile 的共同 capability，且禁止
   interval-derived Flux rate。
5. 未知版本預設拒絕；不得靜默選用最接近的 Profile。
6. 啟動後定期重新偵測，Profile 切換必須在 probe 成功後原子完成。

ECS 版本不是 Semantic Versioning；例如 `3.6.2.6` 有四段。實作應使用 Dell ECS
product-version parser，不可直接假定 Go SemVer library 能正確接受。

## Capability Values

- `native`：官方來源直接提供，adapter 仍需 parser/contract test。
- `derived`：由官方欄位明確計算，mapping 必須記錄公式。
- `conditional`：需要額外設定、scope 選擇或真機確認。
- `unavailable`：此 Profile 不得輸出對應 metric 或捏造 Inventory 值。

Runtime 只允許在 Cluster 的 `capabilities.enabledConditional` 明列已實作的
conditional capability；設定不能把 `unavailable` 改成可用，mixed-version 也永遠
停用 interval-derived rates。可設定名稱與 filesystem/network policy 詳見根目錄
`README.md`。

未來只有在 `tested_builds` 非空、`evidence.status=sandbox-verified`、
`fixture_classification=redacted-sandbox-derived`、API reference 已 reviewed 且
`sandbox_certified=true` 同時成立時，strict loader 才接受 sandbox certification。
目前四個 repository Profile 均未達到這些條件。

## Validation

`profile.schema.json` 是四個 Profile 的結構契約；`internal/profile` 另執行 strict
decode、range/overlap、capability 與 evidence validation。可用下列命令驗證：

```bash
go run ./cmd/ecs-exporter -profiles-dir profiles -validate-profiles
./HARNESS/harness.sh test
```

通過這些檢查只能證明 repository contract、synthetic fixtures 與已提交的
redacted partial-live fixtures 一致，不能證明整個 Dell ECS Profile 真機相容性。
