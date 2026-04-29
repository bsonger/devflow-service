# Diagrams

## Purpose

This document is now the index for repo-local system diagrams.
Use the split docs below instead of one oversized page.

## Diagram index

- `docs/system/diagrams/service-dependencies.md`
  - service-to-service dependencies
  - external systems
  - ownership-oriented dependency view
- `docs/system/diagrams/release-flow.md`
  - manifest build flow
  - release deploy flow
  - release/runtime boundary
- `docs/system/diagrams/runtime-flow.md`
  - runtime reads
  - runtime actions
  - observer/index mental model
- `docs/system/diagrams/resource-ownership.md`
  - resource ownership
  - cross-service dependency view

## Current contract note

- `runtime-service` should be understood as Kubernetes-first
- the default runtime HTTP path is memory-backed and rebuilt by observer sync
- PostgreSQL-backed runtime repository and release-rollout observer support code still exist
- runtime observer state is rebuilt in-process after restart from Kubernetes observation
- runtime reads come from runtime-owned observed state
- runtime actions call Kubernetes for explicit operator-triggered mutations

For narrative context, also see:

- `docs/system/architecture.md`
- `docs/system/flow-overview.md`
- `docs/services/runtime-service.md`
- `docs/system/runtime-storage-model.md`
