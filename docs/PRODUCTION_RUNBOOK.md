# Production Runbook

This runbook applies to Dell ECS Metrics Exporter deployments on Linux
bare-metal/systemd, Docker/OCI, or Kubernetes/Helm. It does not replace Dell ECS
operational procedures. The exporter is read-only and stores only an in-memory
cache; Prometheus is the system of record for historical metrics.

## Service objectives and normal state

- `/health` returns HTTP 200 while the process is alive.
- `/api/v1/health` returns `UP` after all required collectors have initialized.
  `DEGRADED` is serviceable but requires investigation. `DOWN` returns HTTP 503.
- Prometheus scrapes every 15–60 seconds and all required
  `ecs_exporter_cache_age_seconds` series stay below the configured
  `staleTolerance`.
- The deployed image or binary version, Git commit, Profile selection, and ECS
  exact build match the approved release record.

The example rules in `deploy/prometheus/alerts.yaml` use the default
15-minute stale tolerance and one-hour maximum stale age. Change both the
configuration and rule thresholds together.

## First response

1. Record alert start time, exporter version, deployment identity, affected
   cluster/collector, and recent configuration or ECS changes.
2. Check liveness, readiness, and metrics. Do not paste bearer tokens into shell
   history:

   ```bash
   curl --fail --silent https://exporter.example/health
   curl --silent https://exporter.example/api/v1/health | jq .
   curl --silent https://exporter.example/metrics |
     rg 'ecs_exporter_(cache_age|collector_errors|api_errors)'
   ```

3. Check only redacted logs. Never attach credentials, authorization headers,
   cookies, complete private URLs, raw Inventory responses, or ECS response
   bodies to an incident.
4. Determine whether the failure is exporter-wide, a single ECS cluster, or a
   single conditional collector such as CE Flux node resources.

## Exporter down

- Bare Metal:

  ```bash
  sudo systemctl status dell-ecs-metrics-exporter
  sudo journalctl -u dell-ecs-metrics-exporter --since '-30 minutes'
  sudo /path/to/release/deploy/bare-metal/verify.sh
  ```

- Kubernetes:

  ```bash
  kubectl -n monitoring get deploy,pod,svc,networkpolicy
  kubectl -n monitoring describe deploy ecs-exporter-dell-ecs-metrics-exporter
  kubectl -n monitoring logs deploy/ecs-exporter-dell-ecs-metrics-exporter --tail=200
  ```

Check configuration validation, secret file ownership/mounts, CA trust, port
binding, memory limits, and immutable image digest. Restart only after recording
the failure and confirming that configuration is valid. Repeated crashes are
not resolved by continuous restarts.

## Cache stale or too old

Use `cluster` and `collector` labels to isolate the source. Check:

- DNS and TCP connectivity from the exporter to the ECS management endpoint;
- certificate validity and the configured CA, without disabling TLS
  verification;
- ECS monitor account status and `SYSTEM_MONITOR`/`SYSTEM_ADMIN` role;
- HTTP 429/5xx trends, collector duration, and configured timeouts/rate limits;
- whether the exact ECS build still selects the expected Profile.

The exporter deliberately retains the last complete cache after a failed
refresh. Never increase `maxStale` solely to silence an incident. If only Flux
node resources return 503 on ECS CE, follow the documented degraded-mode
boundary; do not infer that physical ECS has the same limitation.

## Collector or API errors

Correlate `ecs_exporter_collector_errors_total`,
`ecs_exporter_api_errors_total`, and redacted structured logs. Authentication
errors require checking the secret manager and ECS account, not copying secrets
into the configuration. Pagination, parser, or unknown-version errors are
compatibility incidents: preserve a sanitized response shape through the
approved evidence process, stop unsupported Profile claims, and open a governed
specification change.

## ECS health or capacity alert

Confirm the signal in ECS administration tools before taking storage action.
The exporter does not modify quota, bucket, replication, or node state. Route
capacity and health remediation to the ECS operations owner. Missing quota
metrics mean “not configured” and must not be interpreted as zero quota.

## Safe restart and rollback

- Bare Metal: validate the previous release archive and checksum, run
  `install.sh --no-start`, validate config/profiles, then restart and run
  `verify.sh`. The installer preserves configuration and secrets. When run
  without `--no-start`, a failed upgrade start automatically restores the
  previous binary, profiles, and systemd unit.
- Kubernetes: roll back to a previously verified image digest and Helm
  revision. Do not use a mutable `latest` tag.
- Docker: recreate the container from the prior verified digest with the same
  read-only mounts and secret references.

After rollback, confirm liveness, `UP` readiness, Inventory authentication,
metric parsing, cache freshness, and Prometheus target health.

## Credential and certificate rotation

Stage new secret files with mode `0640` and ownership `root:ecs-exporter` for
Bare Metal, or update the external secret in Kubernetes. Restart or roll the
deployment, verify successful ECS login, then revoke the old credential. For CA
rotation, install the new trust chain before ECS switches certificates. Never
use `tls.verify: false` as a migration step.

## Backup and disaster recovery

Back up versioned configuration, Helm values, CA certificates, alert rules, and
secret-manager references according to the organization policy. The in-memory
cache is intentionally not backed up. Recovery consists of verifying a signed
release, restoring configuration and secret references, deploying, and waiting
for a complete fresh cache.

## Escalation and closure

Escalate authentication, authorization, secret exposure, signing, CI identity,
or artifact integrity issues to the Security Reviewer. Escalate unknown ECS
versions or response-contract changes to the Project Maintainer. Close an
incident only after the root cause, evidence classification, remediation,
rollback status, and any Profile/support impact are documented.
