# Runtime Observer / Index Model

## Purpose

This document explains the active runtime read-model contract for workload and pod display.

Use it to answer:

- where runtime page data should come from
- which parts call Kubernetes directly
- what the active runtime read path is in pre-production

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

- `POST /api/v1/internal/runtime-workloads/sync`
- `POST /api/v1/internal/runtime-workloads/delete`
- `POST /api/v1/internal/runtime-pods/sync`
- `POST /api/v1/internal/runtime-pods/delete`

These routes are runtime-owned index write APIs, not user-facing APIs.

## Current pre-production status

As of April 29, 2026:

- runtime-service supports workload overview reads
- runtime-service supports internal workload summary sync
- the pre-production database contains `runtime_observed_workloads`
- shared ingress has been verified for public `GET /api/v1/runtime/workload`
- pre-production runtime-service has been verified to repopulate both workload and pod index rows automatically after those rows are deleted from PostgreSQL
- pre-production runtime observation is now owned by the in-process Kubernetes observer inside `runtime-service`

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
