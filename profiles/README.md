# Dell ECS Compatibility Profiles

本目錄提供 Exporter 選擇 Dell ECS adapter 的 machine-readable 契約。Profile 只描述
版本範圍、transport 差異、capability 與已知限制；實際 URI、欄位及 metric mapping
位於 `docs/ecs-api/`。

## Profiles

| Profile | Version range | Mapping | Evidence status |
|---|---|---|---|
| `ecs-3.6` | `>=3.6.0.0, <3.7.0.0` | `docs/ecs-api/ecs-3.6.md` | 共用功能 `validated-shared`；保留 3.6 interval/native 與 Dashboard 差異 |
| `ecs-3.7` | `>=3.7.0.0, <3.8.0.0` | `docs/ecs-api/ecs-3.7.md` | 共用功能 `validated-shared`；interval rate 依已知問題維持 unavailable |
| `ecs-3.8.0` | `>=3.8.0.0, <3.8.1.0` | `docs/ecs-api/ecs-3.8.0.md` | 共用功能 `validated-shared`；Host Header 與 interval policy 仍版本別處理 |
| `ecs-3.8.1` | `>=3.8.1.0, <3.8.2.0` | `docs/ecs-api/ecs-3.8.1.md` | 共用功能 `validated-shared`；interval rate 仍 conditional |

功能驗證採 `shared-live-any-target-version`：同一 production path 功能只要在任一
目標版本取得真實 CE/appliance evidence，就列入四個 Profile 的
`evidence.shared_validated_capabilities`。完整矩陣見
`docs/ecs-api/feature-validation.md`。

`documentation-verified` 不等同 `sandbox_certified`。`documented_releases` 是供
contract test 使用的官方文件可辨識版本清單，不保證列出每個歷史 patch，也不表示
該 build 已通過 Exporter 測試。`tested_builds` 只記 exact-build 執行紀錄，並不是
共用功能驗證的必要條件；目前所有 Profile 仍為空。
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
目前四個 repository Profile 均未達到完整 Profile certification 條件，但已在
machine-readable evidence 中標示共用功能驗證狀態。

## Validation

`profile.schema.json` 是四個 Profile 的結構契約；`internal/profile` 另執行 strict
decode、range/overlap、capability 與 evidence validation。可用下列命令驗證：

```bash
go run ./cmd/ecs-exporter -profiles-dir profiles -validate-profiles
./HARNESS/harness.sh test
```

通過這些檢查會驗證共享功能清單只引用該 Profile 已提供且非 `unavailable` 的
capability。真實功能狀態依 ECS-011 跨版本繼承；完整 exact-build Profile
certification 仍是另一個獨立狀態。
