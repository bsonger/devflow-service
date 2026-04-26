# Tekton Delivery

## Purpose

This file is the current local note for the Tekton delivery chain around image build, scan, manifest patch, and notification.

## Current committed state

The repo currently commits only the image build/push pipeline:

1. `git-clone`
2. `image-build-and-push`

Authoritative files:

- `deployments/tekton/devflow-tekton-image-build-and-push.yaml`
- `deployments/tekton/devflow-tekton-image-build-push-only.yaml`

## Observed gap

For the full manifest-delivery path, the current committed pipeline still lacks:

1. `manifest-patch`
   - specifically patching image commit / digest / image-ref fields in the manifest-side repo or manifest-side source of truth
2. `notification`

## Recommended future chain

Recommended order:

1. `git-clone`
2. `image-build-and-push`
3. `manifest-patch`
4. `notification`

## Logging recommendation

Tekton task logs should always print at least:

- resolved inputs
- resolved repo root / Dockerfile / manifest file
- final image ref
- final image digest
- manifest patch target file and resulting commit
- final notification summary

The committed build task has now been locally improved to emit richer build-time logs.

## Current staged patch behavior

The staged `manifest-patch` contract now assumes the manifest repo keeps image state under an `images:` list.

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

The task now:

- clones the manifest repo
- patches the matching `images[].id`
- appends a new item when the target image block does not exist
- commits and pushes the change back to the manifest repo

`image-id` is the selector for the image entry to patch. If empty, the task falls back to the service `name`.

## Example YAML location

Saved local examples:

- `deployments/tekton/examples/devflow-manifest-delivery-tasks.yaml`
- `deployments/tekton/examples/devflow-manifest-delivery-pipeline.yaml`
- `deployments/tekton/examples/devflow-manifest-delivery-pipelinerun.example.yaml`

These examples are design references only until promoted into the active cluster contract.
