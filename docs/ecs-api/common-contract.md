# Common Dell ECS Adapter Contract

此文件定義四個版本 Profile 的共同候選契約。每個版本文件會覆寫 evidence 狀態與
已知差異。除 ECS 3.6 外，Management API mapping 在取得對應 REST API ZIP 或真機
response 前仍是 `candidate-inherited`。

## Transport and Authentication

| Mapping ID | Contract |
|---|---|
| `MAP-AUTH-001` | HTTPS management port `4443`；`GET /login` 使用 HTTP Basic credential，成功 response 的 `X-SDS-AUTH-TOKEN` 必須作為後續 request header/cookie；以 `Accept: application/json` 呼叫 `GET /user/whoami`，驗證 `user.common_name` 與 `user.roles.role[]` 至少含 `SYSTEM_MONITOR` 或 `SYSTEM_ADMIN`；`GET /logout` 結束 token。 |
| `MAP-VERSION-001` | `GET /vdc/nodes`，從每個 `node[].version` 解析四段 product version 與 optional build suffix；不得從 HTTP header 猜版本。 |
| `MAP-FLUX-001` | `POST /flux/api/external/v2/query`；request `Content-Type: application/json`、`Accept: application/json`，body 為 `{"query":"..."}`；requires `SYSTEM_ADMIN` 或 `SYSTEM_MONITOR`。 |

Token、Basic credential、Cookie、完整 private URL 與 raw response body不得寫入 log、
metric label 或 fixture。Redirect 到不同 origin 時不得轉送 token。

## Logical API Mappings

