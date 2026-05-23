# Staging Deploy

## Purpose

This guide records the repo-local defaults for deploying the 5 backend services to the staging environment through the DevFlow platform.

## Platform deploy endpoints

- platform UI: `https://devflow.bei.com/platform`
- platform API root: `https://devflow.bei.com/api/v1/`

## Staging runtime endpoints

- staging UI: `https://devflow-staging.bei.com/platform`
- staging API root: `https://devflow-staging.bei.com/v1/api`

## Canonical staging identifiers

- project id: `8bbdd172-a11d-435f-9ec0-2fc53caaacfb`
- environment id: `ce3e0499-e862-4322-98e2-264fa6f09286`

## Default staging service set

- `meta-service`
- `config-service`
- `network-service`
- `release-service`
- `runtime-service`

## Deployment contract

When deploying staging through the platform helper scripts:

- use `PLATFORM_BASE_URL=https://devflow.bei.com`
- use the staging `PROJECT_ID` and `ENVIRONMENT_ID` above
- deploy the 5 backend services as one set unless the operator explicitly narrows `SERVICE_NAMES`
- prefer the repo-local platform deploy helper scripts instead of direct Kubernetes YAML application

When validating the deployed staging environment in a browser or by direct API calls:

- open `https://devflow-staging.bei.com/platform`
- use `https://devflow-staging.bei.com/v1/api`
