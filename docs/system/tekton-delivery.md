# Tekton Delivery

## Purpose

This note explains **Tekton / GitOps delivery terminology** used around image build, manifest-repo patch ideas, and downstream notification.

> **Boundary note:** this document does **not** redefine the `Manifest` API resource contract.
> In the active code and resource docs, `Manifest` is the **build-side API resource** owned by `release-service` and documented in `docs/resources/manifest.md`.
> In this file, words like "manifest patch" or "manifest repo" refer to **GitOps-style YAML/source patching ideas**, not to the `Manifest` API resource itself.

If you need to answer build-side ownership questions such as which commit was built, which image was produced, or which Tekton run wrote back progress, use:

- `docs/resources/manifest.md`
- `docs/resources/release.md` for the deploy-side handoff

If you need this file, the question is usually narrower:

- what Tekton pipeline is currently committed
- what GitOps-style manifest-patch steps are only design references
- what future build -> patch -> notify chain is being discussed

## Current committed pipeline truth

The repo currently commits only the **image build / push** portion of the delivery chain.

Current committed task order:

1. `git-clone`
2. `image-build-and-push`

Authoritative committed files:

- `deployments/tekton/devflow-tekton-image-build-and-push.yaml`
- `deployments/tekton/devflow-tekton-image-build-push-only.yaml`

Important reader guardrail:

- this current committed state is a **build-side** Tekton pipeline truth
- it does **not** mean the `Manifest` API resource owns GitOps patching
- it does **not** mean a deploy-side bundle publication chain is already part of the active committed Tekton contract

The active build-side durable record remains the `Manifest` API resource described in `docs/resources/manifest.md`.
The active deploy-side durable record remains the `Release` API resource described in `docs/resources/release.md`.

## Operator guidance: where to inspect build-side versus deploy-side truth

Use the following routing to avoid the most common terminology confusion.

### Build-side truth

If the question is about build execution or image output, inspect the **build-side** `Manifest` surfaces:

- `docs/resources/manifest.md`
- `Manifest` API reads / writeback surfaces
- Tekton `PipelineRun` / `TaskRun` correlation through manifest `pipeline_id`, `trace_id`, and `steps`

Examples:

- which source revision was actually built
- which image ref or digest came out of build
- whether Tekton task progress wrote back correctly
- whether runtime-side Tekton observation updated manifest state

### Deploy-side truth

If the question is about rendered deployment output, OCI bundle publication, Argo CD handoff, or rollout progress, inspect the **deploy-side** `Release` surfaces:

- `docs/resources/release.md`
- release bundle publication / callback surfaces

Examples:

- what deployment bundle was rendered and published
- what artifact ref Argo CD should consume
- which rollout stage is active or failed

### Design-reference manifest-patch ideas

If the question is about a future **manifest-patch** step that edits a GitOps repo or deployment source tree, treat this file as a terminology/reference note only.

That work is about **deploy-side GitOps source mutation ideas**, not about the build-side `Manifest` API resource contract.

## Observed gaps in the active committed chain

For the fuller build -> patch -> notify story, the current committed Tekton pipeline still lacks these stages:

1. `manifest-patch`
   - meaning: patching image commit / digest / image-ref fields in a GitOps repo or other deploy-side source of truth
2. `notification`

These are documented here as **observed gaps**, not as proof that the cluster contract already includes them.

## Historical / design references only

The repo keeps local example YAML for a fuller delivery chain, but these examples are **design references only** until promoted into the active committed contract.

Reference files:

- `deployments/tekton/examples/devflow-manifest-delivery-tasks.yaml`
- `deployments/tekton/examples/devflow-manifest-delivery-pipeline.yaml`
- `deployments/tekton/examples/devflow-manifest-delivery-pipelinerun.example.yaml`

### Example manifest-patch shape

The staged `manifest-patch` example assumes a GitOps/deploy-side repo keeps image state under an `images:` list.

Example:

```yaml
images:
  - id: release-service
    image: registry.cn-hangzhou.aliyuncs.com/devflow/release-service:preproduction
    image_ref: registry.cn-hangzhou.aliyuncs.com/devflow/release-service:preproduction
    digest: sha256:...
    commit: abcdef123456
    source_commit: abcdef123456
```

In that design-reference example, the task would:

- clone the manifest repo
- patch the matching `images[].id`
- append a new item when the target image block does not exist
- commit and push the change back to the manifest repo

`image-id` is the selector for the image entry to patch. If empty, the task falls back to the service `name`.

Again, this is **GitOps manifest-repo patch behavior**, not the `Manifest` API resource contract.

## Recommended future chain

The recommended future chain is:

1. `git-clone`
2. `image-build-and-push`
3. `manifest-patch`
4. `notification`

Interpretation:

- steps 1-2 are the current build-side Tekton chain that the repo already commits
- step 3 is a **recommended future** GitOps / manifest-repo patch stage
- step 4 is a **recommended future** notification stage

This recommendation should not be read as evidence that:

- deploy-side bundle publication currently happens through the committed build pipeline
- the `Manifest` API resource owns deploy-side publication semantics
- the active `Release` contract should be debugged from this file instead of from `docs/resources/release.md`

## Logging recommendation

Tekton task logs should print enough detail for a later operator or agent to localize whether a failure is in build-side execution or in a future patch/notify extension.

Recommended log fields:

- resolved inputs
- resolved repo root / Dockerfile / manifest file
- final image ref
- final image digest
- manifest patch target file and resulting commit
- final notification summary

The currently committed build task has been locally improved to emit richer build-time logs, but patch/notify logging remains a recommendation until those stages become part of the active contract.

## Reader summary

Keep the terminology split explicit:

- **Manifest API resource** = build-side durable record
- **Release API resource** = deploy-side durable record and bundle publication / rollout owner
- **manifest-patch** in this file = GitOps/source-patch terminology used in historical or future delivery-chain discussion
- **current committed** Tekton truth = image build / push only
- **design references only** = local Tekton examples for a fuller future chain