| Mapping ID | HTTP contract | Required source fields | Internal/Prometheus mapping | Missing/error semantics |
|---|---|---|---|---|
| `MAP-CLUSTER-HEALTH-001` | `GET /dashboard/zones/localzone?dataType=current` | `name`、`numNodes`、`numGoodNodes`、`numBadNodes`、`numDisks`、`numGoodDisks`、`numBadDisks` | `ecs_cluster_health` 是 derived Gauge：所有 nodes/disks 都可計數且 bad count 為 0 時為 1，否則為 0；身份使用 VDC `name` | 欄位缺少或無法解析時 collector error，不得假定健康 |
| `MAP-CLUSTER-CAPACITY-001` | `GET /object/capacity` | Legacy nested `cluster_capacity.totalProvisioned_gb`/`totalFree_gb` or ECS 3.8.0 top-level fields with the same names | total/free 依 API 文件的 GB 轉 bytes；used = max(total - free, 0)；輸出 total/used/available Gauges | free > total、負值、invalid number 或同時出現兩種 envelope 使 snapshot 失敗 |
| `MAP-NODE-INFO-001` | `GET /vdc/nodes` | `node[].nodeid`、`nodename`、`rackId`、`version`、`isLocal`；address fields optional | node identity、rack、version；version 也供 Profile selection | `status` 是 lockdown 狀態，不得當健康狀態 |
| `MAP-NODE-HEALTH-001` | `GET /dashboard/zones/localzone/nodes?dataType=current` | node identity、`healthStatus`、good/bad disk counts；records 可位於 `node[]`、`nodes[]` 或 ECS 3.8.0 HAL `_embedded._instances[]` | `ecs_node_health`；只有明確的 healthy/good enum 映射 1，其他已知狀態為 0 | 未知 enum/缺欄位為 collector error，不能映射 0 |
| `MAP-NODE-SERVICE-001` | 與 Node health 相同 response，僅在版本 response 實際提供 `services[]`/`processes[]` 時使用 | `name`、known `status` | conditional `ecs_node_service_health`；保留 `kind=service|process` | 不猜測額外 URI；未知狀態或重複名稱使完整 Node refresh 失敗 |
| `MAP-NODE-RESOURCE-001` | Flux `monitoring_op` | `cpu.usage_idle` with `cpu=cpu-total`；`mem.used`、`mem.total`；`net.bytes_recv`、`net.bytes_sent` with `host,node_id,interface` | CPU ratio = `(100-usage_idle)/100`；memory bytes 直接映射；network bytes 是 monotonic Counter，保留 bounded `interface` label | 只取每個 series 的 `last()`；counter decrease 視為 reset；不得把不同 interface 靜默相加 |
| `MAP-NODE-DISK-001` | Flux `monitoring_op` measurement `disk` | `used`、`total` with `host,node_id,path,device` | 只有設定明確 allowlist 的 filesystem/device 才可聚合為 node disk used/total | 未設定 allowlist 時 capability 為 conditional 且不輸出 |
| `MAP-NAMESPACE-INFO-001` | `GET /object/namespaces` | `namespace[].name/id`、quota/default fields optional | Namespace inventory；cluster namespace count 只能從完整成功 list 計算 | Dell 文件中的 JSON example 有格式瑕疵；parser 只接受真實有效 JSON |
| `MAP-NAMESPACE-QUOTA-001` | `GET /object/namespaces/namespace/{namespace}/quota` | Legacy nested `namespace_quota_details.blockSize`/`notificationSize` or ECS 3.8.0 top-level fields | `blockSize` 為 hard quota、`notificationSize` 為 soft notification threshold；目前 public metric 只輸出 hard quota | `-1` 表示未設定：metric 省略，Inventory `null` + configured false；同時出現兩種 envelope 時拒絕 |
| `MAP-NAMESPACE-BILLING-001` | `GET /object/billing/namespace/{namespace}/info?sizeunit=KB`；follow `marker`/`next_marker` when bucket detail requested | `namespace`、`total_size`、`total_size_unit`、`total_objects`、`sample_time`、`uptodate_till` | namespace used bytes/object count；cluster object count以 namespace-level值聚合，不以 bucket sum 取代 | Billing 是非同步 sample；bucket sum 與 namespace total 暫時不一致不是 parser error |
| `MAP-BUCKET-INFO-001` | Enumerate `GET /object/namespaces`, then `GET /object/bucket?namespace={namespace}` with bounded `limit`/`marker` pagination | `object_bucket[].name`、`namespace`、`vpool`、`created`、feature flags | Bucket inventory、namespace bucket count、replication-group reference | ECS 3.8.0 rejects an empty Namespace filter；cross-Namespace items、duplicate/page loops fail the complete snapshot；`owner` 可進 Inventory但禁止作 metric label |
| `MAP-BUCKET-QUOTA-001` | `GET /object/bucket/{bucketName}/quota?namespace={namespace}` | Legacy nested `bucket_quota_details.blockSize`/`notificationSize` or ECS 3.8.0 top-level fields；nested response 可另含 `bucketname`/`namespace` | `blockSize` -> hard quota；`notificationSize` -> soft quota | `-1` 表示未設定；不得輸出 -1、0 或 cluster capacity 替代；同時出現 nested 與 top-level envelope 時拒絕 |
| `MAP-BUCKET-BILLING-001` | Prefer batch `POST /object/billing/buckets/{namespace}/info?sizeunit=KB` with JSON `{"id":[bucket names...]}`；HTTP 404/405/501 or a missing response item falls back to single bucket `GET /object/billing/buckets/{namespace}/{bucket}/info?sizeunit=KB` | ECS 3.8.0 batch success envelope is plural `bucket_billing_infos[]`；inherited candidate `bucket_billing_info[]` remains accepted；single response can use nested `bucket_billing_info` or ECS 3.8.0 top-level `name`、`namespace`、`total_size`、`total_size_unit`、`total_objects`、`sample_time` | `ecs_bucket_used_bytes`、`ecs_bucket_objects`；billing sample time 與 bucket last-modified 分開保存；billing `KB` 乘 1024 | 500/code999 does not fallback；duplicate/unrequested/malformed batch items fail the refresh；missing requested items fall back individually；雙 envelope、部分或失敗 enrichment 不得覆蓋完整 snapshot |
| `MAP-VDC-PERFORMANCE-001` | Flux `monitoring_vdc` | `cq_performance_latency` p50/p99；`cq_performance_throughput` read/write；`cq_performance_transaction*`；`cq_performance_error*` | 僅 VDC/namespace scope；rate measurement 是 Gauge，`*_delta` 是 interval delta，不是 Counter | 不得重複展開成 Bucket scope；Profile 禁止 interval rate 時不執行這些 queries |
| `MAP-NAMESPACE-PERFORMANCE-001` | Flux `monitoring_vdc` | Namespace-tagged throughput/latency 與 `cq_performance_transaction_ns*`、`cq_performance_error_ns*` | `ecs_namespace_*` throughput/latency/request Gauge；保留 VDC、Namespace、operation、quantile/status class | 不得映射為 Bucket metric |
| `MAP-BUCKET-PERFORMANCE-001` | No documented Management/Flux mapping in current evidence | none | `unavailable` | 不輸出 bucket GET/PUT/HEAD、HTTP class、latency 或 throughput |
| `MAP-REPLICATION-001` | `GET /dashboard/replicationgroups/{id}?dataType=current` and `GET /dashboard/rglinks/{id}?dataType=current` | names/ids、`rglinkStatus`、pending byte fields、`replicationRpoTimestamp` | `ecs_replication_status` from documented status enum；lag = max(now - RPO timestamp, 0) only when timestamp exists | RPO absent when no pending chunks is not an error；clock skew/negative result fails derived value |
| `MAP-RECOVERY-001` | RG link fields and Flux `monitoring_vdc.cq_recover_status_summary` | `FailoverState/ProgressPercent`、`BootstrapState/ProgressPercent` or `data_recovered/data_to_recover` | recovery ratio only when operation kind is explicit；percent / 100 or recovered / total；metric 保留 source/target VDC 以區分同一 group 的多個 link | 不得把 bootstrap、failover、hardware recovery 合併成沒有 kind 的單一狀態 |

