# Portable Project Harness

Baseline version: **1.1.0**

The research rationale and review date are recorded in
`HARNESS/BEST_PRACTICES.md`.

This directory provides a stable verification surface for agents, developers,
and CI while delegating project-specific tools to the repository. It contains
no product, team, language, deployment-provider, or external-service assumption.

The portability boundary is:

- `AGENTS.md`, `PROJECT.md.example`, the documented harness, and starter
  templates are shared baseline files.
- `PROJECT.md`, `HARNESS/config.env`, `HARNESS/evals/`, and active governance
  records are project-owned.
- Existing package scripts, Make targets, tool configuration, and CI workflows
  remain the implementation of each project check.

## Quick Start

From the repository root:

```bash
cp PROJECT.md.example PROJECT.md
cp HARNESS/config.env.example HARNESS/config.env
# Fill every PROJECT.md decision and configure the real required stages.
./HARNESS/harness.sh selftest
./HARNESS/harness.sh doctor
./HARNESS/harness.sh verify
```

Optional reusable contracts:

```bash
cp CODE_REVIEW.md.example CODE_REVIEW.md
cp PLANS.md.example PLANS.md
```

Before enabling CI, merge `.gitignore.example` into the project's existing
ignore rules so `test-results/harness/` and platform metadata remain untracked,
then verify report retention and redaction.

## Stable Commands

```text
doctor                 Validate harness configuration, paths, and environment
environment:doctor     Show and validate execution prerequisites
repo:doctor            Inspect version-control hygiene
governance:doctor      Validate optional specification/traceability records
governance:approved    Validate the active change's human approval record
lint                   Run configured or detected lint checks
format:check           Check formatting without rewriting files
typecheck              Run configured or detected type checks
test                   Run unit/component tests
coverage               Run the project coverage gate
build                  Build project artifacts
integration            Run real integration checks
e2e                    Run end-to-end checks
security               Run the project security gate
ci:policy              Run the configured CI workflow policy gate
supply-chain           Run the configured SBOM/provenance gate
deploy:check           Validate deployment artifacts or configuration
verify                 Run the deterministic project handoff gate
all                    Alias for verify
agent:eval:doctor      Validate the agent evaluation dataset and runner
agent:regression       Run the configured agent regression suite
agent:capability       Run the optional capability benchmark
agent:eval             Run configured regression and capability suites
selftest               Test the portable harness with isolated fixtures
version                Show the portable baseline version
help                    Show command help
```

`verify` always runs doctor, repository hygiene, optional governance discovery,
lint, format, typecheck, test, coverage, and build. It runs integration, E2E, CI
policy, supply-chain, security, and deployment checks when they are declared
required. A missing optional implementation is visibly skipped; a required
stage that is skipped, blocked, or unavailable fails the handoff.

`agent:eval` is intentionally separate. Project verification asks whether the
code and artifacts satisfy their contracts; agent evaluation asks whether an
agent follows instructions and reliably reaches the requested outcome.

## Configuration and Trust Boundary

Copy `config.env.example` to `config.env` and commit it for each adopted project.
The file is sourced by Bash and is therefore trusted executable repository code,
not a passive dotenv file. Review changes to it like changes to a build script.
Complex or security-sensitive commands may be placed in project-owned scripts
and referenced from `config.env`.

Never store passwords, tokens, private keys, personal data, production-only
endpoints, or embedded credentials in `config.env`. Supply secrets through the
runtime or CI secret mechanism. Command strings and generated reports may also
be sensitive; do not embed secrets in command-line arguments.

Common settings:

- `HARNESS_PROJECT_NAME`: display name only.
- `HARNESS_REQUIRED_PATHS`: colon-separated project paths checked by `doctor`.
- `HARNESS_EXPECTED_BASELINE_VERSION`: detects copied baseline drift in the
  harness and `AGENTS.md`.
- `HARNESS_REQUIRED_STAGES`: comma-separated mandatory stages: `lint`, `format`,
  `typecheck`, `test`, `coverage`, `build`, `integration`, `e2e`, `ci-policy`,
  `supply-chain`, `security`, and `deploy`. Use `none` only for a deliberate
  no-gate project documented in `PROJECT.md`.
