# Decisions and Open Questions

## Decisions already established for this work

1. **Keep the plugin in a sibling standalone workspace.** The current plugin
   is at `/home/thinkpad/Desktop/coremetry-grafana-datasource`, not nested in
   the Coremetry repository. No remote or commit has been created.
2. **Use a hybrid plugin.** TypeScript/React provides Grafana editors; a Go
   backend owns Coremetry API requests and access to secure datasource data.
   This is the architecture Phase 2 was instructed to scaffold.
3. **Keep `coremetry-datasource` as the plugin ID.** It is already used by
   plugin metadata, backend startup, provisioning, unsigned-plugin config,
   and validation.
4. **Keep Phase 2 limited to configuration and connectivity.** The query
   editor explains that queries are unavailable; frontend filtering disables
   query execution; backend query calls return an explicit error. No fake
   frames or sample data are a fallback.
5. **Use the authenticated viewer health probe.** Save & Test calls
   `GET /api/services?limit=1` with the secure Bearer token so it verifies
   authenticated viewer access. It does not rely on public liveness.
6. **Keep secrets out of frontend query models and source control.** The API
   token is stored via Grafana secure datasource settings and read by the Go
   backend. Do not inspect/copy `.env` during handoff work.
7. **Keep development exposure narrow.** Grafana binds to loopback, unsigned
   plugin loading is limited to the dev plugin ID, and the existing
   `coremetry_default` network is reused without changing Coremetry deployment
   settings.
8. **Treat Coremetry source APIs as the contract.** PromQL implementation is
   not proof of Prometheus HTTP API compatibility. Use the verified API
   inventory and current source route registrations.

## ADR status and approval distinction

The Phase 1
[ADR-001](../../../coremetry/docs/grafana-integration/ADR-001-grafana-datasource-architecture.md)
still has status **Proposed** and names Coremetry maintainers as decision
owners. Phase 2 explicitly instructed the implementation to use that
architecture, so the hybrid design and sibling workspace are the accepted
working direction for the scaffold. Do not claim that maintainers have
confirmed the final official repository, signing policy, or release owner.

## Decisions that must not be changed silently

- Do not move the plugin into the Coremetry monorepo or create a Git remote
  without maintainer/operator direction.
- Do not change plugin ID, provisioning UID, or container network assumptions
  without updating the metadata checker, tests, and local docs together.
- Do not put API tokens in frontend JavaScript, query JSON, datasource
  `jsonData`, dashboard JSON, logs, or examples.
- Do not call `/api/promql/*` a standard Prometheus HTTP API. The built-in
  Prometheus datasource cannot be assumed to work against Coremetry.
- Do not promise full MetricsQL/PromQL parity across ClickHouse and
  VictoriaMetrics; their evaluator behavior differs.
- Do not silently ignore `partial`, `degraded`, `truncated`, or unsupported
  capability flags from Coremetry.
- Do not change Coremetry auth policy or access controls as a plugin workaround.
- Do not mark the plugin as production-ready based on the scaffold build,
  mocked health test, or running Grafana alone.

## Open questions for maintainers/operator

1. **Official repository and ownership:** Should this sibling become a new
   repository in the Coremetry maintainers' organization? Who owns its
   releases, signing, and Grafana catalog submission?
2. **ADR approval:** Should maintainers update ADR-001 from Proposed after
   confirming ownership and release policy?
3. **Grafana support matrix:** Is Grafana 13.2 the true minimum, and which
   versions must be built and E2E-tested? The current local server is 13.3.1
   while frontend SDK packages are 13.2.4.
4. **Phase 3 query contract:** Should the main target be
   `/api/metrics/query`, `/api/metrics/promql`, or the separately authorized
   `/api/promql/*` Thanos console? Which source(s) and query language should
   the plugin expose?
5. **Dashboard variables:** Which metrics/service/operation variables are
   required initially, and what bounds and empty/error UX should apply?
6. **Phase 4 trace strategy:** Reuse Grafana's native Tempo datasource for
   Coremetry's `/tempo` API, or implement trace search/detail/navigation in
   this plugin? Should plugin provisioning create that second datasource?
7. **Log UX:** Should logs be a Grafana Logs query capability, a link to
   Coremetry's existing Logs page, or a later dedicated surface? How should
   `degraded`, `partial`, and opaque cursor behavior appear?
8. **Tempo exposure:** `/tempo/*` currently bypasses global `/api/*`
   authentication. Is that acceptable on the production network, or should
   Coremetry add authentication before Grafana use outside a trusted network?
9. **Live auth validation:** When can a least-privilege viewer token be used
   to verify Save & Test against the actual local Coremetry instance?
10. **Dependency hygiene:** Should the Node helper pin npm `11.19.1` to match
    `packageManager`, and what is the plan for the 29 previously reported npm
    advisories?
11. **Repository persistence:** The Coremetry Phase 1 documents are untracked,
    and the plugin repository currently has no commit or remote. Which changes
    should be committed, and to which approved remote?

## Assumptions that still require verification

- Grafana can reach Coremetry by Docker DNS name `coremetry` on the external
  `coremetry_default` network.
- A Coremetry viewer token is sufficient for all intended read-only plugin
  routes; verify each route's role gate before building a multi-signal UI.
- The existing `/tempo` implementation covers the subset of Tempo calls the
  supported Grafana versions actually make. Route presence alone is not
  end-to-end compatibility.
- Grafana frontend packages `13.2.4` remain compatible with the supported
  Grafana server versions. Only the local `13.3.1` server is currently known
  to be running.
- API response fields and limits remain unversioned contracts; future Coremetry
  changes may require compatibility tests or API versioning.
- The Phase 1 documents' cross-links to GitHub will become valid only after
  those currently untracked Coremetry files are committed to the linked branch.
