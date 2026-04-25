# gateway

This directory is reserved for backend edge contracts in `devflow-service`.

The current committed pre-production edge contract is Istio-oriented rather than Kong-oriented.
Use it as the stable home for declarative gateway configuration, shared ingress routing, and related support assets without mixing that concern into owner-service modules.

Current repo-local edge artifact:
- `deployments/pre-production/istio/shared-ingress.yaml`

That manifest currently exposes one shared host, `devflow-pre.example.com`, and routes per-service subpaths through Istio:
- `/config` -> `config-service`
- `/network` -> `network-service`
- `/runtime` -> `runtime-service`

## Boundary rule

`gateway/` is an edge contract surface, not a business-logic owner.
Do not use it to hide application, release, runtime, config, or network ownership semantics.
