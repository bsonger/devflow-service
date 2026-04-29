# API

This directory is reserved for stable service contracts in `devflow-service`.

Use it for externally consumed schemas only, such as:
- OpenAPI documents
- protobuf definitions
- other stable cross-process contracts

Do not move service-private application code here.

For the current OpenAPI generation contract and backend-local route caveats, read:

- `api/openapi/README.md`
