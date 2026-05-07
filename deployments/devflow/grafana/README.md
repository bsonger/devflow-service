# Production Grafana

Grafana dashboard-as-code assets for DevFlow production live in this
directory.

Target post-reconcile supported folder:
`devflow-production`

Target post-reconcile supported dashboards:
- `Infra / Cluster`
- `Infra / Node`
- `Service / Overview`
- `Service / Detail`
- `Service / Dependency`
- `Delivery / Release`
- `Page/API / Route Detail`

This target inventory freezes the final folder/dashboard set to reconcile
toward. The current dashboard/import details below describe the
pre-reconcile state at this commit.

Current dashboard:

- `dashboards/devflow-prod-services.json`

Import target:

- UID: `devflow-prod-services`
- Title: `DevFlow Production Services`
- Datasource variable: `${DS_PROMETHEUS}`

The dashboard scopes production with the Prometheus scrape label
`namespace=devflow`, then prefers canonical application metric
labels such as `service_name`. Compatibility queries may temporarily use
`label_replace(...)` from legacy `service` labels during rollout windows.

Do not add `trace_id`, `request_id`, `release_id`, or other high-cardinality
identifiers to dashboard metric queries. Use Prometheus exemplars to jump from
5xx metric samples to traces.
