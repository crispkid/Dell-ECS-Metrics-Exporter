# Execution Plans

本專案對跨多個工作階段、影響 public contract、Dell ECS adapter、認證、安全、
部署或資料相容性的工作使用自包含 Execution Plan。一般小型文件修正不強制建立。

## Location and Naming

- Active plan 放在 `plans/`，檔名使用 governance change ID，例如
  `plans/ECS-002.md`。
- 一個 plan 只追蹤一個 active change；不得在同一檔案混合未關聯需求。
- `DEVELOPMENT_PLAN.md` 保留 active governed change 的摘要、approval record 與
  verification 結果；長期工作細節由 `plans/` 中的 living document 承載。

## Required Plan Sections

每份 plan 必須包含以下段落，並以具體事實填寫：

1. **Purpose and Observable Outcome**：說明使用者或 operator 可觀察到的結果，以及
   如何驗證。
2. **Context and Constraints**：列出適用的規格章節、API mapping、相容性、安全、
   資料與授權界線。
3. **Progress**：用 UTC ISO 8601 時間記錄已完成與待完成項目；每次重要停點更新。
4. **Surprises and Discoveries**：記錄與原假設不同的事實及檔案、command 或 API
   evidence。
5. **Decision Log**：記錄決策、理由、被排除方案、日期與 accountable role；不得
   捏造人員核准。
6. **Implementation Plan**：依執行順序列出受影響檔案、public behavior 與資料流。
7. **Verification Plan**：列出 command、fixture、環境前提、成功標準與 evidence
   位置；mock 與真實 integration 必須清楚區分。
8. **Recovery and Idempotence**：說明安全重試、rollback、cleanup、cache recovery
   與設定相容性。
9. **Outcomes and Retrospective**：完成後記錄實際成果、通過的 evidence、未執行或
   blocked checks、剩餘風險與後續工作。

## Project-Specific Planning Rules

- 目標 Dell ECS 版本未確定前，涉及真實 API 的 plan 必須先加入版本、官方文件、
  sandbox credential 與 redacted response fixture 的取得步驟。
- Metric plan 必須明列 name、type、unit、labels、scope、cardinality 與 source
  semantics；來源不是單調累計值時不得規劃 `_total` Counter。
- Collector plan 必須涵蓋 timeout、retry classification、pagination、bounded
  concurrency、single-flight、atomic cache replacement、partial failure 與 stale
  data。
- API plan 必須涵蓋 authentication/authorization、pagination bounds、RFC 9457
  errors、null/missing fields 與不允許由 query 直接觸發 ECS request。
- Deployment plan 必須涵蓋 least privilege、Secret reference、TLS、probe、
  resources、NetworkPolicy、rollback 與 image provenance。
- 不得把 production credential、endpoint、response body、個人資料或 token 放入
  plan 或 evidence。

## Maintenance

Plan 是可恢復工作的 living document，不是一次性提案。每個 meaningful stopping
point 都要同步 `Progress`、discoveries、decisions 與 verification。完成狀態必須以
observable evidence 為依據；僅有實作者宣稱不構成完成證據。
