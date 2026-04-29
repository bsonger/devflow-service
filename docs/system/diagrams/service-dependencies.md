# Service dependency diagram

```mermaid
flowchart LR
    User[Platform / UI / Operator]

    User --> CS[config-service]
    User --> NS[network-service]
    User --> RS[release-service]
    User --> RTS[runtime-service]

    META[meta-service]
    PG[(PostgreSQL)]
    KS[Kubernetes API]
    TK[Tekton]
    ARGO[Argo CD]
    ZOT[zot OCI registry]
    ALIYUN[Aliyun image registry]

    CS --> META
    CS --> PG

    NS --> META
    NS --> PG

    RS --> META
    RS --> CS
    RS --> NS
    RS --> PG
    RS --> TK
    RS --> ARGO
    RS --> ZOT
    RS --> KS

    RTS --> KS
    RTS --> RS

    TK --> ALIYUN
    ARGO --> ZOT
    ARGO --> KS
```

## Notes

- `config-service` owns `AppConfig` and `WorkloadConfig`
- `network-service` owns `Service` and `Route`
- `release-service` owns `Manifest`, `Release`, `Image`, and `Intent`
- `runtime-service` owns runtime inspection, runtime observation, and runtime operations
- `release-service` is the main cross-service composer
- `runtime-service` is Kubernetes-first for operator-facing reads/actions, with a memory-backed default HTTP path
- PostgreSQL-backed runtime repository and release-rollout observer support code still exist
