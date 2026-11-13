# Coremetry Grafana Datasource — Technical Handoff

## Purpose

Continue the official Coremetry-to-Grafana datasource work without repeating
Phase 1 API discovery or mistaking the Phase 2 scaffold for a working query
plugin. The current deliverable is a hybrid Grafana datasource foundation;
metric, log, and trace query execution is not implemented.

## Repositories and verified state

| Repository       | Location                                              | State                                                                                                                                                                |
| ---------------- | ----------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Coremetry server | `/home/thinkpad/Desktop/coremetry`                    | `main` tracks `origin/main`; runtime source has not been changed for this integration. Phase 1 documents exist as untracked files under `docs/grafana-integration/`. |
| Grafana plugin   | `/home/thinkpad/Desktop/coremetry-grafana-datasource` | Sibling workspace, not nested in Coremetry. Local `main`, no configured remote, no commit yet; scaffold files are untracked.                                         |

The Phase 1 API inventory, architecture, gaps, roadmap, and ADR are in the
Coremetry worktree:

- [API_INVENTORY.md](../../../coremetry/docs/grafana-integration/API_INVENTORY.md)
- [ARCHITECTURE.md](../../../coremetry/docs/grafana-integration/ARCHITECTURE.md)
- [API_GAPS.md](../../../coremetry/docs/grafana-integration/API_GAPS.md)
- [IMPLEMENTATION_ROADMAP.md](../../../coremetry/docs/grafana-integration/IMPLEMENTATION_ROADMAP.md)
- [ADR-001-grafana-datasource-architecture.md](../../../coremetry/docs/grafana-integration/ADR-001-grafana-datasource-architecture.md)

**Portability warning:** those Coremetry documents are currently untracked in
the Coremetry worktree. The plugin [README.md](../../README.md) links to their
GitHub location, which will not contain these documents until they are
committed there. Cross-repository links in this handoff assume the two
workspaces remain siblings.

## Phase status

### Phase 1 — discovery and architecture

Completed as repository-grounded documentation. The key finding is that
Coremetry's PromQL evaluator does **not** make Coremetry a Prometheus HTTP API
server. In particular, the standard `/api/v1/query`, `/api/v1/query_range`,
`/api/v1/labels`, `/api/v1/label/{name}/values`, and `/api/v1/series` routes
are not registered. Coremetry instead has its own metrics API, plus a separate
Thanos-backed `/api/promql/*` console surface.

The recommended design is a standalone hybrid plugin: Grafana frontend in
TypeScript/React and a Go backend for API calls and credentials. The Phase 1
ADR still says **Proposed** with maintainer confirmation pending. Phase 2 was
explicitly instructed to use this architecture, but that does not establish
official repository ownership, signing, or release policy.

### Phase 2 — scaffold and local Grafana

The implementation is in the sibling plugin repository. The following state
was verified from the current files and local runtime:

- Plugin ID is `coremetry-datasource`, present in
  [src/plugin.json](../../src/plugin.json), the Go `datasource.Manage` call,
  provisioning, and the metadata checker.
- Grafana responds at `http://127.0.0.1:3001` with version `13.3.1`.
  `/api/health` returned healthy, and
  `/public/plugins/coremetry-datasource/module.js` returned HTTP 200.
- Datasource provisioning uses UID `coremetry`, URL
  `http://coremetry:8088`, and a Grafana secure datasource field for the
  viewer token. A minimal dashboard with UID `coremetry-dev` is provisioned.
- Save & Test is implemented in the Go backend. It makes authenticated
  `GET /api/services?limit=1`, not a call to the public liveness endpoint.
- Query execution is deliberately disabled: the frontend filters every query
  out, and a direct backend query returns an explicit Phase 2 error with no
  frames.
- The Coremetry instance was not given an API token for Phase 2 validation.
  Successful authenticated Save & Test against the live Coremetry service is
  therefore **not verified**. A Go unit test verifies the success path against
  a mocked HTTP server.
- Frontend/backend builds and unit tests passed in Phase 2. Current Playwright
  tests exist and are wired into CI, but were not run locally.
- Dependency installation reported 29 npm advisories (4 low, 25 moderate);
  they were not addressed in Phase 2.

Do not open, print, copy, or commit the plugin's local `.env`. The checked-in
[.env.example](../../.env.example) contains only blank local credential
settings.

## Current implementation boundary

### Implemented

- Datasource configuration editor for Coremetry base URL and secure viewer
  token.
- Go backend startup, datasource settings loading, authenticated health check,
  and explicit query-unavailable response.
- Local Grafana Compose environment, provisioning, persistent Grafana data,
  and development-only unsigned plugin loading.
