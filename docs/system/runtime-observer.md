# Runtime Observer / Index Model

## Purpose

This document explains the active runtime read-model contract for workload and pod display.

Use it to answer:

- where runtime page data should come from
- which parts call Kubernetes directly
- what the current pre-production gap still is

## Core rule

Runtime display should follow one split:

- read model: observer/index-backed runtime records
- action model: direct Kubernetes mutations only when the user explicitly performs an action

That means:

- workload overview does not query Kubernetes on every page load
- pod list does not query Kubernetes on every page load
- delete pod and restart workload still call Kubernetes

## Read-model resources

Runtime-service currently persists and serves:

- `RuntimeObservedWorkload`
- `RuntimeObservedPod`
- `RuntimeOperation`

The workload overview is controller-level state.
The pod list is instance-level state.

## Public runtime read surface

Shared ingress routes:

- `GET /api/v1/runtime/workload?application_id=...&environment_id=...`
- `GET /api/v1/runtime/pods?application_id=...&environment_id=...`

Frontend usage model:

- call `runtime/workload` for the summary card and conditions
- call `runtime/pods` for the pod table
- refresh both after any explicit runtime action succeeds

## Internal observer write surface

Observer callbacks:

- `POST /api/v1/internal/runtime-spec-workloads/sync`
- `POST /api/v1/internal/runtime-spec-workloads/delete`
- `POST /api/v1/internal/runtime-spec-pods/sync`
- `POST /api/v1/internal/runtime-spec-pods/delete`

These routes are runtime-owned index write APIs, not user-facing APIs.

## Current pre-production status

As of April 29, 2026:

- runtime-service supports workload overview reads
- runtime-service supports internal workload summary sync
- the pre-production database contains `runtime_observed_workloads`
- shared ingress has been verified for public `GET /api/v1/runtime/workload`

Known remaining gap:

- `resource-observer` automatic workload sync has not yet been updated through this repo's committed runtime-observer integration path
- pre-production proof currently depends on runtime-service support plus manually posted workload sync data

## Operator mental model

When a user opens the runtime page, think:

1. read latest workload summary from runtime index
2. read latest pod list from runtime index
3. when the user clicks restart or delete, call Kubernetes through runtime-service
4. refresh workload + pods from runtime index after the action

## Source pointers

- runtime service behavior: `docs/services/runtime-service.md`
- runtime API contract: `docs/resources/runtime-spec.md`
- frontend runtime page rules: `docs/resources/frontend-ui.md`
