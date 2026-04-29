# gateway

This directory is reserved for backend edge contracts in `devflow-service`.

The current committed pre-production edge contract is Istio-oriented rather than Kong-oriented.
Use it as the stable home for declarative gateway configuration, shared ingress routing, and related support assets without mixing that concern into owner-service modules.

Current repo-local edge artifact:
- `deployments/pre-production/istio/shared-ingress.yaml`

The owning routing explanation is:
- `docs/system/ingress-routing.md`

That manifest currently exposes one shared host, `devflow-pre-production.bei.com`, and routes shared API prefixes through Istio:
- `/api/v1/meta/...` -> `meta-service`
- `/api/v1/config/...` -> `config-service`
- `/api/v1/network/...` -> `network-service`
- `/api/v1/runtime/...` -> `runtime-service`
- `/api/v1/release/...` -> `release-service`

It also keeps one legacy compatibility prefix:
- `/api/v1/platform/...` -> `meta-service`

Important routing detail:
- the shared ingress prefixes above are edge-facing paths
- most backend services still register service-internal routes under `/api/v1/...`
- Istio rewrite behavior in `deployments/pre-production/istio/shared-ingress.yaml` is what maps shared prefixes such as `/api/v1/meta/...` to backend-local `/api/v1/...` routes where applicable

## Boundary rule

`gateway/` is an edge contract surface, not a business-logic owner.
Do not use it to hide application, release, runtime, config, or network ownership semantics.
