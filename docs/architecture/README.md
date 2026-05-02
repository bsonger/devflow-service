# DevFlow Architecture

## Purpose

This directory contains architecture diagrams for the DevFlow platform backend (`devflow-service`).
Each diagram focuses on one core question, keeping nodes and connections minimal so that new engineers can understand the system quickly.

## How to read

- Open `index.html` in a browser to see all diagrams rendered in one page.
- Or read the individual `.mmd` files (Mermaid format) in any Mermaid-compatible viewer.
- Start with **Overall Architecture**, then follow **Release Flow** and **CI Build Flow** for the core pipeline. Read **Service Boundaries** last to understand ownership.

## Diagram index

| # | Diagram | File | What it answers |
|---|---|---|---|
| 1 | Overall Architecture | [overall-architecture.mmd](overall-architecture.mmd) | How do users, frontend, gateway, services, database, Tekton, Registry, Kubernetes, and observability connect? |
| 2 | Release Flow | [release-flow.mmd](release-flow.mmd) | How does an Application become a Release, get deployed, observed, and verified? |
| 3 | CI Build Flow | [ci-build-flow.mmd](ci-build-flow.mmd) | How does a git push become a built image and a deployable Manifest? |
| 4 | Service Boundaries | [service-boundaries.mmd](service-boundaries.mmd) | Which service owns which resources, and how do they depend on each other? |

## System overview in 60 seconds

**Five backend services**, one PostgreSQL database, deployed on Kubernetes behind Istio:

| Service | Port | Owns | Storage |
|---|---|---|---|
| `meta-service` | 8081 | Project, Application, Environment, Cluster, ApplicationEnv | PostgreSQL |
| `config-service` | 8082 | AppConfig, WorkloadConfig | PostgreSQL |
| `network-service` | 8086 | Service (network), Route | PostgreSQL |
| `release-service` | 8083 | Manifest, Release, Bundle, Intent | PostgreSQL |
| `runtime-service` | 8084 | RuntimeSpec, ObservedWorkload, ObservedPod, Operator Actions | In-Memory (K8s-backed) |

## Core concepts

### Manifest — immutable build-side snapshot

A `Manifest` freezes everything needed for a build at creation time:

- `git_revision` / `commit_hash` — the exact code that was built
- `image_ref` / `image_digest` — the resulting container image
- `services_snapshot` — network service topology at build time
- `workload_config_snapshot` — workload spec at build time

Once created, a Manifest is never mutated — build progress is written back via Tekton callbacks, but the frozen inputs stay fixed.

### Release — immutable deploy-side snapshot

A `Release` freezes everything needed for deployment:

- `environment_id` — target environment
- `strategy` — rolling / blue-green / canary
- `app_config_snapshot` — env-specific configuration files
- `routes_snapshot` — routing rules at deploy time
- `steps` — ordered execution plan (freeze → render → publish → deploy → observe → finalize)

### Environment differences via Overlay / EnvConfig

Environment-specific values (ConfigMap data, replica counts, resource limits) are managed through:

- **AppConfig** — per-environment configuration files synced from a Git config-repo, resolved by `config-service`
- **WorkloadConfig** — per-application workload spec (replicas, resources, probes)
- **Env Overlay** — at bundle render time, `release-service` injects environment-specific values from AppConfig into the rendered K8s manifests

No hardcoded per-environment branches in application code.

### Release execution strategies

| Strategy | Steps (after common) | Use case |
|---|---|---|
| Rolling | `start_deployment` → `observe_rollout` | Standard gradual update |
| Blue-Green | `deploy_preview` → `observe_preview` → `switch_traffic` → `verify_active` | Zero-downtime with instant rollback |
| Canary | `deploy_canary` → `canary_10` → `canary_30` → `canary_60` → `canary_100` | Progressive traffic shifting |

All strategies share: `freeze_inputs` → `ensure_namespace` → `ensure_pull_secret` → `ensure_appproject_destination` → `render_deployment_bundle` → `publish_bundle` → `create_argocd_application` → `finalize_release`

## Key external systems

| System | Purpose | Used by |
|---|---|---|
| **PostgreSQL 18** (CloudNativePG) | Persistent storage for all services except runtime | meta, config, network, release |
| **Tekton** | CI pipeline — builds container images from git revisions | release-service (dispatch), runtime-service (observe) |
| **OCI Registry** (zot) | Stores container images and release deployment bundles | release-service, Tekton |
| **Argo CD** | GitOps deployment — syncs OCI artifacts into Kubernetes | release-service (creates Application) |
| **Kubernetes** | Workload runtime, observed by runtime-service | runtime-service, release-service |
| **Istio** | API gateway — routes external traffic to backend services | All services |
| **OpenTelemetry** | Distributed tracing + metrics export | All services |
| **Prometheus** | Metrics collection | All services |
| **Pyroscope** | Continuous profiling | All services |

## Assumptions

1. **Pre-production environment only** — the committed Istio and K8s manifests target a single pre-production cluster (`devflow-pre-production.bei.com`). Production topology may differ.
2. **telemetry-service is planned, not implemented** — it appears in diagrams only as a future concern.
3. **Single PostgreSQL cluster** — all stateful services share one CloudNativePG cluster. Service-per-database isolation is not yet implemented.
4. **runtime-service is PostgreSQL-free by design** — it uses in-memory state rebuilt from Kubernetes observation. After restart, state is temporarily empty until the observer syncs.
5. **OCI registry is in-cluster zot** — the pre-production registry runs inside the same Kubernetes cluster. Production may use an external registry.
6. **Release rollout observation currently watches Deployments only** — Rollout object support (Argo Rollouts) is planned but not yet implemented in the runtime observer.
7. **Service-to-service HTTP calls** use the shared `downstreamhttp` client with OTel trace propagation and JSON envelope parsing.
8. **ArgoCD Application source points at OCI artifact** — not at a git repository. The release bundle published to OCI is the deployment source of truth.

## Related docs

- `docs/system/architecture.md` — detailed system architecture
- `docs/system/flow-overview.md` — end-to-end release lifecycle stage contract
- `docs/system/release-steps.md` — release step semantics
- `docs/system/ingress-routing.md` — Istio gateway routing reference
- `docs/services/*.md` — per-service documentation
- `docs/resources/*.md` — per-resource API contracts
