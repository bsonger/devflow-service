# Worker Runtime Policy

## Reader and outcome

This policy is for engineers and agents changing background execution, runtime helpers, or future worker loops in `devflow-service`.

After reading it, a fresh reader should be able to:
- decide what belongs in worker/runtime code versus `service`, `repository`, or HTTP handlers
- keep intent claiming, lease handling, and terminal state updates explicit
- wire background execution with trace, logs, and metrics that remain debuggable
- avoid turning runtime helpers into hidden business or transport god-layers

## Scope

This policy governs code under:

- `internal/intent/service`
- `internal/release/runtime`
- `internal/platform/runtime`
- future background loops, worker entrypoints, and lease-driven executors added to this repo

It complements:

- `docs/policies/service-layer.md`
- `docs/policies/repository-layer.md`
- `docs/policies/downstream-client.md`
- `docs/policies/observability-logging.md`

## Core rules

1. Worker/runtime code owns asynchronous execution mechanics, not HTTP behavior.
2. Worker/runtime code must keep state transitions explicit.
3. Claim and lease behavior must live in persisted state, not only in memory.
4. Background execution must be idempotency-aware.
5. Logs, metrics, and traces must make retries and failures diagnosable without leaking secrets.

## What belongs in worker/runtime code

Worker/runtime code is the place for:

- selecting direct versus intent-driven execution modes
- claiming pending intents with a lease
- maintaining worker-owned execution context for long-running operations
- bootstrapping observability sidecars such as metrics and pprof servers
- building runtime-only config objects such as image or manifest registry settings
- wrapping dependency calls with shared observability helpers when runtime execution crosses process boundaries

## What does not belong in worker/runtime code

Worker/runtime code must not:

- write HTTP responses
- import `github.com/gin-gonic/gin`
- import `internal/platform/httpx`
- import `internal/*/transport/http`
- hide business decisions in bootstrap helpers
- replace repository-owned persistence with ad hoc in-memory coordination

Transport behavior stays in HTTP handlers.
Business orchestration stays in domain services.
Persistence stays in repositories.

## Intent and lease rules

When work is driven by `execution_intents`, prefer these rules:

- `pending` work must be claimed explicitly
- a claim must record `claimed_by`, `claimed_at`, and `lease_expires_at`
- a worker should treat lease ownership as durable state, not a local assumption
- terminal or handoff states should clear worker claim fields when the repository contract expects that
- failure messages should stay short, safe, and useful for operators

Avoid hidden state transitions that cannot be reconstructed from persisted intent records.

## Idempotency and retries

Background execution should assume retries will happen.

Prefer:

- idempotent lookups before mutating downstream systems
- resource-oriented reconciliation using stable identifiers such as `intent_id`, `image_id`, `manifest_id`, or `release_id`
- explicit retry ownership and backoff policy in the caller or worker loop

Avoid:

- fire-and-forget mutation without durable state
- retry behavior hidden inside many nested helpers
- duplicate side effects that depend on best-effort local memory

## Observability rules

Worker/runtime code must support diagnosis of asynchronous failures.

Prefer logs with fields such as:

- `worker_id`
- `intent_id`
- `intent_kind`
- `resource`
- `resource_id`
- `operation`
- `result`

Prefer traces that:

- create a fresh worker span when execution begins in a background context
- reinject logger context after creating spans
- wrap downstream calls with shared dependency observation where practical

Prefer a lightweight last-failure snapshot for operator-facing status surfaces such as `/internal/status`.
That snapshot should contain:

- failure time
- component
- operation
- target
- short safe message

Do not persist secrets, kubeconfigs, tokens, or raw payload dumps in that snapshot.

Prefer metrics that stay low-cardinality.
Do not use `intent_id`, `trace_id`, `request_id`, or other unique identifiers as metric labels.

## Runtime bootstrap rules

Code under `internal/platform/runtime` should stay generic.

It may own:

- config loading and port resolution glue
- service start logging
- observability initialization
- metrics and pprof server startup

It must not own:

- domain-specific release decisions
- resource-specific validation
- cross-domain workflow meaning

## Dependency rules

Background execution may depend on:

- domain services
- repository interfaces
- runtime-only config helpers
- shared downstream clients
- platform observability helpers

Avoid coupling runtime helpers directly to HTTP handler packages or UI-facing transport code.

## Security rules

Do not log or persist these values in worker/runtime diagnostics:

- passwords
- tokens
- registry secrets
- kubeconfigs
- authorization headers
- cookies
- full sensitive payloads

Failure logs should prefer summaries over raw payload dumps.

## Recommended checklist

When adding or changing worker/runtime code, check:

1. is this asynchronous execution plumbing rather than HTTP or repository logic
2. are claim and lease transitions explicit and durable
3. can retries happen safely without duplicate side effects
4. did I keep Gin and HTTP response helpers out of runtime code
5. are logs, metrics, and traces sufficient for debugging without leaking secrets
6. is bootstrap code still generic rather than business-specific

## Verification expectations

When changing worker/runtime code:

- keep HTTP-edge concerns out of runtime helper packages
- keep persisted claim and lease ownership in repository-backed state
- keep docs, verification, and runtime helpers aligned in the same change cycle