## ECS 3.8.0.3 Batch Billing Request Observation

The exact ECS CE build produced different results for different JSON bodies on the same batch
logical API:

| Request body | Live response |
|---|---|
| No JSON entity / nil body | HTTP 415 |
| `{}` | HTTP 200 with `bucket_billing_infos: []` |
| `[]` | HTTP 400 with ECS code 1013 |
| `null` | HTTP 500 with ECS code 999 |
| `{"id":[three redacted Bucket names]}` | HTTP 200 with three `bucket_billing_infos` items；sizes `0`/`2929.6875`/`6835.9375 KB` and object counts `0`/`1`/`3` |

The ECS CE `BucketListParam.class` and non-empty live probe confirm that `id` is the Bucket-name
array. HTTP 500/code999 from `null` is a request-body error, not an unsupported-endpoint signal,
and must not trigger fallback. Only HTTP 404/405/501 or a missing requested item authorizes the
single-bucket fallback.

## Flux Response Contract

JSON response uses a columnar envelope:

```json
{
  "Series": [
    {
      "Datatypes": ["long", "dateTime:RFC3339", "long", "string"],
      "Columns": ["table", "_time", "_value", "_field"],
      "Values": [["0", "2026-01-01T00:00:00Z", "42", "bytes_recv"]]
    }
  ]
}
```

Parser rules:

1. Build a column-name to index map per Series; never assume a fixed order.
2. `Values` length must match `Columns`; malformed rows fail that query.
3. Numeric values can be JSON numbers or strings and can use decimal/exponential notation.
4. `_time` must be RFC3339/RFC3339Nano and within the requested half-open window:
   `_start <= _time < _stop`.
5. Unknown columns may be ignored; missing required tag/field makes the series unusable.
6. Use `last()` for snapshot collectors. Only queries whose Profile permits interval rates may
   use time windows for rate/delta derivation.

## Unit Contract

- Flux `mem`, `disk`, `net.bytes_*` values documented as bytes are used directly.
- Percent fields are divided by 100 and range-checked to `[0,1]`.
- Dashboard replication pending size fields documented as Bytes are used directly.
- Dashboard traffic documented as MB/s is converted to bytes/second using a documented
  adapter constant and must be verified in sandbox.
- Billing `KB` uses 1024 bytes. ECS CE 3.8.0.3 returned `9765.625 KB` for a known
  10,000,000-byte object total, which resolves exactly only with this multiplier.
- Billing `MB/GB/TB` and Management capacity/quota fields named `GB` retain their existing
  decimal candidate multipliers. The KB observation must not be generalized to those units
  without a separate known-size assertion; a mismatch is a mapping change, not a parser
  heuristic.

## Security Boundary

`GET /object/vdcs/vdc/local` and VDC list responses can contain `secretKeys`. The adapter must
drop that field during decoding and must never preserve it in the internal model, cache,
fixture, log, error or evidence.
