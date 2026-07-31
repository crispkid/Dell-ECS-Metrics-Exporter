# Local Prometheus and Grafana test stack

This Compose stack runs Prometheus and Grafana for local Dell ECS Metrics
Exporter testing. It does not contain Dell ECS credentials and does not deploy
the Exporter itself. Prometheus reaches an Exporter published on host port 8080
through `host.docker.internal`.

The stack is for local evaluation only:

- Prometheus listens on `127.0.0.1:9090`.
- Grafana listens on `127.0.0.1:3000`.
- Grafana anonymous access uses the Viewer role with temporary Explore access.
  Viewers cannot save their changes.
- Prometheus data and Grafana state use Docker-managed volumes.
- Prometheus retains local test metrics for seven days.
- The images are pinned to Prometheus `3.13.1` LTS and Grafana `13.1.0`.

For a fixture-based end-to-end demonstration without a Dell ECS appliance, use
the synthetic ECS HTTPS server and Exporter configuration in this directory.
This proves the local fixture-to-Exporter-to-Prometheus-to-Grafana path only. It
is not Dell ECS hardware certification or production integration evidence.

## 1. Prerequisites

- Docker Desktop or Docker Engine with Compose.
- Dell ECS Metrics Exporter running on host port 8080.
- Exporter `prometheus.protected` set to `false`. This local stack does not
  send a Bearer token.

Confirm the Exporter before starting the stack:

```bash
curl --fail --silent --show-error http://127.0.0.1:8080/health
curl --fail --silent --show-error http://127.0.0.1:8080/metrics |
  rg '^ecs_exporter_build_info'
```

The Exporter may be a local binary or another container that publishes
container port 8080 to host port 8080.

## 2. Validate and start

Run commands from the repository root:

```bash
docker compose -f deploy/local/compose.yaml config --quiet
docker compose -f deploy/local/compose.yaml pull
docker compose -f deploy/local/compose.yaml up -d
docker compose -f deploy/local/compose.yaml ps
```

The default URLs are:

| Service | URL | Authentication |
|---|---|---|
| Prometheus | <http://127.0.0.1:9090> | None; loopback only |
| Prometheus targets | <http://127.0.0.1:9090/targets> | None; loopback only |
| Grafana | <http://127.0.0.1:3000> | Anonymous Viewer; Explore enabled, saving disabled |

Grafana automatically provisions:

- a default Prometheus datasource using `http://prometheus:9090`.

No Grafana login is required. Open **Explore**, select the Prometheus datasource,
and query the Exporter metrics. Grafana Dashboard JSON/provisioning remains
outside the V1 product scope.

To use different host ports:

```bash
PROMETHEUS_PORT=19090 GRAFANA_PORT=13000 \
  docker compose -f deploy/local/compose.yaml up -d
```

## 3. Verify the integration

Check both services:

```bash
curl --fail --silent --show-error http://127.0.0.1:9090/-/healthy
curl --fail --silent --show-error http://127.0.0.1:3000/api/health
```

Query the Exporter scrape result:

```bash
curl --fail --silent --show-error --get \
  --data-urlencode 'query=up{job="dell-ecs-metrics-exporter"}' \
  http://127.0.0.1:9090/api/v1/query |
  jq .
```

The result value must become `1`. Prometheus scrapes every 15 seconds, so wait
for one interval after the Exporter starts.

Useful Grafana Explore queries:

```promql
ecs_cluster_health
```

```promql
100 * sum by (cluster) (ecs_cluster_capacity_used_bytes)
  / sum by (cluster) (ecs_cluster_capacity_total_bytes)
```

```promql
ecs_exporter_cache_age_seconds
```

```promql
increase(ecs_exporter_collector_errors_total[15m])
```

Validate the loaded Prometheus configuration and alert rules inside the
version-pinned image:

```bash
docker compose -f deploy/local/compose.yaml run --rm --no-deps \
  --entrypoint /bin/promtool prometheus \
  check config /etc/prometheus/prometheus.yml
```

The provided alert rules expect
`job="dell-ecs-metrics-exporter"`, which matches this local configuration.

