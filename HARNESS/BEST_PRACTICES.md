# Best-Practice Basis

Last reviewed: **2026-07-23**

This file records the external rationale for the portable baseline. It is not a
project execution contract and does not override `AGENTS.md`, `PROJECT.md`, or
the harness command documentation.

## Agent Instructions and Long Work

- [OpenAI: Custom instructions with AGENTS.md](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
  supports concise, practical repository instructions, nested scope, real
  commands, constraints, and completion criteria.
- [OpenAI: Using PLANS.md for multi-hour problem solving](https://developers.openai.com/cookbook/articles/codex_exec_plans)
  supports self-contained living plans with progress, discoveries, decisions,
  verification, and outcomes.

Baseline decision: keep the root `AGENTS.md` focused, move project facts to
`PROJECT.md`, and provide optional plan and review contracts.

## Evaluation and Long-Running Harnesses

- [Anthropic: Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
  distinguishes capability and regression evaluation, outcome and transcript,
  deterministic/model/human graders, and reliability across multiple trials.
- [Anthropic: Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
  supports incremental work, explicit progress artifacts, baseline end-to-end
  checks, and observable verification before completion.
- [Anthropic: Scaling Managed Agents](https://www.anthropic.com/engineering/managed-agents)
  notes that harness assumptions can become stale and favors small, stable
  interfaces between execution, session evidence, and orchestration.

Baseline decision: separate deterministic project verification from agent
behavior evaluation, retain outcome evidence, distinguish regression from
capability suites, and keep CLI interfaces stable while implementation modules
can evolve.

## CI, Provenance, and Reproducibility

- [GitHub Actions secure use reference](https://docs.github.com/en/actions/reference/security/secure-use)
  supports least-privilege workflow tokens, immutable third-party action
  revisions, careful secret handling, and protected environments.
- [GitHub Actions OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)
  supports workload identity in place of long-lived cloud credentials.
- [SLSA v1.2 tracks](https://slsa.dev/spec/v1.2/tracks) defines increasing
  trustworthiness and completeness for source and build provenance.
- [Reproducible Builds documentation](https://reproducible-builds.org/docs/)
  covers deterministic outputs, managed environmental variance, recorded build
  environments, checksums, and verification.

Baseline decision: expose CI-policy and supply-chain gates, record runtime and
stage evidence, and delegate tool-specific enforcement to each project.

## Maintenance Rule

Review these sources before a major baseline release or when repeated real-world
failures show that a rule or harness assumption is no longer useful. Prefer
removing obsolete complexity over accumulating model-specific workarounds.
