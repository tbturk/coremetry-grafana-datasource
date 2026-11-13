# Phase 3 Notes — Metrics and PromQL

## Current plugin boundary

The plugin does not execute metrics queries. Its frontend
[DataSource](../../src/datasource.ts) drops all queries through
`filterQuery(false)`, and backend
[QueryData](../../pkg/plugin/datasource.go) returns an explicit Phase 2 error.
Phase 3 must implement actual queries and tests; do not treat the health check
as evidence that metric queries work.

## Verified Coremetry metrics APIs

All paths below are Coremetry-specific and use the server's `/api/*`
authentication middleware. Read-only metric endpoints are viewer-accessible
unless a route has an explicit stronger role gate. They are unversioned.

### Metric catalogue and direct metric query

| Endpoint                     | Contract and source                                                                                                                                                                                                                                                                                                                                                                 |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /api/metrics/names`     | Metric catalogue. Optional `service`, server-side substring `q`, `limit`, `offset`, `metricsrc=vm                                                                                                                                                                                                                                                                                   | ch`, and other supported scope options. The legacy unpaged response is `MetricInfo[]`; paged/search response is `{names,total,hasMore}`. `limit` is capped at 1000. See [`metrics_handlers.go`](../../../coremetry/internal/api/metrics_handlers.go), `getMetricNames`. |
| `GET /api/metrics/query`     | Named metric series. Query parameters include required `name`, optional `service`, `agg`, `groupBy`, `filters`, `instance`, `engine`, `from`, `to`, `step`, `maxDataPoints`, and metric-source/scope options. Response includes `{series,rowsCapped,meta}` and an optional `note`. See [`metrics_handlers.go`](../../../coremetry/internal/api/metrics_handlers.go), `queryMetric`. |
| `GET /api/metrics`           | Direct point lookup with `name`, `service`, `from`, `to`, `limit`; returns `{points,truncated}`. This is not a general Prometheus query endpoint. See [`metrics_handlers.go`](../../../coremetry/internal/api/metrics_handlers.go), `getMetrics`.                                                                                                                                   |
| `GET /api/metrics/labels`    | Metric label values. Uses `metric`, `key`, optional `since`, `q`, `limit`, and supported metric-source/cluster scope. `since` defaults to 24h and is capped at 7d. See [`metrics_handlers.go`](../../../coremetry/internal/api/metrics_handlers.go), `getMetricLabelValues`.                                                                                                        |
| `GET /api/metrics/attr-keys` | Attribute/label key discovery for optional `metric`, `service`, `since`. The window defaults to 24h and is capped at 7d. See [`metrics_handlers.go`](../../../coremetry/internal/api/metrics_handlers.go), `getMetricAttrKeys`.                                                                                                                                                     |
| `GET /api/metrics/histogram` | Explicit-histogram query; shares the metric query time/window and source model. See [`metrics_routes.go`](../../../coremetry/internal/api/metrics_routes.go) and `getMetricHistogram` in [`metrics_handlers.go`](../../../coremetry/internal/api/metrics_handlers.go).                                                                                                              |
| `GET /api/metrics/resolve`   | Resolves a Coremetry metric descriptor passed as `m=<base64url(JSON)>`; returns series and optional exemplars. Not a generic Prometheus API. See [`metrics_routes.go`](../../../coremetry/internal/api/metrics_routes.go).                                                                                                                                                          |

The series wire type is verified in
[`spanmetric.go`](../../../coremetry/internal/chstore/spanmetric.go):

```json
{
  "groupKey": ["<group value>", "..."],
  "points": [{ "time": 0, "value": 0.0 }]
}
```

`time` is Unix nanoseconds at the bucket start. This is a schema illustration,
not a captured live result.

### Coremetry PromQL series endpoint

`GET /api/metrics/promql` evaluates a range query against the configured
metrics source. Important parameters include:

- Required `query`; maximum query length is 8192 bytes.
- `from`, `to`, `step`, and `maxDataPoints`; `maxDataPoints` is clamped to 4000. Missing `from`/`to` default to a one-hour window ending now.
- `metricsrc=vm|ch` can select the source for the request where supported.
- `instant=1` is available only with `metricsrc=vm`.
- `meta=1` opts into a `{series,meta}` envelope; without it the response
  remains `SpanMetricSeries[]`.
- Top-N and cluster-scope options are also implemented; read the current
  handler/capabilities rather than hard-coding old limits.

Source: [`metrics_routes.go`](../../../coremetry/internal/api/metrics_routes.go)
and [`promql.go`](../../../coremetry/internal/api/promql.go),
`queryPromQL`.

The language differs by source:

- ClickHouse uses Coremetry's `internal/promql` evaluator, a deliberately
  bounded PromQL subset. Unsupported expressions are client errors.
- VictoriaMetrics accepts MetricsQL (a PromQL superset) and receives the
  query text as entered.
- `GET /api/metrics/capabilities` reports source, language, supported
  features, and limits. It should inform editor capabilities and UX.
- `GET /api/metrics/discover/labels`,
  `/api/metrics/discover/label-values`,
  `/api/metrics/discover/cardinality`, and
  `/api/metrics/discover/series-sample` are server-side discovery endpoints.
  These are VictoriaMetrics-only; ClickHouse responds with an explicit
  `supported:false` reason. `top-queries` is limited to admin/editor because
  it exposes other operators' query text.

Sources: [`metrics_capabilities.go`](../../../coremetry/internal/api/metrics_capabilities.go)
and [`metrics_discovery.go`](../../../coremetry/internal/api/metrics_discovery.go).

### Separate Thanos-backed PromQL console

There is a second family at `/api/promql/*`:

- `GET|POST /api/promql/query` (`query`, optional `time`, required `cluster`)
- `GET|POST /api/promql/query_range` (`query`, required `start`, `end`,
  `cluster`; optional `step`, `points`)
- `GET /api/promql/labels`, `/api/promql/label/{name}/values`, and
  `/api/promql/series`

Query and range calls accept GET parameters or
`application/x-www-form-urlencoded` POST bodies. The query endpoints require
admin/editor roles, require a configured `cluster`, use console budgets and
rate limits, and return a Prometheus-style envelope with `status`, `data`,
warnings/infos, and Coremetry `meta`. This is a distinct Thanos console path,
not a drop-in Prometheus datasource API.

Sources: [`promql_console_routes.go`](../../../coremetry/internal/api/promql_console_routes.go)
and [`promql_console_handlers.go`](../../../coremetry/internal/api/promql_console_handlers.go).

## Prometheus HTTP API compatibility finding

Phase 1 source inspection found no registered standard endpoints:

```text
/api/v1/query
/api/v1/query_range
/api/v1/labels
/api/v1/label/{name}/values
/api/v1/series
```

Do not point Grafana's built-in Prometheus datasource at Coremetry and assume
these routes exist. PromQL support in `/api/metrics/promql`, or
Prometheus-shaped response fields in the Thanos console, does not establish
HTTP API compatibility.

## Suggested query implementation

1. Define a typed query model with explicit target kind (Coremetry metric
   descriptor vs PromQL-like expression), query text/descriptor, aggregation
   and groupings where relevant, and stable defaults. Decide whether one
   editor can represent both APIs before adding UI modes.
2. Use Grafana's request time range and `maxDataPoints` to construct bounded
   `from`, `to`, `step`, and `maxDataPoints`; send them from the Go backend.
   Never send the secure token from the frontend query model.
3. For Coremetry-specific metrics, call `/api/metrics/query`; for expression
   queries, call `/api/metrics/promql`. Use `/api/metrics/capabilities` and
   discovery routes for feature-aware editor/variable behavior.
4. Convert verified `groupKey` / nanosecond `points` into Grafana data frames
   in the backend. Preserve `rowsCapped`, `meta`, warnings, and API errors
   rather than turning partial results into success-shaped complete data.
5. Propagate context cancellation, use bounded HTTP timeouts/body sizes, and
   test query limits and Grafana frame shape. Keep query tests against a
   mock HTTP server plus focused frame-mapping tests.

This is a recommendation, not implemented behavior.

## Risks and open questions

- Which API is the primary Grafana target: `/api/metrics/query`,
  `/api/metrics/promql`, or the Thanos console? They differ in language,
  authorization, source selection, and response contract.
- What query language does the plugin promise when ClickHouse and
  VictoriaMetrics are configured? A single “PromQL” label would be
  misleading unless capabilities and subset differences are visible.
- Should the datasource target only metric points, only spans-derived
  measurements, or both? Do not assume one route covers all OTel metrics.
- Which backend/source is authoritative when `metricsrc` is omitted, and how
  should the plugin expose cluster scope?
- Grafana variables need bounded server-side search; do not fetch a full
  catalogue or validate a selected value against a sampled subset.
- Coremetry API routes are currently unversioned and error envelopes vary.
  The plugin must preserve status/error details and tolerate optional fields.
- Avoid raw-span aggregation. Coremetry has ClickHouse materialized views and
  bounded query paths; use the existing API rather than querying storage
  directly.

For the full Phase 1 analysis, see [API_INVENTORY.md](../../../coremetry/docs/grafana-integration/API_INVENTORY.md)
and [API_GAPS.md](../../../coremetry/docs/grafana-integration/API_GAPS.md).