- `HARNESS_FAIL_ON_SKIP`: fail every invoked skipped stage.
- `HARNESS_REQUIRED_TOOLS`: comma-separated executable names checked by doctor.
- `HARNESS_ENVIRONMENT_COMMAND`: project-owned toolchain/version validation.
- `HARNESS_COMMAND_TIMEOUT_SECONDS`: zero disables the timeout; a positive value
  requires `timeout` or `gtimeout` on `PATH`.
- `HARNESS_*_COMMAND`: the project implementation for each check.

Paths in `HARNESS_REQUIRED_PATHS` may contain spaces but not colons.

## Command Resolution

Each standard project stage resolves in this order:

1. Its `HARNESS_*_COMMAND` value.
2. A matching root Make target.
3. Matching scripts in the root or shallow child `package.json` files.
4. Conservative conventions for configured Python, Go, or Rust projects.
5. A visible skip when no implementation is available.

Explicit commands are recommended for monorepos, nonstandard layouts, multiple
test tiers, frozen/offline dependency rules, or environment setup. Set a command
to `skip` only when the omission is intentional and documented.

Automatic detection supports:

- Node.js package scripts at the root and up to two directory levels below it.
  `packageManager` takes precedence; otherwise lockfiles select npm, pnpm, Yarn,
  or Bun.
- Python projects with matching pytest, Ruff, Black, mypy, and coverage
  configuration. A local `.venv` or `venv` is preferred.
- root Go modules and Rust workspaces using their conventional commands.
- root Makefiles with matching targets.

The harness does not install dependencies, start services, or download browsers.
Package managers and language tools can still download dependencies unless the
project configures frozen or offline behavior.

## Environment, Timeout, and Network Contract

`environment:doctor` records the OS and Bash version, checks required tools, and
runs the optional project environment command. Exact versions should also live
in version-manager files and `PROJECT.md`.

`HARNESS_NETWORK_POLICY` accepts:

- `deny`: block stages listed as network-dependent.
- `declared`: permit the stages listed in `HARNESS_NETWORK_STAGES` and mark the
  rest as network-free by contract.
- `allow`: permit network use for all stages.

The harness exports `HARNESS_NETWORK_ALLOWED=true|false` to project commands and
records it in evidence. This is a contract and routing guard, not an operating
system sandbox; use CI/container/sandbox controls when network denial must be
technically enforced.

Service-dependent checks must fail or report blocked when credentials, services,
browsers, containers, or network prerequisites are missing. They must not become
passing results through mocks or static-only smoke checks.

## Machine-Readable Evidence

`verify` and `agent:eval` write JSON evidence by default:

```text
test-results/harness/verify.json
test-results/harness/agent-eval.json
```

Each report contains the harness version, run type, UTC timestamps, repository
revision when available, OS, Bash and detected toolchain versions, overall
result, and stage records with status, exit code, duration, network declaration,
command, and detail.
Statuses distinguish `passed`, `failed`, `skipped`, and `blocked`.
Project commands may exit with code `3` to report a missing prerequisite as
`blocked`; other non-zero codes are failures. A blocked required stage still
fails the overall handoff.

Configuration:

- `HARNESS_WRITE_REPORTS=false` disables reports.
- `HARNESS_REPORT_DIR` changes the output directory.
- `HARNESS_REPORT_FILE` overrides the filename for the current invocation.

Reports are evidence, not proof by themselves. Protect and redact them as
potentially secret-bearing outputs. Project commands should link their JUnit,
coverage, SARIF, SBOM, provenance, screenshots, and artifact checksums from the
project-designated evidence location.

## Agent Evaluation Contract

To adopt agent evaluation:

1. Copy `HARNESS/templates/evals/` to `HARNESS/evals/` and create real cases.
2. Configure `HARNESS_AGENT_REGRESSION_COMMAND`.
3. Optionally configure `HARNESS_AGENT_CAPABILITY_COMMAND`.
4. Set `HARNESS_AGENT_TRIALS` and document metrics and thresholds in
   `PROJECT.md`.
5. Run `agent:eval:doctor`, then `agent:eval`.

