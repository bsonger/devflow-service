# Deployments

This directory is reserved for in-repo deployment artifacts for `devflow-service`.

Examples:
- Kubernetes manifests
- Helm charts
- environment-specific deployment overlays

Keep build-time Docker base-image assets under `docker/`.

Current local pre-production flow:
- apply `deployments/tekton/devflow-tekton-image-build-and-push.yaml` first when the cluster task needs the monorepo-aware build logic
- use `deployments/tekton/meta-service-preproduction-build-pipelinerun.yaml` to build and push `meta-service:preproduction`
- use `deployments/tekton/config-service-preproduction-build-pipelinerun.yaml` to build and push `config-service:preproduction`
- use `deployments/tekton/network-service-preproduction-build-pipelinerun.yaml` to build and push `network-service:preproduction`
- use `deployments/tekton/release-service-preproduction-build-pipelinerun.yaml` to build and push `release-service:preproduction`
- use `deployments/tekton/runtime-service-preproduction-build-pipelinerun.yaml` to build and push `runtime-service:preproduction`
- use `kubectl apply -f deployments/pre-production/meta-service.yaml` to deploy namespace, configmap, service, and deployment for `meta-service`
- use `kubectl apply -f deployments/pre-production/config-service.yaml` to deploy `config-service`
- use `kubectl apply -f deployments/pre-production/network-service.yaml` to deploy `network-service`
- use `kubectl apply -f deployments/pre-production/runtime-service.yaml` to deploy `runtime-service`
- use `kubectl apply -f deployments/pre-production/istio/shared-ingress.yaml` to expose `config-service`, `network-service`, and `runtime-service` through one Istio host with per-service subpaths

Istio edge note:
- `deployments/pre-production/istio/shared-ingress.yaml` is the committed pre-production edge contract for extracted service ingress
- it uses one host, `devflow-pre.example.com`, with service-specific subpaths: `/config`, `/network`, and `/runtime`
- the `Gateway` and `VirtualService` in that file should be updated together if the shared ingress host or gateway selector changes
- the backend Kubernetes `Service` objects remain `ClusterIP`; edge exposure belongs to Istio ingress rather than per-service load balancers

Pre-production manifest note:
- the platform currently does not support Kubernetes `Secret` mounts for this service bootstrap path
- `deployments/pre-production/meta-service.yaml` uses a `ConfigMap` named `meta-service-config`
- `deployments/pre-production/config-service.yaml` uses a `ConfigMap` named `config-service-config`
- `deployments/pre-production/network-service.yaml` uses a `ConfigMap` named `network-service-config`
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
- the committed service manifests already hardcode the non-default build selection, for example:
  - `SERVICE_NAME=config-service --build-arg SERVICE_PORT=8082`
  - `SERVICE_NAME=network-service --build-arg SERVICE_PORT=8086`
  - `SERVICE_NAME=release-service --build-arg SERVICE_PORT=8083`
  - `SERVICE_NAME=runtime-service --build-arg SERVICE_PORT=8084`
