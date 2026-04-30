# M003 / S04 — Research

**Date:** 2026-05-01

## Summary

S04 owns the repo-artifact alignment requirements: **R016** (code / resource docs / service docs / verification surfaces must tell the same contract) and **R017** (a fresh reader can reconstruct ownership, required metadata, and runtime-service consumption assumptions from repo artifacts only). The code seams from S02 and S03 are already in place: `internal/release/service/release_bundle.go` produces the stable identity labels on rendered workloads, `internal/release/service/release.go` owns the Argo handoff object, `internal/runtime/observer/release_rollout.go` consumes workload labels and writes back only `observe_rollout` / `finalize_release`, and `internal/runtime/config/config.go` auto-starts the rollout observer when in-cluster wiring exists.

The remaining work is mostly documentation/verification synchronization, not new architecture. The authoritative contract already lives in `docs/system/flow-overview.md`, `docs/resources/release.md`, `docs/system/release-steps.md`, `docs/system/release-writeback.md`, `docs/services/release-service.md`, and `docs/services/runtime-service.md`. S04 should propagate that same story into the remaining reader-facing artifacts that operators and later agents actually use: the runtime-focused system/resource docs, the release detail UI contract, and the verification surfaces that claim what `bash scripts/verify.sh` proves. The planner should treat this as a targeted doc-alignment slice with small, explicit proof updates rather than a code-change slice.

## Recommendation

Start by aligning **reader-routing and ownership wording** across the runtime-facing docs, then tighten the **verification-surface wording** so recovery/verification docs explicitly say contract drift in code/docs is a verifier failure. This follows the repo's doc-synchronization policy and the `verify-before-complete` skill rule: claims of alignment should be backed by concrete proof surfaces, not implied by prose.

Prefer a two-task split:
1. **Contract surface alignment task** — update the remaining docs that still describe runtime observation, rollout/restart, or release detail timelines indirectly.
2. **Verification surface alignment task** — update `docs/policies/verification.md`, `scripts/README.md`, and possibly `docs/system/recovery.md` only where they should explicitly mention release-contract/doc consistency and the canonical stage/ownership docs.

No web/library research is needed. This is established Go/Kubernetes/Argo behavior already documented locally.

## Implementation Landscape

### Key Files

- `docs/system/flow-overview.md` — authoritative stage map. Already names stage owners, inputs, outputs, downstream consumers, metadata labels vs annotations, and stage-7 boundary rules. Treat this as the top source for doc wording.
- `docs/resources/release.md` — authoritative deploy-side contract. Already contains the Argo metadata table, step ownership table, callback-owner rules, and explicit `start_deployment` vs `observe_rollout` / `finalize_release` split.
- `docs/system/release-steps.md` — authoritative stable step-code/owner semantics. Use when normalizing any timeline wording elsewhere.
- `docs/system/release-writeback.md` — authoritative callback route contract. Already says runtime observers are active callback senders, not owners, and that `start_deployment` must not be callback-owned.
- `docs/services/release-service.md` — service-owner guide for stages 2–6 plus the release-owned callback boundary in stage 7. Good source for release-service ownership phrasing.
- `docs/services/runtime-service.md` — runtime-side contract. Already states the required release-owned labels, annotations-as-supplementary rule, PostgreSQL-free runtime read model, and rollout observer as callback sender only.
- `docs/system/runtime-observer.md` — runtime read-model explainer. It already mentions label-based identity rebuild and active release rollout observation, but it does **not** yet foreground the full release-contract language as strongly as the newer owner docs. Strong candidate for S04 alignment.
- `docs/resources/runtime-spec.md` — runtime API contract. It states runtime reads/actions clearly and mentions release-generated labels, but it does not yet spell out the full stable label set or the release-vs-runtime ownership split as explicitly as `docs/services/runtime-service.md`. Strong candidate for S04 alignment.
- `docs/resources/runtime-frontend-checklist.md` — short UI checklist. It currently explains read/write split but not the release-contract assumptions behind runtime identity reconstruction. Candidate for a small alignment note rather than a large rewrite.
- `docs/resources/frontend-ui.md` — UI contract for release detail and runtime pages. The release detail section already renders `steps`, but planner should check whether its language for the execution banner/timeline still matches the one-owner step semantics and whether it names the runtime/release ownership split clearly enough for fresh readers.
- `docs/policies/verification.md` — canonical verification contract. Already says code changes affecting API/domain/service behavior must include matching doc updates. Candidate for a targeted sentence that release-contract/doc drift is part of verification expectations.
- `scripts/README.md` — reader-facing explanation of what `bash scripts/verify.sh` proves. Candidate for an explicit note that release-flow contract docs must stay in sync with code/test surfaces.
- `docs/system/recovery.md` — failure-routing guide. Only touch if needed to route release-contract drift more directly to the authoritative docs.
- `internal/runtime/observer/release_rollout.go` — code anchor for stage-7 runtime observation consumption. Reads `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id`, falls back to `app.kubernetes.io/name` for deployment name, and posts only `observe_rollout` / `finalize_release` updates.
- `internal/runtime/config/config.go` — code anchor proving clustered runtime startup auto-starts the rollout observer when in-cluster config exists.
- `internal/release/domain/types.go` — canonical constants for release/runtime label names; useful when doc wording should point to a code source of truth.

