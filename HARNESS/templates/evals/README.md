# Agent Evaluation Starter

Copy this directory to `HARNESS/evals/`, rename `case.yaml.example`, and create
cases from real recurring failures and important project workflows. Keep
fixtures isolated from production data and credentials.

`agent:eval` is distinct from `verify`:

- regression cases protect behavior that should remain reliable and normally
  gate changes;
- capability cases measure difficult or emerging behavior and may have a low
  pass rate without failing a release;
- the project-owned runner decides thresholds and performs the configured number
  of trials from `HARNESS_AGENT_TRIALS`.

Prefer deterministic outcome/state graders for coding tasks. Use model-based or
human-calibrated graders only for qualities that cannot be reduced to stable
checks. Retain redacted transcripts for diagnosis, but grade the final outcome
rather than trusting an agent's self-report.
