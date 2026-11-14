# Phase 3 Completion Report — Metrics Datasource MVP

## Executive Summary

Phase 3 implements functional metrics querying for the official Coremetry Grafana datasource integration using Coremetry's `GET /api/metrics/promql` endpoint. The implementation satisfies all criteria for Phase 3:
- Full backend query execution in Go (`pkg/plugin/datasource.go`, `client.go`, `dataframe.go`, `models/`).
- Frontend PromQL Code mode query editor in TypeScript/React (`src/components/QueryEditor.tsx`, `src/datasource.ts`).
- Time-series and instant query conversions to Grafana DataFrames.
- Real-world integration testing against live Grafana 13.3.1 and live Coremetry demo data.
- Development dashboard updated with 4 live panels (request rate, error rate, latency, connection usage).
- No Coremetry source code was modified.
- Full test suites pass cleanly.

---

## 1. API Strategy & Compatibility Findings

### Standard Prometheus HTTP API
Source code inspection of Coremetry (`/home/thinkpad/Desktop/coremetry`) confirmed that Coremetry **does not** implement the standard Prometheus HTTP API routes:
- `/api/v1/query` (Missing)
- `/api/v1/query_range` (Missing)
- `/api/v1/labels` (Missing)
- `/api/v1/label/{name}/values` (Missing)
- `/api/v1/series` (Missing)

Pointing Grafana's standard Prometheus datasource at Coremetry is not supported.

### Selected API Strategy
The plugin uses `GET /api/metrics/promql`:
- Accepts: `query`, `from`, `to`, `step`, `maxDataPoints`, `instant`, `meta=1`.
- Timestamps: Coremetry expects `from` and `to` as Unix nanoseconds (`int64`). Series points also return `time` in Unix nanoseconds.
- Backends:
  - **ClickHouse:** Uses Coremetry's internal PromQL engine. Returns `SpanMetricSeries[]` where each series has `groupKey: []string` and `points: []{time, value}`. The `instant=1` flag is rejected by ClickHouse; the datasource simulates instant queries by querying a 15-minute lookback window and taking the latest point for each series.
  - **VictoriaMetrics:** Accepts `instant=1` directly and supports standard MetricsQL syntax.
- Authentication: Uses `Authorization: Bearer <token>` server-side. Credentials never leak to frontend state or query parameters.

---

## 2. Implementation Overview

### Frontend
- [`src/types.ts`](../../src/types.ts): Defined `CoremetryQuery` supporting `rawQuery`, `instant`, `range`, `step`, and `DEFAULT_QUERY`.
- [`src/datasource.ts`](../../src/datasource.ts): Updated `filterQuery` to allow queries with non-empty `rawQuery`.
- [`src/components/QueryEditor.tsx`](../../src/components/QueryEditor.tsx): PromQL expression textarea, Instant/Range radio options, Min step input, and query change/run handlers.
- [`src/components/QueryEditor.test.tsx`](../../src/components/QueryEditor.test.tsx): Jest unit tests for editor rendering, mode switching, and input updates.
- [`src/plugin.json`](../../src/plugin.json): Enabled `"metrics": true`.

### Backend
- [`pkg/models/query.go`](../../pkg/models/query.go): Internal model for deserializing Grafana queries (`QueryModel`), time ranges, and duration parsing.
- [`pkg/models/series.go`](../../pkg/models/series.go): Wire types for Coremetry PromQL responses (`MetricSeries`, `MetricPoint`, `PromQLResponse`).
- [`pkg/plugin/client.go`](../../pkg/plugin/client.go): HTTP client for Coremetry API communication with request timeouts, context cancellation, step calculation, and error parsing (400, 401, 403, 502).
- [`pkg/plugin/dataframe.go`](../../pkg/plugin/dataframe.go): Converts Coremetry series into Grafana DataFrames:
  - `SeriesToRangeFrames`: Converts nanosecond timestamps and values into time-series DataFrames with metadata type `timeseries-multi`.
  - `SeriesToInstantFrames`: Maps the latest point per series to instant vector frames.
  - `extractByLabels`: Extracts label key-value pairs from PromQL `by(...)` clauses and `key=value` strings.
- [`pkg/plugin/datasource.go`](../../pkg/plugin/datasource.go): `QueryData` implementation handling concurrent query execution, context cancellation, and error isolation per `refId`.

---

## 3. Test & Verification Results

### Unit Tests
1. **Frontend Jest Tests:**
   - `src/components/ConfigEditor.test.tsx` (Passed)
   - `src/components/QueryEditor.test.tsx` (Passed)
   - 2 suites, 6 tests passing.
2. **Backend Go Tests:**
   - `pkg/models` (Passed)
   - `pkg/plugin` (Passed - client, dataframe, datasource QueryData, health check)

### Live Grafana Integration Tests
Tested against local Grafana 13.3.1 (`http://127.0.0.1:3001`) and live Coremetry (`http://coremetry:8088`):
1. **Save & Test Health Check:** HTTP 200 `OK: Connected to Coremetry; authenticated viewer API access verified.`
2. **Range Query:** `sum by(service_name) (rate(http.server.requests[5m]))` returned 6 series with labels (`service_name`) and 10–43 data points per series.
3. **Instant Query:** `sum by(service_name) (rate(http.server.requests[5m]))` with `instant: true` returned 6 series, 1 latest row per series with labels and evaluated rates.
4. **Invalid PromQL Syntax:** `sum(nonexistent_metric{invalid =))` returned HTTP 400 with plugin error: `unsupported request: promql: expected a quoted matcher value, got ")" (at position 32)`.
5. **Empty Results:** `nonexistent_metric` returned HTTP 200 with an empty time-series frame.
6. **Blank Queries:** Whitespace-only queries returned an empty frame immediately without dispatching upstream requests.
7. **Development Dashboard:** Provisioned under UID `coremetry-dev` with 4 working panels:
   - Panel 1: Request Rate (HTTP)
   - Panel 2: Error Rate (Authentication Failures)
   - Panel 3: Average Latency (HTTP Server Duration)
   - Panel 4: Database Connections Usage

---

## 4. Phase 4 Readiness

The datasource architecture is clean, modular, and ready for Phase 4:
- Log query editor and LogQL/search endpoints can be integrated into `src/components/QueryEditor.tsx` or a mode selector.
- Backend `client.go` is easily extensible to invoke `/api/logs/query`.
- Secure credential management and health check infrastructure are fully verified and operational.