- Plugin metadata, build/test scripts, frontend unit tests, backend tests, and
  CI/E2E test definitions.

### Not implemented

- Metrics or PromQL query execution, Grafana data-frame conversion, metric
  variables, or Explore metrics.
- Service/operation discovery in the plugin.
- Logs, trace search/retrieval, trace-to-logs, or trace links in the plugin.
- Grafana alerting support.
- Live authenticated Save & Test using a real Coremetry viewer token.
- A production plugin remote, signing/distribution setup, or maintainer-owned
  release process.

The Coremetry server's own
[Grafana Tempo provisioning](../../../coremetry/grafana/provisioning/datasources/coremetry.yaml)
is separate. The plugin Compose provisioning currently creates only the
Coremetry datasource; it does not provision a Tempo datasource.

## Local access and commands

- Grafana: `http://127.0.0.1:3001`; sign in with credentials configured locally
  in `.env`. No password is recorded in this handoff.
- Coremetry URL from Grafana's container: `http://coremetry:8088`.
- Grafana uses the existing external Docker network `coremetry_default`.
  Start Coremetry first; do not replace this with container-local `localhost`.

Run commands from the plugin repository root:

```bash
cp .env.example .env
# Set a unique local Grafana password in .env; a viewer token is optional.
./scripts/dev.sh install
./scripts/dev.sh build
./scripts/backend.sh test
./scripts/backend.sh build
./scripts/dev.sh metadata
docker compose up -d
```

For ongoing work, see the complete command table in
[README.md](../../README.md). Common checks are:

```bash
./scripts/dev.sh watch
./scripts/dev.sh test
./scripts/dev.sh lint
./scripts/dev.sh typecheck
./scripts/dev.sh format:check
./scripts/dev.sh metadata
./scripts/check.sh
docker compose logs -f grafana
docker compose down
```

The scripts run in Node `24.5.0` and Go `1.26.3` containers. Compose publishes
Grafana only on loopback. `docker compose down` preserves the Grafana data
volume.

## Important files

| File                                                                                                                                                                               | Responsibility                                                                           |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| [src/plugin.json](../../src/plugin.json)                                                                                                                                           | Grafana plugin ID, type, backend flag, and minimum Grafana dependency.                   |
| [src/module.ts](../../src/module.ts), [src/datasource.ts](../../src/datasource.ts)                                                                                                 | Frontend plugin registration and `DataSourceWithBackend` boundary.                       |
| [ConfigEditor.tsx](../../src/components/ConfigEditor.tsx), [QueryEditor.tsx](../../src/components/QueryEditor.tsx)                                                                 | URL/secure-token UI and the intentional Phase 2 query notice.                            |
| [pkg/main.go](../../pkg/main.go)                                                                                                                                                   | Starts the backend under the plugin ID.                                                  |
| [datasource.go](../../pkg/plugin/datasource.go)                                                                                                                                    | Health check and explicit non-implemented query response.                                |
| [settings.go](../../pkg/models/settings.go)                                                                                                                                        | Reads URL and decrypted secure token; excludes both from JSON serialization.             |
| [docker-compose.yaml](../../docker-compose.yaml)                                                                                                                                   | Local Grafana, loopback port, external Coremetry network, and dev-only unsigned loading. |
| [provisioning/datasources/coremetry.yaml](../../provisioning/datasources/coremetry.yaml)                                                                                           | Datasource UID, URL, and secure token environment binding.                               |
| [provisioning/dashboards/provider.yaml](../../provisioning/dashboards/provider.yaml), [coremetry-development.json](../../provisioning/dashboards/files/coremetry-development.json) | File-based dashboard provisioning and minimal scaffold dashboard.                        |
| [scripts/check.sh](../../scripts/check.sh), [package.json](../../package.json), [go.mod](../../go.mod)                                                                             | Validation workflow and dependency/toolchain declarations.                               |

## Recommended next steps

1. Keep Phase 3 and Phase 4 separate. Start Phase 3 with a narrow metrics-query
   design based on [PHASE3_NOTES.md](./PHASE3_NOTES.md), not a Prometheus
   datasource compatibility assumption.
2. Resolve with maintainers the production repository location, remote,
   signing/distribution owner, supported Grafana versions, and the Phase 1
   ADR's Proposed status.
3. Before claiming authenticated connectivity, provide a least-privilege
   Coremetry viewer token through the local secure configuration and test
   Save & Test without exposing the token.
4. Reconcile the package's declared npm version and the version bundled by the
   Node helper image; review the reported dependency advisories.
5. Run the existing Playwright E2E suite in the local environment or CI before
   relying on browser-level behavior.
6. Only implement later phases after the API and UX decisions in
   [DECISIONS.md](./DECISIONS.md) are settled.
