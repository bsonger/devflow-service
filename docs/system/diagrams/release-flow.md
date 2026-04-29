# Release flow diagrams

## Manifest build sequence diagram

```mermaid
sequenceDiagram
    participant U as Platform / Operator
    participant RS as release-service
    participant META as meta-service
    participant CS as config-service
    participant NS as network-service
    participant TK as Tekton
    participant IMG as Image Registry
    participant PG as PostgreSQL

    U->>RS: POST /api/v1/release/manifests
    RS->>META: resolve application projection
    RS->>CS: read workload config
    RS->>NS: read services
    RS->>TK: start image build pipeline
    RS->>PG: create manifest row with frozen snapshots
    TK->>IMG: build and push application image
    TK->>RS: write back task / status / result
    RS->>PG: update manifest status to Ready/Succeeded
```

## Notes

- `Manifest` is the release-owned durable build record
- frozen inputs come from `meta-service`, `config-service`, and `network-service`
- Tekton produces the image result, but the durable system record lives on the manifest row

## Release deploy sequence diagram

```mermaid
sequenceDiagram
    participant U as Platform / Operator
    participant RS as release-service
    participant META as meta-service
    participant CS as config-service
    participant NS as network-service
    participant PG as PostgreSQL
    participant ZOT as zot
    participant ARGO as Argo CD
    participant CB as Callback sender
    participant K8S as Kubernetes

    U->>RS: POST /api/v1/release/releases
    RS->>PG: read frozen manifest
    RS->>CS: read app config for target environment
    RS->>NS: read routes for target environment
    RS->>META: resolve application / environment / cluster deploy target
    RS->>PG: create release row + freeze live inputs
    RS->>PG: store rendered release bundle
    RS->>ZOT: publish deployment bundle as OCI artifact
    RS->>ARGO: create or update Application
    RS->>ARGO: request sync
    ARGO->>ZOT: pull OCI bundle
    ARGO->>K8S: apply ServiceAccount / ConfigMap / Service / Deployment / VirtualService
    Note over K8S,RS: rollout progress may be reported later through release-owned callbacks
    CB->>RS: POST /api/v1/verify/release/steps
    RS->>PG: persist release step updates + final status
```

## Current release stages

1. `freeze_inputs`
2. `ensure_namespace`
3. `ensure_pull_secret`
4. `ensure_appproject_destination`
5. `render_deployment_bundle`
6. `publish_bundle`
7. `create_argocd_application`
8. `start_deployment`
9. `observe_rollout`
10. `finalize_release`

See also:

- `docs/system/release-steps.md`
- `docs/system/release-writeback.md`

## Boundary note

- `release-service` starts deployment by creating/updating the Argo CD `Application`
- `release-service` no longer polls Argo CD application status directly during release detail reads
- rollout progress, when reported asynchronously, should come back through release-owned writeback routes
- do not read this diagram as proof that `runtime-service` currently auto-starts release rollout writeback; the in-tree rollout observer is not started by the active runtime startup path
