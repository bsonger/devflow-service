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

Banned:
- `apk add`
- `apt-get`
- `yum`
- `dnf`
- `go install`
- any inline package or tool installation step

If a dependency needs installation, promote it into the controlled base-image contract first.

## Builder image publication rule

When republishing the controlled builder image from this repo, publish the amd64 variant explicitly:

```sh
docker buildx build \
  --platform linux/amd64 \
  -f docker/golang-builder.Dockerfile \
  -t registry.cn-hangzhou.aliyuncs.com/devflow/golang-builder:1.26.2-alpine3.22 \
  --push .
```
