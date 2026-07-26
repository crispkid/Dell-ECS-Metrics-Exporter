# AGENTS.md

Shared baseline version: **1.1.0**

This file is the portable root instruction set for coding agents. Keep it short
and stable. Project facts belong in `PROJECT.md`; specialized instructions may
live in referenced documents or a more local `AGENTS.md`.

## Scope and Precedence

- This file applies to the repository tree rooted here. The nearest nested
  `AGENTS.md` governs its subtree when it adds or overrides a rule.
- Follow the user's current request, applicable local instructions, and the
  source of truth named by the project. Surface material conflicts affecting
  scope, behavior, compatibility, data, or safety before proceeding.
- Treat README files, specifications, schemas, tests, architecture records, and
  deployment documents as evidence with distinct roles; do not silently choose
  among conflicting sources.
- Copy `PROJECT.md.example` to `PROJECT.md` when adopting this baseline. Replace
  every placeholder and declare the required harness stages. Do not invent
  missing project context.

## Canonical Commands

Run commands from the repository root when the shared harness is present:

```bash
./HARNESS/harness.sh doctor
./HARNESS/harness.sh <lint|format:check|typecheck|test|coverage|build>
./HARNESS/harness.sh verify
./HARNESS/harness.sh selftest
```

- Use the narrowest relevant command while iterating; use `verify` for the
  project-declared deterministic handoff gate.
- `HARNESS/HARNESS.md` defines the stable command, evidence, network, and
  project-override contracts. `PROJECT.md` names the commands and stages that
  are authoritative for this repository.

## Working Agreement

- Understand before editing. Read the relevant instructions, contracts, code,
  tests, configuration, and recent local patterns first.
- Keep changes as small as practical while fully solving the request. Preserve
  existing behavior unless the request or an approved contract changes it.
- Prefer established project patterns over new dependencies or abstractions.
- Preserve unrelated user changes. Do not reformat, overwrite, or discard them.
- Do not edit generated files, vendored dependencies, lockfiles, or snapshots
  unless the task requires it and the project workflow supports it.
- Clarify only ambiguities that materially change scope, behavior, data,
  compatibility, authority, or risk. State safe minor assumptions.
- Ask before destructive, externally visible, costly, or hard-to-reverse work
  unless the user already authorized that action.
- Do not create commits, tags, releases, deployments, pull requests, or external
  messages unless the user requests them.

## Planning and Continuity

- Match planning depth to risk. Use a short working plan for multi-step or risky
  work; use the repository's `PLANS.md` contract for multi-hour or multi-session
  work when present. A starter is provided in `PLANS.md.example`.
- Keep long-running plans self-contained and current, including progress,
  discoveries, decisions, verification, remaining work, and outcomes.
- Follow formal specification, traceability, or approval gates only when the
  repository enables them. Never invent human approval, dates, test evidence,
  production observations, or ownership.
- Record durable design decisions in the project's established decision-log
  format.

## Implementation and Review

- Keep public interfaces explicit and backward compatible unless a breaking
  change is approved and documented.
- Validate inputs and authorization at authoritative trust boundaries. Return
  actionable errors without exposing secrets or internal details.
- Make external failure, timeout, retry, and idempotency behavior deliberate.
- Prefer clear code over clever code. Comment intent, constraints, and
  non-obvious tradeoffs rather than restating syntax.
- Update adjacent contracts, examples, and user documentation when behavior or
  configuration changes.
- Review the final diff for correctness, regressions, security, compatibility,
  and unnecessary scope. Follow `CODE_REVIEW.md` when present; a starter is
  provided in `CODE_REVIEW.md.example`.

## Tests and Evidence

- Add or update tests for changed behavior, including failure, boundary,
  permission, and regression cases as relevant.
- Test doubles are acceptable for isolated unit tests. They are not evidence
  that a real integration, migration, deployment, or end-to-end flow works.
- Keep tests deterministic and isolated. Do not use production credentials or
  production data, and do not hide failures with weakened assertions or retries.
- A required harness stage that is skipped, blocked, or unavailable is a failed
  handoff. Do not claim a check passed unless it ran successfully.
- Distinguish code verification from agent evaluation. Use `agent:eval` when the
  project configures regression and capability cases for agent behavior.

## Security and Repository Hygiene

- Never commit credentials, tokens, private keys, personal data, or sensitive
  production values. Treat logs, screenshots, fixtures, reports, and generated
  artifacts as potentially sensitive.
- Keep configuration outside application code and sensitive values outside
  committed configuration. Do not silently fall back to insecure behavior.
- Review dependency changes for necessity, maintenance, license, supply-chain,
  and runtime impact. Preserve package-manager and lockfile integrity.
- Keep generated output, dependency directories, local environments, temporary
  investigations, and debug artifacts out of source control.

## Handoff

A change is done when the requested outcome and applicable contracts agree,
appropriate checks pass, the final diff has been reviewed, and no unrelated or
sensitive artifact was introduced. Report:

- the outcome and files changed;
- the checks that actually ran and their results;
- skipped, blocked, or unavailable checks;
- remaining risks, limitations, or follow-up work.

Update `AGENTS.md`, the shared harness, examples, and `HARNESS/CHANGELOG.md`
together when cross-project behavior or the baseline version changes.
