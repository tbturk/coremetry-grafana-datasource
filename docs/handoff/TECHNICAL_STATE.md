# Technical State

## Versions and compatibility

| Component                 | Current value                                                                                   | Evidence / qualification                                                                                                |
| ------------------------- | ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Grafana server            | `grafana/grafana:13.3.1`                                                                        | Current local container; `/api/health` reported `13.3.1`.                                                               |
| Plugin Grafana dependency | `>=13.2.0`                                                                                      | [src/plugin.json](../../src/plugin.json). Only Grafana 13.3.1 is known to have been exercised locally.                  |
| Frontend Grafana packages | `@grafana/data`, `@grafana/i18n`, `@grafana/runtime`, `@grafana/schema`, `@grafana/ui` `13.2.4` | Declared in [package.json](../../package.json) and installed versions were inspected.                                   |
| React / React DOM         | `19.3.1`                                                                                        | Installed lockfile versions; manifest allows `^19.0.0`.                                                                 |
| TypeScript                | `5.9.2`                                                                                         | Exact manifest version.                                                                                                 |
| Jest / ESLint / Prettier  | `30.5.2` / `9.39.5` / `3.10.1`                                                                  | Installed versions.                                                                                                     |
| Playwright                | `@playwright/test` `1.65.0`                                                                     | Installed version; E2E tests are present but were not run locally.                                                      |
| Go                        | `1.26.3`                                                                                        | `go.mod` and the `golang:1.26.3-bookworm` backend helper image.                                                         |
| Grafana Go backend SDK    | `github.com/grafana/grafana-plugin-sdk-go v0.285.0`                                             | [go.mod](../../go.mod).                                                                                                 |
| Node                      | `.nvmrc` is `24`; helper image is `24.5.0`                                                      | `package.json` engine minimum is `>=24.5`; helper image is pinned to `node:24.5.0-bookworm-slim`.                       |
| npm                       | Package declares `npm@11.19.1`; helper image currently bundles `11.5.1`                         | Version metadata and helper runtime do not match. Confirm whether to pin the helper's npm before changing dependencies. |

## Datasource architecture

The plugin is hybrid:

- TypeScript/React config and query editors are registered in
  [src/module.ts](../../src/module.ts).
- The frontend datasource extends Grafana's `DataSourceWithBackend`.
- `filterQuery` currently returns `false` for every query, so ordinary
  frontend query execution is intentionally suppressed.
- Go's `datasource.Manage("coremetry-datasource", ...)` starts the backend.
- `QueryData` currently returns a per-query `StatusBadRequest` stating that
  metric execution is unavailable in Phase 2. It returns no sample frames.
- `plugin.json` declares a datasource with `backend: true`; no metrics, logs,
  or traces features have been implemented just by declaring this plugin type.

## Configuration and health check

`ConfigEditor` stores the Coremetry base URL in datasource `url` and the
viewer token in Grafana's `secureJsonData.apiToken` using `SecretInput`.
`LoadPluginSettings` reads the token only from
`DataSourceInstanceSettings.DecryptedSecureJSONData`. Both settings are tagged
`json:"-"`; they are not serialized into the plugin settings model.

The backend health check:

1. Rejects missing settings, malformed datasource JSON, a missing token, and
   invalid base URLs. The URL must be absolute HTTP(S), have a host, and must
   not contain user info, query, or fragment.
2. Appends `/api/services?limit=1` to the configured base URL and sends
   `Authorization: Bearer <token>` plus `Accept: application/json`.
3. Uses a 5-second HTTP timeout, refuses redirects, and reads at most 1 MiB.
4. Requires a 2xx response with a JSON `services` array. It does not validate
   each service row against the entire Coremetry service schema.
5. Returns a Grafana health error with distinct messages for 401, 403,
   malformed response/URL, missing token, and reachability failures. Error
   messages do not echo the token or upstream response body.

The successful path is covered by an `httptest` Go unit test; live
authenticated connectivity to Coremetry remains unverified because no viewer
token was supplied. The health check deliberately uses an authenticated
viewer-protected API endpoint, not unauthenticated `GET /api/health`.

## Local Grafana and provisioning

[docker-compose.yaml](../../docker-compose.yaml):

- Uses `grafana/grafana:13.3.1` by default.
- Binds container port 3000 only to `127.0.0.1:3001`.
- Requires locally configured admin user/password variables; no fixed
  production password is supplied.
- Disables anonymous access and sign-up.
- Enables unsigned loading only for `coremetry-datasource`, in this
  development Compose file.
- Mounts `dist/` read-only as the plugin directory and persists Grafana data
  in the `grafana-data` volume.
- Joins the existing external `coremetry_default` network. From Grafana,
  Coremetry is `http://coremetry:8088`; host/container `localhost` is not the
  Coremetry service.

[provisioning/datasources/coremetry.yaml](../../provisioning/datasources/coremetry.yaml)
provisions datasource UID `coremetry`, editable, with URL
`http://coremetry:8088`; it binds the secure token from the
`COREMETRY_API_TOKEN` environment variable. The checked-in example contains
blank credentials; never use or document values from local `.env`.

Dashboard provider
[provider.yaml](../../provisioning/dashboards/provider.yaml) loads
[coremetry-development.json](../../provisioning/dashboards/files/coremetry-development.json),
UID `coremetry-dev`. It contains a single text panel explaining that metric
queries are not implemented. It is not a functional metrics dashboard.

The running local Grafana was verified healthy at `127.0.0.1:3001`, and the
plugin module asset returned HTTP 200. Phase 2 had also verified datasource
and dashboard provisioning. Do not infer successful live Save & Test from
those checks.

## Build, test, and CI

The main commands are documented in [README.md](../../README.md):

```bash
./scripts/dev.sh install
./scripts/dev.sh metadata
./scripts/dev.sh typecheck
./scripts/dev.sh lint
./scripts/dev.sh format:check
./scripts/dev.sh test
./scripts/dev.sh build
./scripts/backend.sh test
./scripts/backend.sh build
./scripts/check.sh
```

`scripts/dev.sh` uses the pinned Node container; `scripts/backend.sh` uses the
pinned Go container. `scripts/check.sh` runs metadata validation, frontend
checks/build, backend tests, and Mage build.

Current test coverage includes:

- Frontend Jest tests for URL editing and secure-token input behavior.
- Backend `httptest` cases for health success, auth failures, missing token,
  malformed response/URL, timeout, redirect rejection, and the explicit
  Phase 2 query error.
- A settings test that verifies secure token data is not JSON-serialized.
- Playwright E2E tests for datasource editor rendering, missing-token Save &
  Test, and the query editor's Phase 2 notice.

The Playwright tests are configured in CI but were not run locally. There are
no tests for real Coremetry auth, metrics endpoint behavior, Grafana data-frame
mapping, Explore variables, logs, traces, or alerts.

## Security and operational caveats

- The API token is a datasource-wide credential, not a per-user credential.
  Coremetry's service tokens carry roles; they are not route-scoped.
- Use a viewer token for read-only datasource access. Do not put credentials
  into query JSON, logs, documentation, or frontend code.
- The health URL permits either HTTP or HTTPS; production TLS and network
  policy remain deployment decisions.
- Unsigned loading and loopback port binding are development-only safeguards,
  not production distribution policy.
- npm installation previously reported 29 advisories: 4 low and 25 moderate.
  No dependency remediation was included in Phase 2.
