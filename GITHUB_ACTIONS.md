# GitHub Actions Security and Delivery Contract

Active Change: ECS-008

本文件定義 `.github/workflows/` 的最低安全與交付要求。`ci.yml` 已實作未信任 PR
可執行的唯讀 verification workflow；真實 ECS integration 與 release/publish
workflow 使用受保護 environment 與 fail-closed exact-build job。

## Workflow Boundaries

- Pull request workflow 執行 deterministic lint、format、typecheck、unit/component、
  coverage、build、secret/security 及 deployment static checks，不使用 production
  secrets。
- 真實 ECS integration 只在受保護 environment、受信任 branch/event 與明確核准後
  執行；fork/untrusted PR 永遠不能取得 sandbox 或 production credentials。
- Release workflow 只接受 protected tag/branch，重新驗證 commit，產生 immutable
  image digest、Helm package、checksums、SBOM 與 provenance。
- Production deployment 若未來採 CI，自 release 分離並使用 protected environment；
  不得讓 PR workflow 直接部署。

## Permissions and Authentication

- Workflow/job 預設明列 `contents: read`，未列出的 `GITHUB_TOKEN` 權限皆為 `none`。
- 只有需要的 release job 可取得 `packages: write`、`id-token: write` 或 attestation
  權限；不得在 workflow level 全域提高。
- Cloud/registry 優先使用 OIDC/workload identity；禁止新增長效 credential，除非
  Security Reviewer 記錄必要性、scope、rotation、revocation 與到期日。
- Environment secrets 只能提供給受保護 job，且不得透過 command line、debug log、
  artifact、cache key 或 output 暴露。

## Action and Script Integrity

- 第三方 actions 與 reusable workflows 固定到已審查的完整 commit SHA，旁註人類
  可讀 release；禁止只使用 mutable tag 或 branch。
- GitHub-owned actions 也必須固定 SHA，更新透過受審查 dependency PR。
- 不把 issue、PR title/body、branch、commit message 或其他 untrusted context 直接
  插入 shell；透過 environment/input 傳遞並由接收程式驗證。
- 禁止在 privileged job checkout 或執行 untrusted fork code。
- Downloaded tools/artifacts 必須驗證 checksum/signature，且版本固定。

## Required Jobs After Product Bootstrap

1. Harness selftest/doctor and governance validation.
2. Go formatting、static analysis/typecheck、unit/component tests 與 80% coverage gate。
3. Reproducible binary/container build。
4. Secret scan、dependency vulnerability/license scan 與 container scan。
5. Helm lint/template、Kubernetes schema/policy 與 probe/security-context validation。
6. CI policy scan，檢查 permissions、SHA pinning、untrusted interpolation 與 privileged
   event boundaries。
7. Supply-chain job，產生 checksums、SPDX/CycloneDX SBOM、provenance，並依發布前
   選定政策簽章與驗證。
8. 受保護且為 production release prerequisite 的 exact-build ECS integration；
   可獨立執行的 deployed E2E 與 target-scale gate。

`HARNESS_CI_POLICY_COMMAND` 與 `HARNESS_SUPPLY_CHAIN_COMMAND` 必須指向實際、可重現
的工具或 project-owned script；在工具未選定前不得把這兩個 stage 宣告為通過。

## Artifacts and Retention

- Unit/coverage/Harness evidence 預設保留 30 天。
- Security scan、SBOM、provenance、checksums 與 release metadata 預設保留 90 天，
  若組織政策要求更久則依政策。
- Artifact 上傳前遮罩 private endpoint、username、token、Authorization header、
  Cookie、raw ECS response、inventory owner 與其他 production data。
- CI cache 不得包含 secrets、credential files、raw responses 或未信任可執行檔。

## Branch and Release Protection

- Required checks 必須由受保護 workflow 產生；不得允許一般 PR 修改 workflow 後以
  降低 gate 的版本自我核准。
- Release environment 需要 Project Maintainer 核准；涉及 credential、permission、
  signing 或 production deployment 時另需 Security Reviewer。
- `ecs-certification`、`ecs-ce-compatibility`、`ecs-e2e`、`ecs-performance`
  environments 只允許受控的 self-hosted Linux runners。CE 只允許已知
  `node-resources` collector error 導致的 DEGRADED。Tag 前必須先把同一 full
  commit 的候選版本部署至 E2E/target-scale 環境；scripts 會拒絕 commit 不符的
  exporter。
- Release 使用 Semantic Versioning，metric/API/config breaking change 必須已有
  specification、migration/deprecation 與 traceability。
- 發布只引用已通過相同 commit SHA 的 evidence；不得重新標記未驗證 image。

## Incident Response

若發現 credential 或 supply-chain compromise，立即停止相關 workflow、撤銷 token/
identity、隔離 artifacts、保存經遮罩 evidence、通知 Project Maintainer 與 Security
Reviewer，並以新的 governed change 記錄修復與重發程序。
