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

This target inventory freezes the final post-reconcile folder/dashboard
set to reconcile toward. The current imported dashboards in this directory
are listed below and reflect the current pre-reconcile state at this
commit.

Current dashboards:

- `dashboards/devflow-prod-services.json`
  - UID: `devflow-prod-services`
  - Title: `Service / Overview`
- `dashboards/devflow-prod-service-detail.json`
  - UID: `devflow-prod-service-detail`
  - Title: `Service / Detail`
- `dashboards/devflow-prod-release-control.json`
  - UID: `devflow-prod-release-control`
  - Title: `Delivery / Release`
- `dashboards/devflow-prod-dependency-detail.json`
  - UID: `devflow-prod-dependency-detail`
  - Title: `Service / Dependency`
- `dashboards/devflow-prod-kubernetes-workloads.json`
  - UID: `devflow-prod-kubernetes-workloads`
  - Title: `Infra / Cluster`
- `dashboards/devflow-prod-node-health.json`
  - UID: `devflow-prod-node-health`
  - Title: `Infra / Node`
- `dashboards/devflow-prod-route-detail.json`
  - UID: `devflow-prod-route-detail`
  - Title: `Page/API / Route Detail`

Folder inventory:

- Current shared pre-reconcile Grafana folder: `devflow`
- Current shared pre-reconcile folder UID: `ef9tqdkc8vtoge`
- Target post-reconcile Grafana folder: `devflow-production`

Dashboard intent:

- Overview: service health, request rate, error rate, latency, hot routes, pod resource usage
- Service Detail: per-service and per-route request, status, latency, payload size, inflight, 5xx exemplar jump-off
- Release Control: release throughput, failures, rollback, duration
- Dependency Detail: downstream call rate, error ratio, latency
- Kubernetes Workloads: deployment readiness, pod phase, restarts, pod CPU/memory, scrape health
- Node Health: node CPU, memory, root disk, load, DevFlow pod distribution by node

Query rules:

- scope production with `namespace=devflow`
- prefer canonical metric labels such as `service_name`, `http_route`, and `http_response_status_class`
- do not add `trace_id`, `request_id`, `release_id`, or other high-cardinality identifiers to metric label queries
- use Prometheus exemplars on 5xx panels to jump into traces