## 4. Run the synthetic end-to-end flow

Build the Exporter:

```bash
./HARNESS/harness.sh build
```

In terminal 1, start the local synthetic Dell ECS HTTPS API. It serves the
repository's ECS 3.6 document-derived fixtures and generates a fresh in-memory
authentication token each time:

```bash
go run ./deploy/local/mock-ecs \
  -listen 127.0.0.1:4443 \
  -fixtures testdata/ecs
```

In terminal 2, provide test-only credentials through the process environment
and start the Exporter:

```bash
ECS_MOCK_USERNAME=monitor \
ECS_MOCK_PASSWORD=synthetic-only \
  ./dist/ecs-exporter \
    -config deploy/local/exporter.simulation.yaml \
    -profiles-dir profiles
```

The mock accepts any non-empty Basic Authentication values. These example
values are not credentials for a real system. The Exporter disables TLS
verification only because the mock generates an ephemeral self-signed
certificate; do not copy that setting to production.

Wait at least 15 seconds for one Prometheus scrape, then verify:

```bash
curl --fail --silent --show-error --get \
  --data-urlencode 'query=up{job="dell-ecs-metrics-exporter"}' \
  http://127.0.0.1:9090/api/v1/query

curl --fail --silent --show-error --get \
  --data-urlencode 'query=ecs_cluster_health{cluster="synthetic-ecs"}' \
  http://127.0.0.1:9090/api/v1/query
```

Open Grafana at <http://127.0.0.1:3000>, select **Explore** and run:

```promql
ecs_cluster_health{cluster="synthetic-ecs"}
```

```promql
100 * ecs_cluster_capacity_used_bytes{cluster="synthetic-ecs"}
  / ecs_cluster_capacity_total_bytes{cluster="synthetic-ecs"}
```

```promql
ecs_node_cpu_usage_ratio{cluster="synthetic-ecs"}
```

```promql
ecs_bucket_used_bytes{cluster="synthetic-ecs"}
```

Expected synthetic values include cluster health `1`, capacity utilization
approximately `73.33%`, node-a CPU usage `0.275`, and two bucket series. Stop
the Exporter and mock with `Ctrl-C`; Prometheus and Grafana may remain running.

## 5. Troubleshooting

### Docker credential helper prevents image pull

If `docker compose pull` reports `error getting credentials`, unlock the
operating-system keychain or repair the Docker Desktop registry login. Do not
delete credential files without first confirming what other registries use
them. If both pinned images are already present locally, start without pulling:

```bash
docker compose -f deploy/local/compose.yaml up -d --pull never
```

### Exporter target is DOWN

Verify the Exporter is published on host port 8080:

```bash
curl --fail http://127.0.0.1:8080/metrics
docker compose -f deploy/local/compose.yaml exec prometheus \
  wget -qO- http://host.docker.internal:8080/health
```

On Linux, the Compose `host-gateway` mapping requires a current Docker Engine.
Do not replace `host.docker.internal` with `localhost`: inside the Prometheus
container, `localhost` means the Prometheus container itself.

### Grafana has no Dell ECS data

Confirm, in order:

1. The Prometheus target is `UP`.
2. `ecs_exporter_build_info` exists in Prometheus.
3. Exporter `/api/v1/health` is `UP` or the documented test-only `DEGRADED`.
4. Grafana Explore uses the provisioned Prometheus datasource.
5. The exact ECS Profile supports the metric. Optional or unavailable metrics
   produce an empty panel rather than a fabricated value.

View logs:

```bash
docker compose -f deploy/local/compose.yaml logs --tail=200 prometheus
docker compose -f deploy/local/compose.yaml logs --tail=200 grafana
```

## 6. Stop or reset

Stop containers while keeping Prometheus/Grafana data:

```bash
docker compose -f deploy/local/compose.yaml down
```

Restart later with `up -d`. To permanently delete the local Prometheus history,
Grafana database, and Compose volumes:

```bash
docker compose -f deploy/local/compose.yaml down --volumes
```

The `--volumes` operation is destructive for this local test stack. It does not
delete Exporter configuration, ECS secrets, or data stored in Dell ECS.
