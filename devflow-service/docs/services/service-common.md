# Service Common

This file is migrated from `devflow-control` as a cross-repo reference.
It is ownership context only.

## Owns

- shared bootstrap
- shared middleware
- shared observability primitives
- shared HTTP helpers
- shared request/response/error envelope helpers

## Does Not Own

- domain entities
- business APIs
- control-plane state

## Downstream Consumers

- all backend DevFlow services
