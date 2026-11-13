# Phase 4 Notes — Services, RED, Traces, and Logs

## Service and operation discovery

| Endpoint                   | Verified contract                                                                                                                                                                                                                                                                                                             |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /api/services`        | Paginated service summaries. `limit` defaults to 50 and is capped at 500; `offset` is non-negative. Supports server-side name search and filters/sort. Response includes `services`, `hasMore`, `offset`, `limit`, and total when requested/available. See [`api.go`](../../../coremetry/internal/api/api.go), `getServices`. |
| `GET /api/service-names`   | Searchable names for variables/pickers. Parameters `q`, `limit` (default 200, max 1000), `offset`; response `{names,total,hasMore}`. See `getServiceNames` in [`api.go`](../../../coremetry/internal/api/api.go).                                                                                                             |
| `GET /api/operation-names` | Operation picker. Parameters include `service`, `q`, `limit` (default 200, max 1000), `offset`; response `{names,total,hasMore}`. See `getOperationNames` in [`api.go`](../../../coremetry/internal/api/api.go).                                                                                                              |
| `GET /api/operations`      | Operation summaries for a service and optional `since`, `from`, `to`; sourced through the store. See `getOperations` in [`api.go`](../../../coremetry/internal/api/api.go).                                                                                                                                                   |

`/api/services` is not a substitute for `/api/service-names`: the former is a
bounded summary page, while the latter is a server-side-search catalogue.

## Service-level RED

`GET /api/services/{name}/metric-red` reads the service's OTel metric series.
The handler uses `from`/`to` (default one-hour window),
`maxDataPoints` (capped at 4000), `rateWindow` (default 180 seconds, capped at
600), optional `env`, and `family=grpc` for gRPC metrics. The metrics source
can be selected using the metric-source mechanism.

Response envelope fields include `service`, `source`, `metric` identity
metadata, `metricExists`, `stepSeconds`, `note`, `series`, plus optional
status/unit/environment/partial metadata. `series` is a map whose series
arrays use the same `groupKey`/`points` representation as metric queries.
Common keys are:

- `rate` — requests per second.
- `error_rate` — percentage, not a fraction. If error status labels cannot
  be identified, the response exposes `errorsUnknown`; it must not be
  displayed as zero error rate.
- `p50`, `p95`, `p99`, `avg` — latency series, normally milliseconds when
  the source unit can be identified. Unknown units are reported; do not infer
  a unit.

Source: [`service_metric_red.go`](../../../coremetry/internal/api/service_metric_red.go),
`serviceMetricREDResponse` and `getServiceMetricRED`. The endpoint is
viewer-readable through the global API auth middleware.

Related `GET /api/services/{name}/metric-throughput` is registered in
[`api.go`](../../../coremetry/internal/api/api.go); use the RED endpoint when
the full RED series contract is needed.

## Trace search and retrieval

Coremetry's native trace routes are not a complete Tempo API:

- `GET /api/traces` searches a bounded time range (default one hour) and
  accepts service/search/trace ID/error/root and pagination/sort filters.
  The main response is `{traces,hasMore}` with total/diagnostic fields when
  applicable. See `parseTraceFilter` and `getTraces` in
  [`api.go`](../../../coremetry/internal/api/api.go).
- `GET /api/traces/count` and `/api/traces/aggregate` provide bounded count
  and aggregate views.
- `GET /api/traces/{id}` returns `{traceId,spans,source,analysis}` when spans
  are found. Depending on source/retention, it may also expose
  `spanCapped`/`spanTotal`, `replicaMiss`, or an aggregate `stub` with
  `stubReason`. It can resolve spans from Tempo or ClickHouse; it is not
  always a raw-span response.
- `GET /api/traces/{id}/bundle` combines trace detail with logs and optional
  Oracle error records. `logLimit`, `oracleLimit`, and optional time `hint`
  are bounded by the handler. Its envelope contains `logs`, `oracle`,
  `truncated`, `logLimit`, and `window` when spans provide a window; degraded
  backend states are explicit.
- `GET /api/traces/{id}/link-identity` accepts optional selected `span` and
  comma-separated `keys` (at most five safe attribute keys). Its response
  includes `requestId`, `attrs`, `candidates`, `identities`, `partial`,
  and timezone/selection metadata. The log lookup is intentionally bounded.

Sources: [`trace_routes.go`](../../../coremetry/internal/api/trace_routes.go),
[`trace_bundle.go`](../../../coremetry/internal/api/trace_bundle.go), and
[`trace_link_identity.go`](../../../coremetry/internal/api/trace_link_identity.go).

## Tempo compatibility

Coremetry registers a limited Tempo-compatible route family:

```text
GET /tempo/api/echo
GET /tempo/ready
GET /tempo/api/search
GET /tempo/api/search/tags
GET /tempo/api/v2/search/tags
GET /tempo/api/search/tag/{name}/values
GET /tempo/api/v2/search/tag/{name}/values
GET /tempo/api/traces/{id}
GET /tempo/api/v2/traces/{id}
```

`/tempo/api/search` accepts `q`, legacy `tags`, `minDuration`,
`maxDuration`, `limit`, `start`, and `end`. TraceQL handling only lifts a
simple equality subset; unsupported grammar is not full Tempo TraceQL.
Search returns `{traces:[{traceID,rootServiceName,rootTraceName,
startTimeUnixNano,durationMs}]}`. Tag routes return tag names/values. Trace
retrieval returns OTLP protobuf by default and JSON only when explicitly
requested; v2 wraps the trace in Tempo's `TraceByIDResponse`.

Source: [`tempo.go`](../../../coremetry/internal/api/tempo.go).

**Authentication warning:** Coremetry's global auth middleware protects
`/api/*`, but [`SkipPath`](../../../coremetry/internal/auth/auth.go) bypasses
authentication for paths outside `/api/`. These `/tempo/*` routes are
therefore currently public. Constrain network reachability and confirm whether
that policy is acceptable before exposing Tempo through a wider network.

The Coremetry repository's Grafana file provisions a Tempo datasource at
`http://coremetry:8088/tempo`, UID `coremetry-tempo`. The plugin repo does not
provision this datasource, and the Phase 2 plugin has no trace links. A future
integration must decide whether to reuse Grafana's native Tempo datasource
for traces or implement trace navigation in the Coremetry plugin.

## Logs and trace/log correlation

Registered log routes include:

```text
GET  /api/logs
POST /api/logs/search
GET  /api/logs/stream
GET  /api/logs/timeseries
GET  /api/logs/fields
GET  /api/logs/fieldstats
GET  /api/logs/field-values
GET  /api/logs/context
GET  /api/logs/templates
GET  /api/logs/patterns
```

`GET /api/logs` and `POST /api/logs/search` share the same search contract.
Useful filters include `service`, `cluster`, `env`, `search`, `from`, `to`,
`severity`, `traceId`, `spanId`, and `hasTrace`. Paging uses `limit`, optional
`paging=1`, and an opaque backend-owned `after` cursor; do not parse or
construct that cursor. POST takes a flat JSON object, has a 1 MiB body cap,
and converts supported scalar values to the GET parameter semantics.

Normal responses include `{total,logs,nextCursor}`. Optional honesty/degraded
fields include `partial`, `shardsFailed`, `totalIsLowerBound`,
`unappliedFilters`, `degraded`, and `reason`. A slow/unreachable log backend
can return HTTP 200 with an explicitly degraded empty result, so the plugin
must preserve those flags instead of interpreting every 200 as complete.

Each log record exposes JSON `traceId` and `spanId`; filters on those fields
support log-to-trace correlation. Coremetry can search for trace IDs across
its selected logs backend (ClickHouse or optional Elasticsearch). Do not
assume all attributes are available identically across backends.

Sources: [`logs_routes.go`](../../../coremetry/internal/api/logs_routes.go),
[`api_logs.go`](../../../coremetry/internal/api/api_logs.go), and
[`logstore.go`](../../../coremetry/internal/logstore/logstore.go).

## API gaps and implementation guidance

- Coremetry already has service/operation discovery, RED, native trace
  search/detail, Tempo-compatible routes, logs, and correlation endpoints.
  A plugin should consume these APIs; direct ClickHouse/Elasticsearch access
  is not part of this design.
- The Tempo compatibility layer is a subset, public by current auth policy,
  and only partially supports TraceQL. Test the exact Grafana Tempo plugin
  calls before promising full compatibility.
- Plugin Compose currently provisions only the Coremetry datasource. Decide
  whether Phase 4 provisions a Tempo datasource as a separate datasource or
  provides custom trace links; avoid duplicating a second trace stack without
  a specific UX need.
- Logs support both GET and POST because opaque cursor values can exceed URL
  limits. Keep cursors opaque and preserve backend partial/degraded flags.
- There is no uniform API version or uniform error envelope. Maintain source
  compatibility tests and never silently convert missing/degraded data into
  valid-looking complete frames.
- Coremetry API tokens are role-bearing rather than endpoint-scoped; use a
  viewer token and do not weaken Coremetry authorization to accommodate
  Grafana.

For the original endpoint inventory and gaps, see
[API_INVENTORY.md](../../../coremetry/docs/grafana-integration/API_INVENTORY.md)
and [API_GAPS.md](../../../coremetry/docs/grafana-integration/API_GAPS.md).
