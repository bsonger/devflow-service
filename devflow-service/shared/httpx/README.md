# shared/httpx

Package `httpx` holds infrastructure-only HTTP response and pagination helpers extracted into the root `devflow-service` module.
These helpers are safe for reuse across future owner modules because they shape transport behavior without owning domain logic.
