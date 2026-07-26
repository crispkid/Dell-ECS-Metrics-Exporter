# Portable Harness Changelog

All notable cross-project changes to the shared agent and harness baseline are
recorded here. Project-only command changes belong in that project's history.

## 1.1.0 - 2026-07-23

- Shortened the portable `AGENTS.md` and moved long-running plan and code-review
  guidance into reusable starter contracts.
- Added machine-readable JSON evidence for `verify` and `agent:eval`, including
  stage status, exit code, duration, network declaration, revision, and runtime.
- Added separate regression and capability commands for project-owned agent
  behavior evaluation datasets and runners.
- Added required-tool checks, optional environment validation, portable opt-in
  command timeouts, and an explicit network-use contract.
- Added configurable CI-policy and supply-chain stages plus secure CI guidance.
- Clarified that test doubles are valid unit-test evidence but cannot substitute
  for real integration or end-to-end verification.
- Added project-context fields for agent evaluation, evidence retention,
  network use, CI permissions, provenance, and SBOM policy.
- Added portable ignore entries for generated harness evidence and macOS
  metadata.
- Recorded the official and primary-source rationale in
  `HARNESS/BEST_PRACTICES.md`.
- Split evidence reporting, ecosystem adapters, and governance/repository policy
  into focused modules under `HARNESS/lib/`.

## 1.0.0 - 2026-07-15

- Established the portable `AGENTS.md` and stable harness command surface.
- Added configurable required stages and fail-on-skip enforcement.
- Added structured active-change discovery and approval records with explicit
  legacy compatibility switches.
- Added baseline version validation and a `version` command.
- Added `packageManager`-aware Node.js command selection.
- Added project context and governance starter templates.
- Documented supported platforms, secret handling, CI expectations, and baseline
  update rules.
