# Workload-config pre-production verification

This directory contains the tracked operator proof surface for the shared-ingress workload-config API.

## Goal

Prove one real pre-production workload-config row can be:

1. listed by `application_id`
2. fetched by row `id`
3. updated with the constrained payload shape
4. re-read for read-after-write confirmation

The script also distinguishes the expected failure classes documented by the backend contract:

- `invalid_argument` for malformed constrained writes
- `failed_precondition` for explicit legacy-wide payload fields
- missing-row behavior after repository cleanup of incompatible stored data

## Safe usage rules

Do **not** commit live tokens, cookies, or raw operator-specific payloads.

Use environment variables only:

- `PREPROD_BASE_URL`
- `PREPROD_AUTH_HEADER` or `PREPROD_COOKIE_HEADER`
- `APPLICATION_ID`
- `WORKLOAD_CONFIG_ID`

The script writes captured status/body files under `test/workloadconfig/.tmp/` by default so evidence stays local and untracked.

## Required command

## Supported write fields in the probe payload

The probe intentionally sends only the constrained contract fields:

- `application_id`
- `replicas`
- `service_account_name`
- `resources.size_class`
- `probes.liveness.*`
- `probes.readiness.*`
- repeated `env` rows

That keeps the proof aligned with the current backend handler/service contract and avoids silently reintroducing legacy `resources.requests` / `resources.limits` writes.

## Expected outcomes

Set `EXPECTED_OUTCOME` when you want the script to enforce a specific result:

- `clean_round_trip` — update returns 2xx and read-after-write still returns the same row with the submitted `application_id` and `size_class`
- `invalid_argument` — update returns `400` with error code `invalid_argument`
- `failed_precondition` — update returns `412` with error code `failed_precondition`
- `missing_after_update` — read-after-write returns `404` or the row disappears from the application list, which localizes the issue to repository cleanup / missing-row behavior

Example:

## Read-after-write inspection points

The script always captures these files under `FLOW_DIR`:

- `list_before_update.body.json`
- `read_before_update.body.json`
- `update.payload.json`
- `update.body.json`
- `read_after_write.body.json`
- `list_after_update.body.json`
- matching `.status` files for each request

These make it easy to separate:

- auth or shared-ingress routing failures
- handler validation failures (`invalid_argument`, `failed_precondition`)
- repository cleanup or missing-row behavior after a legacy/incompatible payload

## Application detail page verification

Use the same `APPLICATION_ID` and `WORKLOAD_CONFIG_ID` from the API probe when checking the deployed application detail page workload tab.

Expected page checkpoints after a clean round trip:

1. the workload tab renders a structured summary instead of raw JSON helpers
2. the visible size-class card matches the backend canonical table:
   - `small` → requests `100m / 128Mi`, limits `500m / 512Mi`
   - `medium` → requests `250m / 256Mi`, limits `1 / 1Gi`
   - `large` → requests `500m / 512Mi`, limits `2 / 2Gi`
   - `xlarge` → requests `1 / 1Gi`, limits `4 / 4Gi`
3. the right-side drawer keeps the same constrained fields as the probe payload: replicas, service account name, `resources.size_class`, liveness/readiness probe rows, and repeated env rows
4. a save failure stays inline and leaves the drawer open for correction
5. a refresh shows the same summary values that the API probe captured in `read_after_write.body.json`

If the page summary disagrees with the API probe while the probe itself succeeds, localize the regression to frontend mapping/copy (`ApplicationDetailPage.tsx`) before suspecting ingress or backend drift.

## Grep anchors

This README intentionally includes the same contract vocabulary the task verifier checks:

- `workload-configs`
- `application_id`
- `size_class`
- `read-after-write`
- `failed_precondition`
- `invalid_argument`
