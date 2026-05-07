# Production Grafana

Grafana dashboard-as-code assets for DevFlow production live in this
directory.

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
