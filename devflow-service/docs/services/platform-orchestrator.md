# Platform Orchestrator

This file is migrated from `devflow-control` as a cross-repo reference.
It is a service-boundary summary, not current repo-local implementation truth.

## Owns

- platform command/query facade only
- cross-service read composition
- console-first write orchestration without taking domain ownership

## Does Not Own

- domain resources such as `Project`, `Application`, `Environment`, `Manifest`, `Image`, or `Release`

## Upstream Dependencies

- app/config/network/release/runtime owner services
- shared backend primitives

## Downstream Consumers

- `platform-web`
- future platform-facing automation
