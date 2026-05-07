# Pre-Production Grafana

Grafana dashboard-as-code assets for DevFlow pre-production live in this
directory.

Target post-reconcile supported folder:
`devflow-pre-production`

Target post-reconcile supported dashboards:
- `Infra / Cluster`
- `Infra / Node`
- `Service / Overview`
- `Service / Detail`
- `Service / Dependency`
- `Delivery / Release`
- `Page/API / Route Detail`

This target inventory freezes the final post-reconcile folder/dashboard
set to reconcile toward. The committed dashboard files listed below
reflect the pre-reconcile inventory snapshot at this commit.

Current dashboards:

- `dashboards/devflow-preprod-services.json`
  - UID: `devflow-preprod-services`
  - Title: `Service / Overview`
- `dashboards/devflow-preprod-service-detail.json`
  - UID: `devflow-preprod-service-detail`
  - Title: `Service / Detail`
- `dashboards/devflow-preprod-release-control.json`
  - UID: `devflow-preprod-release-control`
  - Title: `Delivery / Release`
- `dashboards/devflow-preprod-dependency-detail.json`
  - UID: `devflow-preprod-dependency-detail`
  - Title: `Service / Dependency`
- `dashboards/devflow-preprod-kubernetes-workloads.json`
  - UID: `devflow-preprod-kubernetes-workloads`
  - Title: `Infra / Cluster`
- `dashboards/devflow-preprod-node-health.json`
  - UID: `devflow-preprod-node-health`
  - Title: `Infra / Node`
- `dashboards/devflow-preprod-route-detail.json`
  - UID: `devflow-preprod-route-detail`
  - Title: `Page/API / Route Detail`

Folder inventory:

- Current shared pre-reconcile Grafana folder: `devflow`
- Current shared pre-reconcile folder UID: `ef9tqdkc8vtoge`
- Provenance: captured from Grafana API inventory during this cleanup pass on 2026-05-08
- Target post-reconcile Grafana folder: `devflow-pre-production`

Dashboard intent:

- Overview: service health, request rate, error rate, latency, hot routes, pod resource usage
- Service Detail: per-service and per-route request, status, latency, payload size, inflight, 5xx exemplar jump-off
- Release Control: release throughput, failures, rollback, duration
- Dependency Detail: downstream call rate, error ratio, latency
- Page/API / Route Detail: per-route traffic, status classes, latency, payload size, and exemplar jump-off for route-level investigation
- Kubernetes Workloads: deployment readiness, pod phase, restarts, pod CPU/memory, scrape health
- Node Health: node CPU, memory, root disk, load, DevFlow pod distribution by node

Query rules:

- scope pre-production with `namespace=devflow-pre-production`
- prefer canonical metric labels such as `service_name`, `http_route`, and `http_response_status_class`
- do not add `trace_id`, `request_id`, `release_id`, or other high-cardinality identifiers to metric label queries
- use Prometheus exemplars on 5xx panels to jump into traces
