# Deployments

This directory is reserved for in-repo deployment artifacts for `devflow-service`.

Examples:
- Kubernetes manifests
- Helm charts
- environment-specific deployment overlays

Keep build-time Docker base-image assets under `docker/`.

Current local pre-production flow:
- apply `deployments/tekton/devflow-tekton-image-build-and-push.yaml` and `deployments/tekton/devflow-tekton-image-build-push-only.yaml` first when the cluster task or pipeline needs the monorepo-aware build logic
- use `deployments/tekton/meta-service-preproduction-build-pipelinerun.yaml` to build and push `meta-service:preproduction`
- use `deployments/tekton/config-service-preproduction-build-pipelinerun.yaml` to build and push `config-service:preproduction`
- use `deployments/tekton/network-service-preproduction-build-pipelinerun.yaml` to build and push `network-service:preproduction`
- use `deployments/tekton/release-service-preproduction-build-pipelinerun.yaml` to build and push `release-service:preproduction`
- use `deployments/tekton/runtime-service-preproduction-build-pipelinerun.yaml` to build and push `runtime-service:preproduction`
- use `kubectl apply -f deployments/pre-production/meta-service.yaml` to deploy namespace `devflow-pre-production`, configmap, service, and deployment for `meta-service`
- use `kubectl apply -f deployments/pre-production/config-service.yaml` to deploy `config-service`
- use `kubectl apply -f deployments/pre-production/network-service.yaml` to deploy `network-service`
- use `kubectl apply -f deployments/pre-production/release-bundle-argocd-repo-creds.yaml` to register the pre-production OCI release-bundle repo credentials/prefix for Argo CD
- use `kubectl apply -f deployments/pre-production/release-service.yaml` to deploy `release-service`
- use `kubectl apply -f deployments/pre-production/runtime-service.yaml` to deploy `runtime-service`
- use `kubectl apply -f deployments/pre-production/istio/shared-ingress.yaml` to expose shared pre-production HTTP routes through `devflow-pre-production.bei.com`

`release-service` remains a backend `ClusterIP` service inside Kubernetes,
but it is exposed at the shared edge through Istio path routing.

Istio edge note:
- `deployments/pre-production/istio/shared-ingress.yaml` is the committed pre-production edge contract for extracted service ingress
- the committed pre-production namespace is `devflow-pre-production`
- it uses one host, `devflow-pre-production.bei.com`
- current primary service routes are:
  - `/api/v1/meta/...` -> `meta-service`
  - `/api/v1/config/...` -> `config-service`
  - `/api/v1/network/...` -> `network-service`
  - `/api/v1/runtime/...` -> `runtime-service`
  - `/api/v1/release/...` -> `release-service`
- `/api/v1/platform/...` remains as a legacy compatibility route to `meta-service`
- the `Gateway` and `VirtualService` in that file should be updated together if the shared ingress host or gateway selector changes
- the backend Kubernetes `Service` objects remain `ClusterIP`; edge exposure belongs to Istio ingress rather than per-service load balancers

Pre-production manifest note:
- the platform currently does not support Kubernetes `Secret` mounts for this service bootstrap path
- `deployments/pre-production/meta-service.yaml` uses a `ConfigMap` named `meta-service-config`
- `deployments/pre-production/config-service.yaml` uses a `ConfigMap` named `config-service-config`
- `deployments/pre-production/network-service.yaml` uses a `ConfigMap` named `network-service-config`
- `deployments/pre-production/release-bundle-argocd-repo-creds.yaml` registers the `oci://zot.zot.svc.cluster.local:5000/devflow/releases` prefix as an Argo CD OCI repo-creds secret for release bundle pull access
- `deployments/pre-production/release-service.yaml` uses a `ConfigMap` named `release-service-config`
- `deployments/pre-production/runtime-service.yaml` uses a `ConfigMap` named `runtime-service-config`
- update `data.config.yaml` in those files before applying them to a real environment

Docker build note for this monorepo:
- the root `Dockerfile` still defaults to `meta-service`
- do **not** treat ad-hoc local Docker builds as the deployment contract for service-boundary extraction work
- non-default service image selection must be hardcoded in committed Tekton manifests
- only repo entrypoints under `cmd/` are buildable
- `config-service`, `network-service`, `release-service`, and `runtime-service` are now separate runnable images in this repo

The committed repo contract is:
- `meta-service` is the default runnable image from the root `Dockerfile`
- `config-service`, `network-service`, `release-service`, and `runtime-service` are selected explicitly through committed `BUILD_ARGS` in cluster build manifests

The committed Tekton manifests that make this explicit are:
- `deployments/tekton/meta-service-preproduction-build-pipelinerun.yaml`
- `deployments/tekton/config-service-preproduction-build-pipelinerun.yaml`
- `deployments/tekton/config-service-pipeline-run-template.yaml`
- `deployments/tekton/network-service-preproduction-build-pipelinerun.yaml`
- `deployments/tekton/network-service-pipeline-run-template.yaml`
- `deployments/tekton/release-service-preproduction-build-pipelinerun.yaml`
- `deployments/tekton/release-service-pipeline-run-template.yaml`
- `deployments/tekton/runtime-service-preproduction-build-pipelinerun.yaml`
- `deployments/tekton/runtime-service-pipeline-run-template.yaml`

Tekton task note:
- `deployments/tekton/devflow-tekton-image-build-and-push.yaml` accepts an optional `BUILD_ARGS` param
- `deployments/tekton/devflow-tekton-image-build-push-only.yaml` must also forward `BUILD_ARGS` into the build task
- the committed service manifests already hardcode the non-default build selection, for example:
  - `SERVICE_NAME=config-service --build-arg SERVICE_PORT=8082`
  - `SERVICE_NAME=network-service --build-arg SERVICE_PORT=8086`
  - `SERVICE_NAME=release-service --build-arg SERVICE_PORT=8083`
  - `SERVICE_NAME=runtime-service --build-arg SERVICE_PORT=8084`
