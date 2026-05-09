# ERD — billing-service

```mermaid
erDiagram
    INVOICE ||--o{ INVOICE_LINE : contains
    INVOICE {
        uuid id PK
        uuid reservation_id UK
        uuid driver_id "logical FK to user_service"
        enum status "OPEN | CLOSED | PAID | VOID"
        bigint total_idr
        text idempotency_key UK
        timestamptz created_at
        timestamptz closed_at
        timestamptz paid_at
    }
    INVOICE_LINE {
        uuid id PK
        uuid invoice_id FK
        enum kind "BOOKING | HOURLY | OVERNIGHT | CANCELLATION | NOSHOW"
        bigint amount_idr
        jsonb metadata
        timestamptz created_at
    }
    OUTBOX_EVENT {
        bigint id PK
        text aggregate_type
        text aggregate_id
        text event_type
        jsonb payload
        timestamptz created_at
        timestamptz published_at
    }
```

## Why append-only line items

Audit trail. We never mutate or delete a line — corrections are new lines (e.g.
a refund line with negative amount). The invoice `total_idr` is denormalised
for query speed but always equals `SUM(invoice_line.amount_idr)`.
