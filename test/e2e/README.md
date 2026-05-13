# End-to-End Tests

This directory contains repository-level release-flow verification that hits the
shared pre-production ingress instead of package-local mocks.

Current active script:

- `test/e2e/preprod_release_flow.sh`

It validates:

- release create through `release-service`
- release polling through the shared ingress
- strategy-specific step convergence for `rolling`, `blueGreen`, and `canary`

Execution rule:

- run flows serially for the same `application_id` / `environment_id`
- do not start `rolling` and `blueGreen` concurrently against the same live Argo
  `Application`, or the control plane may hit an expected resource-version
  conflict while updating the same `applications.argoproj.io/<name>`

`canary` is intentionally modeled as a stage-observation e2e, not a default
success-path e2e, because the rendered Argo Rollout includes pause stages and
needs explicit promotion semantics for a stable unattended success assertion.
The current control-plane behavior proves acceptance when:

- top-level release `status` is `Running`
- `deploy_canary` is `Succeeded`
- later canary promotion steps remain `Pending` or move to `Running`
- `finalize_release` remains `Pending`

Recommended canary usage:

```sh
PREPROD_BASE_URL=https://devflow-pre-production.bei.com \
MANIFEST_ID=<uuid> \
ENVIRONMENT_ID=<uuid> \
STRATEGY=canary \
EXPECTED_STATUS=Running \
bash test/e2e/preprod_release_flow.sh
```
