# Resources

This directory contains current resource-contract docs for `devflow-service`.

Use:
- `application.md` for the `Application` resource contract
- `application-environment.md` for the application-to-environment binding contract
- `frontend-ui.md` for the frontend information architecture and page-level field contract
- `project.md` for the `Project` resource contract
- `cluster.md` for the `Cluster` resource contract
- `environment.md` for the `Environment` resource contract
- `appconfig.md` for the `AppConfig` resource contract
- `workloadconfig.md` for the `WorkloadConfig` resource contract
- `service.md` for the application-owned `Service` resource contract
- `route.md` for the application-owned `Route` resource contract
- `runtime-spec.md` for the `RuntimeSpec` and lookup-side runtime revision contract
- `manifest.md` for the `Manifest` resource contract
- `intent.md` for the `Intent` resource contract
- `release.md` for the `Release` resource contract

These docs should describe current resource behavior, request and response shape, validation, and source pointers in this repo.
For shared resource CRUD, pagination, filter, and soft-delete rules, start with `docs/policies/resource-api.md`.
Keep service-boundary and merge-status docs under `docs/services/`.
