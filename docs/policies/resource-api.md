# Resource API Policy

## Reader and outcome

This policy is for engineers and agents changing resource-oriented HTTP behavior or `docs/resources/*.md` contracts in `devflow-service`.

After reading it, a fresh reader should be able to:
- add or change a resource endpoint without inventing one-off list or delete behavior
- keep pagination, filtering, and soft-delete semantics consistent across resources
- know what every resource doc must explain for a cold reader
- keep handlers, resource docs, and verification aligned when public behavior changes

## Scope

This policy governs:

- resource-oriented HTTP behavior under `internal/*/transport/http`
- resource contract docs under `docs/resources/*.md`

It complements:

- `docs/policies/http-handler.md`
- `docs/policies/error-handling.md`
- `docs/policies/api-compatibility.md`

## Core rules

1. Resource endpoints should follow one predictable CRUD shape unless there is a clear domain reason not to.
2. List endpoints should use the shared pagination contract.
3. Soft delete behavior must be explicit in both code and docs.
4. Public resource docs must describe current behavior, not aspirational future behavior.
5. Resource docs, handlers, and validation behavior must change together.
6. Public API field names, request-body keys, and query parameters must use `snake_case`.
7. `GET` endpoints may use query filters; `POST` and `DELETE` endpoints must carry business selectors in the JSON body instead of query strings.

## Resource endpoint shape

Preferred resource surface:

- `POST /api/v1/<resources>` for create
- `GET /api/v1/<resources>` for list
- `GET /api/v1/<resources>/{id}` for get
- `PUT` or `PATCH /api/v1/<resources>/{id}` for update
- `DELETE /api/v1/<resources>/{id}` for soft delete when deletion is supported

Default rule: do not add `/applications/{application_id}` to resource paths.

Prefer flat resources plus explicit `application_id` in request body or query filters, such as:

- `POST /api/v1/services` with `application_id` in body
- `GET /api/v1/services?application_id=...`
- `POST /api/v1/routes` with `application_id` in body
- `GET /api/v1/routes?application_id=...&environment_id=...`

Only use parent-bound paths when a resource is intentionally path-scoped and the exception is explicitly documented in both the handler contract and `docs/resources/*.md`.

Preferred API naming examples:

- `application_id`
- `environment_id`
- `cluster_id`
- `repo_address`
- `page_size`
- `include_deleted`


## Selector location rules

Use query parameters for read filters on `GET` only.

Preferred pattern:

- `GET /api/v1/<resources>?application_id=...&environment_id=...` for reads and list filters
- `POST /api/v1/<resources>` with `application_id`, `environment_id`, and similar business selectors in the JSON body
- `DELETE /api/v1/<resources>/{id}` with any required business selector fields in the JSON body

Do not put business selectors such as these on `POST` or `DELETE` query strings:

- `application_id`
- `environment_id`
- `project_id`
- `cluster_id`
- `manifest_id`
- `release_id`

Reason:

- read filtering and write targeting stay visually distinct
- handler behavior stays more predictable across the repo
- write-side contracts remain explicit in resource docs and OpenAPI surfaces

Current repo-level convention:

- `GET` uses query filters
- `POST` uses request body
- `DELETE` uses request body when additional business selectors are required beyond the path id

## List endpoint rules

When a resource exposes a list endpoint, it should:

1. parse filters at the handler edge
2. support `page` and `page_size` through `httpx.ParsePagination`
3. support `include_deleted` through `httpx.IncludeDeleted` when the resource is soft-deletable
4. paginate the returned slice with `httpx.PaginateSlice`
5. return the list envelope through `httpx.WriteList`

Do not invent custom pagination field names such as `limit`, `offset`, or `per_page` on one resource while the rest of the repo uses `page` and `page_size`.

## Filter rules

Filters should stay explicit and low-surprise.

Prefer query filters such as:

- foreign-key filters like `project_id`, `application_id`, `cluster_id`
- stable enum or status filters like `status`, `type`, `kind`
- small string filters like `name`
- `include_deleted` for soft-delete visibility

Avoid query parameters that expose internal implementation detail or highly user-specific identity.

## Soft delete rules

If a resource supports soft delete:

- `DELETE` should mark the record deleted instead of hard-deleting it
- normal `GET` and list behavior should exclude deleted records by default
- `include_deleted=true` should opt in to deleted rows only where the resource already supports that behavior
- docs must say the resource is soft-deleted and whether list endpoints support `include_deleted`

## Error rules

Resource handlers should stay aligned with the standard error vocabulary.

Prefer:

- `invalid_argument` for malformed ids, filters, or pagination
- `not_found` for missing resources
- `conflict` for uniqueness collisions
- `failed_precondition` for dependency-readiness blockers

Public resource docs should describe these behaviors briefly in `Validation notes` instead of copying every internal error path.

## Resource doc template rules

Each file under `docs/resources/*.md` should explain, in this order when practical:

1. ownership
2. purpose
3. common base fields
4. resource field table
5. API surface
6. create or update rules
7. validation notes
8. source pointers

If the resource has special behavior, also document the smallest additional section needed, such as:

- status values
- writeback routes
- validation endpoints
- explicitly-declared path-scoped exceptions

## Resource doc behavior rules

Resource docs should explicitly answer:

- is the resource createable, listable, gettable, updateable, deletable
- whether deletion is soft delete
- which list filters are supported today
- whether `include_deleted` is supported
- what top-level status or type enums are relevant
- which fields are user-writable versus system-managed

Do not describe filters or routes that the current handlers do not actually support.

## Recommended checklist

When changing a resource API, check:

1. did I keep list pagination on `page` and `page_size`
2. did I keep soft-delete behavior explicit and documented
3. did I reuse `httpx.WriteList` and shared pagination helpers
4. are supported filters documented exactly as implemented today
5. did I update `docs/resources/*.md` in the same change cycle
6. did I keep error behavior aligned with the shared error vocabulary

## Verification expectations

When changing resource-facing behavior:

- keep resource docs aligned with current handler behavior
- keep list, filter, and delete semantics consistent across resource endpoints
- keep docs, handlers, and verification aligned in the same change cycle
