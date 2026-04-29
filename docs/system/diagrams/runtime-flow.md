# Runtime flow diagram

## Runtime read / action sequence diagram

```mermaid
sequenceDiagram
    participant U as Platform / Operator
    participant RTS as runtime-service
    participant IDX as Runtime observer / index
    participant K8S as Kubernetes

    U->>RTS: GET /api/v1/runtime/workload?application_id&environment_id
    RTS->>IDX: read observed workload summary
    IDX-->>RTS: latest workload state
    RTS-->>U: workload overview

    U->>RTS: GET /api/v1/runtime/pods?application_id&environment_id
    RTS->>IDX: read observed pod list
    IDX-->>RTS: latest pod state
    RTS-->>U: pod list

    U->>RTS: POST /api/v1/runtime/rollouts
    RTS->>K8S: patch Deployment restartedAt
    K8S-->>RTS: accepted
    RTS-->>U: action accepted

    U->>RTS: DELETE /api/v1/runtime/pods/{pod_name}
    RTS->>K8S: delete Pod
    K8S-->>RTS: accepted
    RTS-->>U: action accepted
```

## Notes

- runtime reads should prefer observer/index-backed state
- runtime actions should call Kubernetes only for explicit user-triggered mutations
- after an action succeeds, the UI should refresh workload + pod reads from the runtime index
- the default runtime HTTP path is memory-backed and does not load runtime rows from PostgreSQL at startup
- PostgreSQL-backed runtime repository and release-rollout observer support code still exist; see `docs/system/runtime-storage-model.md`
- observer state is rebuilt in-process after restart from Kubernetes observations
