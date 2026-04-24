# Docker Baseline

This document defines the controlled Docker baseline for `devflow-service`.

## Go and image baseline

- target Go baseline: `1.26.2`
- local builds, CI, builder images, and packaging must align to `Go 1.26.2`

## Base image policy

- build-time tools belong in controlled builder images
- runtime installation dependencies belong in controlled runtime base images
- service Dockerfiles are packaging-only surfaces
- the repo-local builder baseline source lives at `docker/golang-builder.Dockerfile`

## Service Dockerfile rules

Allowed:
- choose a controlled base image
- copy built artifacts and tracked runtime files
- declare runtime metadata such as `ENTRYPOINT`, `CMD`, or `EXPOSE`

Banned:
- `apk add`
- `apt-get`
- `yum`
- `dnf`
- `go install`
- any inline package or tool installation step

If a dependency needs installation, promote it into the controlled base-image contract first.
