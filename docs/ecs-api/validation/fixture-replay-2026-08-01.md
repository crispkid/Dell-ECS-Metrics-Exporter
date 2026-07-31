# Four-profile Flux Fixture Replay — 2026-08-01

## Classification

- Evidence class: deterministic synthetic fixture regression
- Live ECS evidence: no
- Profile certification effect: none
- Active change: ECS-010
- ECS-011 disposition: regression support only; shared live validation comes from the CE and
  appliance records, not from this synthetic replay by itself

## Scope

The same production collector and parser path was replayed for `ecs-3.6`, `ecs-3.7`,
`ecs-3.8.0`, and `ecs-3.8.1`. Each version-specific `nodes.json` selected its expected
Profile. The shared split fixtures exercised CPU, Memory, Network, VDC core, VDC latency,
and Namespace request mappings.

## Result

All four replay cases passed these deterministic assertions:

- two Nodes were inventoried without exposing their identities in the report;
- CPU, Memory used/total and Network receive/transmit mappings produced the expected counts;
- VDC read/write throughput, four latency dimensions, three VDC request status series and
  three Namespace request status series produced 12 Performance records;
- Profile policy remained version-specific: 3.6 interval rates `native`, 3.7 and 3.8.0
  `unavailable`, and 3.8.1 `conditional`;
- all-null Flux placeholders were accepted as a successful empty window;
- an injected HTTP 503 produced a redacted `partial` report without leaking the API logical
  name or response content;
- conditional Disk required an explicit filesystem allowlist.

The report serialization regression rejects fixture endpoint, Node, VDC, Namespace and raw
metric values. This proves the probe's redaction and mapping behavior under controlled input.
It does not prove an ECS 3.6/3.7/3.8.0 appliance implements the same Flux response contract.

A separate local synthetic HTTPS calibration exercised the compiled Probe through the real ECS
client: TLS exception, login, version/Profile selection, Cluster/Node GETs, three Node Flux
queries, three Performance Flux queries, JSON report and logout all passed. The report contained
the synthetic build and expected counts but no endpoint, resource identity or values. This is
still deterministic path evidence, not a live ECS appliance observation.
