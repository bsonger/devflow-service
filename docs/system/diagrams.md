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
    RTS -. current implementation still persists runtime records .-> PG

    TK --> ALIYUN
    ARGO --> ZOT
    ARGO --> KS
```

### Notes

- `config-service` owns `AppConfig` and `WorkloadConfig`
- `network-service` owns `Service` and `Route`
- `release-service` owns `Manifest`, `Release`, `Image`, and `Intent`
- `runtime-service` owns runtime inspection and runtime operations
- `release-service` is the main cross-service composer: it reads application / environment / cluster metadata from `meta-service`, workload and app config from `config-service`, and network topology from `network-service`
- `runtime-service` should be understood as Kubernetes-first; current code still contains runtime persistence in PostgreSQL, but pod listing / pod delete / rollout restart are Kubernetes-driven operations

## 2. Manifest build sequence diagram

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

### Notes

- `Manifest` is the release-owned durable build record
- its frozen inputs come from `meta-service`, `config-service`, and `network-service`
- Tekton produces the image result, but the durable system record lives on the manifest row

## 3. Release deploy sequence diagram

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

## 4. Resource ownership diagram

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
        RWORK[RuntimeObservedWorkload]
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

## 5. Cross-service resource dependency view

```mermaid
flowchart LR
    APPMETA[Application / Environment / Cluster]
    AC[AppConfig]
    WC[WorkloadConfig]
    SVC[Service]
    RT[Route]
    MF[Manifest]
    RL[Release]
    APP[Argo Application]
    WK[Workload in Kubernetes]

    APPMETA --> MF
    APPMETA --> RL
    WC --> MF
    SVC --> MF
    MF --> RL
    AC --> RL
    RT --> RL
    RL --> APP
    APP --> WK
```

### Notes

- `Manifest` freezes application metadata, workload config, and service snapshot for build-time and later deploy-time consumption
- `Release` freezes app config and route snapshot at release time, then resolves the final deploy target from meta-service
- Argo deploys the release-generated bundle, not the original Git config repo directly
- runtime pod inspection and runtime operations act on live Kubernetes workloads rather than reading release state from PostgreSQL first
- runtime workload overview and pod display both prefer runtime-owned observed index data

## Source pointers

- service ownership: `docs/services/`
- resource ownership: `docs/resources/`
- current repo shape: `docs/system/architecture.md`
- release writeback: `docs/system/release-writeback.md`
- release steps: `docs/system/release-steps.md`