### Natural Seams

1. **Runtime contract docs seam**
   - Files: `docs/system/runtime-observer.md`, `docs/resources/runtime-spec.md`, `docs/resources/runtime-frontend-checklist.md`
   - Goal: make runtime-facing artifacts explicitly say they consume the release-owned label contract and do not own release truth.

2. **Release/operator UI docs seam**
   - Files: `docs/resources/frontend-ui.md`
   - Goal: ensure release-detail timeline wording matches S03 semantics (`start_deployment` = release-owned handoff; rollout callbacks advance `observe_rollout` / `finalize_release`).

3. **Verification/recovery docs seam**
   - Files: `docs/policies/verification.md`, `scripts/README.md`, maybe `docs/system/recovery.md`
   - Goal: ensure the repo’s proof surfaces claim the same contract-alignment responsibility that S04 is delivering.

### Build Order

1. **Audit runtime-facing docs first** because R017 depends on a fresh reader being able to reconstruct runtime consumption assumptions without jumping through multiple files. This is the highest-value reader path after `flow-overview.md`.
2. **Then align release/operator UI docs** so resource-facing behavior mirrors the normalized step model from S03.
3. **Last, update verification/recovery wording** so the repo’s handoff/proof docs accurately describe what contract drift means and where to look next.

This order keeps the work reader-first: first fix what a fresh agent/operator reads to understand the contract, then fix what the verifier/recovery surfaces claim about that contract.

### Verification Approach

Primary verification is doc-and-proof-surface consistency plus repo verification from the root:

- `bash scripts/verify.sh`
- optionally `go test ./internal/release/service ./internal/runtime/observer ./internal/release/transport/http` if the executor wants fresh seam-level proof while editing docs around those contracts

Reader-proof checklist for the executor/planner:

- Starting from `docs/system/flow-overview.md`, a reader can route to `docs/services/release-service.md` and `docs/services/runtime-service.md` without finding conflicting ownership claims.
- Runtime docs consistently say the stable identity contract is labels (`app.kubernetes.io/name`, `devflow.io/release-id`, `devflow.application/id`, `devflow.environment/id`) and annotations are supplementary only.
- Runtime docs consistently say `runtime-service` may send rollout callbacks but does not own release truth.
- Release/UI docs consistently say `start_deployment` is release-service-owned handoff, while callback-owned rollout confirmation advances `observe_rollout` and `finalize_release`.
- Verification/recovery docs consistently say doc drift is a contract failure to fix, not accepted migration noise.

## Constraints

- Use the existing local authority ladder: `docs/system/flow-overview.md`, `docs/resources/release.md`, `docs/system/release-steps.md`, and `docs/system/release-writeback.md` are the current truth; S04 should propagate them, not invent new contract wording.
- Keep the current ownership model unchanged: `release-service` owns release truth; `runtime-service` consumes runtime state plus release-defined metadata and may send callbacks.
- Do not expand into new business logic. This slice is alignment work unless exploration exposes a concrete contradiction between docs and code.
- Preserve the repo’s layered doc model from `AGENTS.md` / `docs/system/architecture.md`: system docs for current truth, service docs for owner diagnostics, resource docs for API/resource contracts, policy docs for durable rules.

## Common Pitfalls

- **Updating secondary docs without preserving reader routing** — Many files intentionally say “start with `docs/system/flow-overview.md`.” Keep that routing pattern intact instead of duplicating full lifecycle explanations everywhere.
- **Letting runtime docs imply release ownership drift** — Any wording that makes runtime-service sound like the owner of release status, steps, or callback policy would regress D005/D007 and contradict `internal/runtime/observer/release_rollout.go`.
- **Re-describing metadata loosely** — The stable label set is explicit now. Avoid vague phrases like “release metadata labels” in docs that should enumerate the concrete keys.
- **Claiming verifier enforcement that does not exist** — Verification docs may say what the contract expects, but do not invent a new script behavior unless the slice also adds it.

## Open Risks

- `docs/resources/frontend-ui.md` is large and may contain older release-detail wording in multiple places; planners should scope the edit narrowly to release timeline/runtime sections rather than broad rewriting.
- There may be minor wording drift between `docs/system/runtime-observer.md` and `docs/resources/runtime-spec.md` about restart/rollout language; keep API semantics stable while aligning ownership language.

## Skills Discovered

| Technology | Skill | Status |
|------------|-------|--------|
| Go documentation | `samber/cc-skills-golang@golang-documentation` | available |
| Monorepo governance | `aj-geddes/useful-ai-prompts@monorepo-management` | available |
| Argo CD / Kubernetes contract docs | `personamanagmentlayer/pcl@argocd-expert` | available |

