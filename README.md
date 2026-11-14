# Coremetry Grafana Datasource

Official Grafana datasource plugin for [Coremetry](https://github.com/cosretr/coremetry), extending Grafana into an APM integration supporting **Service Discovery**, **RED Metrics**, **Trace Navigation**, **Log Querying**, and **Trace/Log Correlation**.

---

## Architecture & Integration Strategy

```
┌─────────────────────────────────────────────────────────────┐
│                       Grafana 13.x                          │
│                                                             │
│   Explore / Dashboards (Metrics, Services, RED, Logs)       │
│               ▲                           ▲                 │
│               │ (DataLink on traceId)     │ (tracesToLogs)  │
│               ▼                           ▼                 │
│   ┌───────────────────────┐   ┌─────────────────────────┐   │
│   │  Coremetry Datasource │   │ Coremetry Tempo (Native)│   │
│   │   (type: coremetry)   │   │     (type: tempo)       │   │
│   └───────────┬───────────┘   └────────────┬────────────┘   │
└───────────────┼────────────────────────────┼────────────────┘
                │                            │
                │ /api/*                     │ /tempo/*
                ▼                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     Coremetry Server                        │
│                                                             │
│  - PromQL Engine (/api/metrics/promql)                      │
│  - Service Directory & RED (/api/services)                  │
│  - Logs Search (/api/logs)                                  │
│  - Traces & Tempo Gateway (/api/traces, /tempo/*)           │
└─────────────────────────────────────────────────────────────┘
```

### 1. Prometheus HTTP API Compatibility Finding

**Coremetry does not implement the standard Prometheus HTTP API** (`/api/v1/query`, `/api/v1/query_range`, etc.). This plugin functions as a native backend adapter communicating directly with Coremetry's `GET /api/metrics/promql`.

### 2. Trace Integration (Native Tempo Compatibility)

Coremetry exposes an internal Tempo-compatible gateway under `/tempo/*` (`/tempo/ready`, `/tempo/api/search`, `/tempo/api/v2/traces/{id}`).

- **Native Tempo Datasource:** Configured with `type: tempo`, UID `coremetry-tempo`, URL `http://coremetry:8088/tempo`.
- **Trace/Log Correlation:**
  - **Logs to Trace:** The Coremetry datasource attaches a Grafana `DataLink` to any `traceId` field in log records, opening the trace in the Tempo viewer.
  - **Traces to Logs:** Tempo is provisioned with `tracesToLogs.datasourceUid: "coremetry"`, enabling 1-click navigation from any span directly to related Coremetry logs.

---

## Supported Query Capabilities (Phase 4 APM)

| Query Mode           | Endpoint                              | Description                                                                                               |
| -------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| **Metrics (PromQL)** | `GET /api/metrics/promql`             | Code mode PromQL expressions, instant/range queries, auto/min step.                                       |
| **Services**         | `GET /api/services`                   | Service catalogue returning table frames with health, throughput, error rate, and P99 latency.            |
| **Service RED**      | `GET /api/services/{name}/metric-red` | Service-level Rate, Error rate, and Latency (P50, P95, P99, Avg) time-series.                             |
| **Logs**             | `GET /api/logs`                       | Log stream with service, severity, text search, and trace ID filtering. Formatted as Grafana Logs frames. |
| **Traces**           | `GET /api/traces`                     | Trace search with service, traceId, and duration, including DataLinks to Tempo.                           |

---

## Dashboard Template Variables

The datasource implements `metricFindQuery` and `CallResource` for dynamic dashboard variables:

| Variable Query                                  | Output                             | Description                                                                         |
| ----------------------------------------------- | ---------------------------------- | ----------------------------------------------------------------------------------- |
| `services()` or `services`                      | `auth-service`, `api-gateway`, ... | Discovers all active services from `GET /api/service-names`.                        |
| `operations($service)` or `endpoints($service)` | `POST /login`, `GET /users`, ...   | Discovers operation names for the selected service from `GET /api/operation-names`. |

---

## Configuration & Authentication

1. In Grafana, navigate to **Connections > Data Sources > Add data source** and select **Coremetry**.
2. **URL:** Base URL of your Coremetry instance (e.g. `http://coremetry:8088`).
3. **Viewer API Token:** Enter a valid Coremetry token (encrypted server-side via `secureJsonData.apiToken`).
4. **Trace Datasource UID:** UID of the Tempo datasource connected to `/tempo` (default: `coremetry-tempo`).
5. Click **Save & Test**. The plugin verifies authenticated viewer access against `GET /api/services?limit=1`.

---

## Provisioned Dashboards

The plugin includes 4 ready-to-use Grafana dashboards:

1. **Coremetry APM Overview (`coremetry-apm-overview`):**
   - Service Directory table with health indicators, throughput, and error rates.
   - Fleet-wide HTTP request rate, authentication failures, and latency graphs.
   - Live application log stream.
2. **Coremetry Service Performance (`coremetry-service-performance`):**
   - Dynamic `$service` dropdown variable.
   - Dedicated RED metrics panels (Rate, Error Rate %, P99/Avg Latency).
   - Filtered application logs with trace correlation.
   - Recent traces table with links to Tempo trace viewer.
3. **Coremetry Logs and Traces (`coremetry-logs-traces`):**
   - Cross-signal log exploration with `$service`, `$search`, and `$trace_id` filters.
   - Traces table with 1-click links to the Tempo waterfall view.
4. **Coremetry Datasource Development (`coremetry-dev`):**
   - 4-panel PromQL development reference dashboard.

---

## Example Queries

### 1. PromQL Metrics Mode

```promql
sum by(service_name) (rate(http.server.requests[5m]))
```

### 2. Service RED Mode

- Service: `coremetry-monolithic` (or `$service`)
- RED Metric: `rate` (or `error_rate`, `p99`, `avg`, `all`)

### 3. Logs Mode

- Service: `auth-service`
- Severity: `17` (Error)
- Search: `failed`
- Trace ID: `a03eac4e490b2e8605c74c3d3b253cce`

### 4. Traces Mode

- Service: `notification-service`
- Limit: `25`

---

## Development Commands

All development tasks are executed via containerized helper scripts:

```bash
# Frontend development & checks
./scripts/dev.sh install        # Run npm ci
./scripts/dev.sh metadata       # Validate plugin.json metadata
./scripts/dev.sh typecheck      # Run tsc --noEmit
./scripts/dev.sh lint           # Run eslint
./scripts/dev.sh format:check   # Run prettier check
./scripts/dev.sh test           # Run Jest unit tests
./scripts/dev.sh build          # Build frontend assets to dist/

# Go backend development & checks
./scripts/backend.sh test       # Run Go unit tests (pkg/models, pkg/plugin)
./scripts/backend.sh build      # Run Mage cross-platform binary build
./scripts/backend.sh check      # Run Go tests and Mage build

# Full verification
./scripts/check.sh              # Runs metadata, frontend check, backend check
```

---

## Troubleshooting

| Issue                             | Likely Cause                     | Solution                                                                                 |
| --------------------------------- | -------------------------------- | ---------------------------------------------------------------------------------------- |
| **Health Check 401 Unauthorized** | Missing or invalid API token     | Verify `COREMETRY_API_TOKEN` in `.env` or re-enter token in settings.                    |
| **Health Check 403 Forbidden**    | API token lacks viewer role      | Grant `viewer` role to the token in Coremetry.                                           |
| **Trace link fails to open**      | Tempo datasource UID mismatch    | Check `traceDatasourceUid` in datasource settings matches Tempo UID (`coremetry-tempo`). |
| **Empty RED metrics**             | Service does not emit RED series | Ensure service has HTTP or gRPC duration metrics in Coremetry (`/metric-red`).           |
| **HTTP 502 Bad Gateway**          | Coremetry container unreachable  | Check if Coremetry is running on the shared Docker network (`coremetry_default`).        |

---

## Roadmap

- **Phase 5:** Production Packaging, signing, and marketplace release readiness.
