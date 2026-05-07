# Pre-Production Grafana

Grafana dashboard-as-code assets for DevFlow pre-production live in this
directory.

Supported folder: `devflow-pre-production`

Supported dashboards:
- `Infra / Cluster`
- `Infra / Node`
- `Service / Overview`
- `Service / Detail`
- `Service / Dependency`
- `Delivery / Release`
- `Page/API / Route Detail`

Current dashboard:

- `dashboards/devflow-preprod-services.json`

Import target:

- UID: `devflow-preprod-services`
- Title: `DevFlow Pre-Production Services`
- Datasource variable: `${DS_PROMETHEUS}`

The dashboard scopes pre-production with the Prometheus scrape label
`namespace=devflow-pre-production`, then prefers canonical application metric
labels such as `service_name`. Compatibility queries may temporarily use
`label_replace(...)` from legacy `service` labels during rollout windows.

Do not add `trace_id`, `request_id`, `release_id`, or other high-cardinality
identifiers to dashboard metric queries. Use Prometheus exemplars to jump from
5xx metric samples to traces.
