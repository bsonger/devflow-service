# Docker Baseline

This document defines the controlled Docker baseline for `devflow-service`.

## Go and image baseline

- target Go baseline: `1.26.2`
- local builds, CI, builder images, and packaging must align to `Go 1.26.2`
- approved builder image for in-repo multi-stage builds: `registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22`
- expected published builder platform: `linux/amd64`

## Base image policy

- build-time tools belong in controlled builder images
- runtime installation dependencies belong in controlled runtime base images
- service Dockerfiles should use thin multi-stage builds
- the repo-local builder baseline source lives at `docker/golang-builder.Dockerfile`

## Service Dockerfile rules

Allowed:
- choose a controlled base image
- compile in an approved builder stage
- copy built artifacts and tracked runtime files into the final runtime stage
- declare runtime metadata such as `ENTRYPOINT`, `CMD`, or `EXPOSE`
- keep one root `Dockerfile` that defaults to the active local contract

Banned:
- `apk add`
- `apt-get`
- `yum`
- `dnf`
- `go install`
- any inline package or tool installation step

If a dependency needs installation, promote it into the controlled base-image contract first.

## Monorepo build selection rule

The root `Dockerfile` may remain parameterized internally, but non-default service selection must not be treated as a local operator choice.
For `config-service`, `network-service`, `release-service`, and `runtime-service`, the authoritative packaging contract is the committed Tekton manifest set under `deployments/tekton/`.

Committed cluster-build examples:

```text
deployments/tekton/config-service-preproduction-build-pipelinerun.yaml
deployments/tekton/config-service-pipeline-run-template.yaml
deployments/tekton/network-service-preproduction-build-pipelinerun.yaml
deployments/tekton/network-service-pipeline-run-template.yaml
deployments/tekton/release-service-preproduction-build-pipelinerun.yaml
deployments/tekton/release-service-pipeline-run-template.yaml
deployments/tekton/runtime-service-preproduction-build-pipelinerun.yaml
deployments/tekton/runtime-service-pipeline-run-template.yaml
```

Parameterization does not create new services by itself.
Only domains with a runnable `cmd/<service>/main.go` entrypoint in this repo are separately buildable images here.
In particular:
- `config-service` is now a runnable extracted entrypoint
- `network-service` is now a runnable extracted entrypoint
- `runtime-service` is now a runnable extracted entrypoint

The committed docs, runnable `cmd/` entrypoints, and cluster build manifests are the authoritative contract.
Local ad-hoc Docker builds must not be treated as proof that a new service boundary exists.

## Builder image publication rule

When republishing the controlled builder image from this repo, publish the amd64 variant explicitly:

```sh
docker buildx build \
  --platform linux/amd64 \
  -f docker/golang-builder.Dockerfile \
  -t registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22 \
  --push .
```