Evaluation cases should define the task, isolated fixture, allowed and forbidden
changes, observable success state, graders, retained artifacts, and metrics.

- Regression suites protect behavior that should remain reliable and normally
  use a near-100% threshold.
- Capability suites measure difficult behavior and may intentionally have a low
  success rate. Their runner should exit non-zero only for runner failure or an
  explicitly configured threshold.
- Coding evals should prefer deterministic state and test graders. Use
  human-calibrated model graders for subjective properties.
- Because agent behavior is non-deterministic, runners should execute the
  configured number of trials and report pass@1 and consistency metrics such as
  pass^k when appropriate.
- Retain redacted transcripts for diagnosis, but grade final outcomes rather
  than trusting an agent's self-report.

## Test and Coverage Contract

- `test` covers isolated unit and component behavior. Test doubles are allowed
  when they improve isolation and determinism.
- `coverage` enforces `HARNESS_COVERAGE_THRESHOLD` through the project's tool.
- `integration` crosses real configured component or service boundaries.
- `e2e` exercises a deployed or locally running user flow and validates final
  system state.
- Static checks and mocked unit tests are not integration, E2E, migration,
  deployment, or release evidence.
- Do not weaken assertions, delete valuable tests, retry away failures, or lower
  thresholds merely to make the gate pass.

## Optional Governance Contract

Governance is adaptable rather than required for every repository:

- `auto` skips governance when none of its control files exist; a partial set
  fails validation.
- `required` requires the complete configured set.
- `off` disables governance checks.

The default set is `SPECIFICATION.md`, `SPEC_CHANGELOG.md`,
`DEVELOPMENT_PLAN.md`, `TEST_PLAN.md`, and `TRACEABILITY.md`. The active change
resolves from the explicit environment value, `HARNESS/ACTIVE_CHANGE`, a
structured changelog marker, or opt-in legacy discovery.

`governance:approved` requires structured Gate 2 and Gate 4 status, approver,
and ISO approval date in the active plan section. The harness never creates or
infers approval. Starter files live under `HARNESS/templates/governance/`.

## CI and Supply Chain

Run `verify` for the deterministic local gate and service-dependent stages in
separate jobs where practical. Configure `ci:policy` and `supply-chain` with the
project's selected tools rather than relying on fragile generic YAML parsing.
A GitHub Actions policy starter lives under `HARNESS/templates/ci/`.

The canonical baseline itself should test:

- Bash 3.2 on macOS and current Bash on Linux.
- ShellCheck.
- Isolated harness self-tests.
- A real representative project for every supported ecosystem.
- JSON evidence parsing and failure-path reporting.

Projects should adopt least-privilege CI tokens, immutable third-party action
revisions, workload identity instead of long-lived cloud credentials, locked
dependencies, and appropriate checksum, SBOM, provenance, signing, and
verification policies.

## Supported Runtime

- macOS and Linux with Bash 3.2 or newer are supported.
- Native Windows shells are not supported; use WSL.
- Standard `awk`, `find`, `grep`, `sed`, and core filesystem utilities are
  required.
- Git is optional for execution but required for revision and repository
  hygiene evidence.
- Optional language, container, browser, scanner, and service tools are required
  only by stages that use them.

## Baseline Versioning and Extension

- `./HARNESS/harness.sh version` prints the shared version.
- Keep `HARNESS_EXPECTED_BASELINE_VERSION` in committed project configuration.
- Record cross-project changes in `HARNESS/CHANGELOG.md` using semantic versioning.
- Update the markers in `AGENTS.md`, `PROJECT.md.example`, `harness.sh`,
  `config.env.example`, this document, tests, and changelog together.
- Project-only behavior belongs in `PROJECT.md`, project scripts, eval cases, or
  `config.env` and does not require a shared baseline version change.

New shared checks should be deterministic, non-destructive by default, runnable
from the repository root, explicit about prerequisites and network access, and
covered by self-tests. Add a shared command only when it is meaningful across
projects.

Implementation modules live under `HARNESS/lib/`: `adapters.sh` owns ecosystem
detection, `governance.sh` owns repository/governance policy, and `reporting.sh`
owns evidence generation. Their shell functions are internal; the documented
CLI commands are the stable interface.
