# Services

This directory contains current service-boundary docs for `devflow-service`.

Use:
- `meta-service.md` for the active service boundary and migration state
- `app-service.md` for the app-owned boundary that still remains inside `meta-service`
- `config-service.md`, `network-service.md`, `release-service.md`, and `runtime-service.md` for extracted runnable service boundaries now hosted in this repo
- `platform-orchestrator.md`, `platform-web.md`, and `service-common.md` for additional cross-repo service-boundary context that is still reference-only in this repo
- `docs/resources/` for current resource contracts, API behavior, and validation rules

These docs should describe the current code in this repo.
Do not treat migrated material from sibling repos as authoritative if it conflicts with the current implementation here.
