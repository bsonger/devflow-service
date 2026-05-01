# Workload-config pre-production verification

This directory contains the tracked operator proof surface for the shared-ingress workload-config API.

## Files

- `preprod_workload_config_flow.sh` — repeatable `GET -> PUT -> GET` probe against `/api/v1/config/workload-configs`

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

```sh
PREPROD_BASE_URL=https://<shared-ingress-host> \
PREPROD_AUTH_HEADER='Authorization: Bearer <token>' \
APPLICATION_ID=<application-uuid> \
WORKLOAD_CONFIG_ID=<workload-config-uuid> \
bash test/workloadconfig/preprod_workload_config_flow.sh
```

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

```sh
EXPECTED_OUTCOME=clean_round_trip \
PREPROD_BASE_URL=https://<shared-ingress-host> \
PREPROD_COOKIE_HEADER='Cookie: session=<masked>' \
APPLICATION_ID=<application-uuid> \
WORKLOAD_CONFIG_ID=<workload-config-uuid> \
bash test/workloadconfig/preprod_workload_config_flow.sh
```

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

## Grep anchors

This README intentionally includes the same contract vocabulary the task verifier checks:

- `workload-configs`
- `application_id`
- `size_class`
- `read-after-write`
- `failed_precondition`
- `invalid_argument`
