# billing-service

[![Security](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Reliability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Maintainability](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Duplications](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=duplicated_lines_density)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=pintarparkir_billing-service&metric=coverage)](https://sonarcloud.io/summary/new_code?id=pintarparkir_billing-service)

> **Purpose:** Invoice lifecycle management — owns invoice ledger, pricing engine, cancel/no-show fees, and reconciliation.
> **Author:** Farid Triwicaksono

## Architecture Overview

![Architecture](docs/PintarParkir.architecture.svg)

## E2E Flow

![Flow Diagram](docs/flow.diagram.svg)

## Sequence Diagrams

- [Billing & Checkout Flow](docs/sequence-diagrams/03-billing-checkout-flow.md)

## Tech Stack

- Go 1.25 + Gin (HTTP) + gRPC
- PostgreSQL (pgcrypto for PII encryption)
- Redis (caching + distributed locks)
- RabbitMQ (async event-driven via outbox pattern)
- Cloud Run (GCP) with auto-scaling
- OpenTelemetry (traces + metrics)

**Service-specific:** Pricing engine (booking/hourly/overnight/cancel/no-show fees), invoice lifecycle, gRPC server (h2c)

## API

See [OpenAPI Specification](docs/api-specifications/openapi-spec.yaml) and [AsyncAPI Specification](docs/api-specifications/asyncapi-spec.yaml).

## Running Locally

```bash
cp configs/.env.example configs/.env
make run
```

## Testing

```bash
make test          # unit tests
make test-coverage # with coverage report
```

## Deployment

CD via GitHub Actions → GCP Cloud Run (asia-southeast1).
Triggers on push to `main`.

Cloud Run URL: `https://billing-service-725nddkmwq-as.a.run.app`
