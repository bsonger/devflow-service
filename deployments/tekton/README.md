# Tekton

This directory holds the committed Tekton surfaces for `devflow-service`.

## Current committed flow

Today the active committed pipeline is:

1. `git-clone`
2. `image-build-and-push`

Current files:

- task: `deployments/tekton/devflow-tekton-image-build-and-push.yaml`
- pipeline: `deployments/tekton/devflow-tekton-image-build-push-only.yaml`
- service-specific `PipelineRun` templates and pre-production runs under the same directory

## Staged manifest-delivery contract

The repo now also carries a staged formal contract for the full manifest-oriented flow:

- `deployments/tekton/devflow-tekton-manifest-delivery-staged.yaml`
- `deployments/tekton/manifest-delivery-pipeline-run-template.yaml`

This staged contract is committed for review and iteration, but it is **not yet** the active cluster-default pipeline.

## Current gap summary

The current committed pipeline does **not yet** include these later-stage steps:

1. `manifest-patch`
   - update manifest-side image digest / commit / image reference fields
2. `notification`
   - publish final delivery result after build + patch

That means the current committed flow is still only the image build/push half, not the full manifest-delivery chain.

## Recommended target flow

For manifest-oriented delivery, the recommended task order is:

1. `git-clone`
2. `image-build-and-push`
3. `manifest-patch`
4. `notification`

Recommended data handoff between tasks:

- `git-clone`
  - `git-commit`
  - `git-branch`
  - `git-tag`
- `image-build-and-push`
  - `image-ref`
  - `image-tag`
  - `image-digest`
- `manifest-patch`
  - `patched-commit`
  - `patched-path`
- `notification`
  - no hard output required; should emit complete terminal summary logs

## Manifest patch contract

The staged `manifest-patch` task is now meant to be usable, not placeholder-only.

Expected manifest-side YAML shape:

```yaml
images:
  - id: release-service
    image: registry.cn-hangzhou.aliyuncs.com/devflow/release-service:preproduction
    image_ref: registry.cn-hangzhou.aliyuncs.com/devflow/release-service:preproduction
    digest: sha256:...
    commit: abcdef123456
    source_commit: abcdef123456
```

Task behavior:

- clone the manifest repo
- locate `manifest-file`
- use `image-id` to select one item under `images[]`
- if the item does not exist, append it
- patch:
  - `image`
  - `image_ref`
  - `digest`
  - `commit`
  - `source_commit`
- commit and push back to `manifest-git-revision`

## Local example bundle

Example future-state YAMLs are saved here:

- `deployments/tekton/examples/devflow-manifest-delivery-tasks.yaml`
- `deployments/tekton/examples/devflow-manifest-delivery-pipeline.yaml`
- `deployments/tekton/examples/devflow-manifest-delivery-pipelinerun.example.yaml`

These examples are for local design capture and are **not** yet the repo’s active deployment contract.
