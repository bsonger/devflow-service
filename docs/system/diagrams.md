# Diagrams

## Purpose

This document provides visual repo-local diagrams for the current DevFlow backend runtime shape.
It is visualization support for the active local contract, not a replacement for the owning resource or service docs.

Use these diagrams when you need to explain:

- service-to-service dependencies
- `Manifest` and `Release` execution flow
- resource ownership boundaries

## 1. Service dependency diagram

```mermaid
flowchart LR
    User[Platform / UI / Operator]

    User --> CS[config-service]
    User --> NS[network-service]
    User --> RS[release-service]
    User --> RTS[runtime-service]

    CS --> PG[(PostgreSQL)]
    NS --> PG
    RS --> PG
    RTS --> PG

    RS --> TK[Tekton]
    RS --> ARGO[Argo CD]
    RS --> ZOT[zot OCI registry]
    RS --> KS[Kubernetes API]

    RTS --> KS

    TK --> ALIYUN[Aliyun image registry]
    ARGO --> ZOT
    ARGO --> KS
```

### Notes

- `config-service` owns `AppConfig` and `WorkloadConfig`
- `network-service` owns `Service` and `Route`
- `release-service` owns `Manifest`, `Release`, `Image`, and `Intent`
- `runtime-service` owns runtime inspection and runtime operations
- all four services currently depend on the shared PostgreSQL cluster

## 2. Manifest / Release sequence diagram

```mermaid
sequenceDiagram
    participant U as Platform / Operator
    participant RS as release-service
    participant TK as Tekton
    participant IMG as Image Registry
    participant ZOT as zot
    participant ARGO as Argo CD
    participant K8S as Kubernetes
    participant PG as PostgreSQL

    U->>RS: POST /api/v1/release/manifests
    RS->>PG: create manifest row
    RS->>TK: start image build pipeline
    TK->>IMG: build and push application image
    TK->>RS: write back task / status / result
    RS->>PG: update manifest status to Ready/Succeeded

    U->>RS: POST /api/v1/release/releases
    RS->>PG: create release row + freeze inputs
    RS->>PG: store rendered release bundle
    RS->>ZOT: publish deployment bundle as OCI artifact
    RS->>ARGO: create or update Application
    RS->>ARGO: request sync
    ARGO->>ZOT: pull OCI bundle
    ARGO->>K8S: apply ServiceAccount / ConfigMap / Service / Deployment
    RS->>ARGO: read Application status
    RS->>PG: reconcile release steps + final status
```

### Current release stages

For normal rolling release, the key steps are:

1. `freeze_inputs`
2. `render_deployment_bundle`
3. `publish_bundle`
4. `create_argocd_application`
5. `start_deployment`
6. `observe_rollout`
7. `finalize_release`

See also:

- `docs/system/release-steps.md`
- `docs/system/release-writeback.md`

## 3. Resource ownership diagram

```mermaid
flowchart TB
    subgraph Config["config-service"]
        AC[AppConfig]
        WC[WorkloadConfig]
    end

    subgraph Network["network-service"]
        SVC[Service]
        RT[Route]
    end

    subgraph Release["release-service"]
        MF[Manifest]
        RL[Release]
        IMG[Image]
        INT[Intent]
    end

    subgraph Runtime["runtime-service"]
        RSPEC[RuntimeSpec]
        RREV[RuntimeSpecRevision]
        RPOD[RuntimeObservedPod]
        ROP[RuntimeOperation]
    end
```

### Ownership rules

- one resource belongs to one service only
- `Manifest` and `Release` are release-owned resources
- `Service` and `Route` are network-owned resources
- `AppConfig` is config-owned and is consumed by release at freeze time
- runtime data is runtime-owned even when release reads deployment health indirectly through Argo

## 4. Cross-service resource dependency view

```mermaid
flowchart LR
    AC[AppConfig]
    WC[WorkloadConfig]
    SVC[Service]
    RT[Route]
    MF[Manifest]
    RL[Release]
    APP[Argo Application]
    WK[Workload in Kubernetes]

    AC --> RL
    SVC --> MF
    WC --> MF
    MF --> RL
    RT --> RL
    RL --> APP
    APP --> WK
```

### Notes

- `Manifest` freezes workload and service snapshot
- `Release` freezes app config and route snapshot at release time
- Argo deploys the release-generated bundle, not the original Git config repo directly

## Source pointers

- service ownership: `docs/services/`
- resource ownership: `docs/resources/`
- current repo shape: `docs/system/architecture.md`
- release writeback: `docs/system/release-writeback.md`
- release steps: `docs/system/release-steps.md`
