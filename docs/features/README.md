# Features — billing-service

| File                           | Status | Summary                                       |
|--------------------------------|--------|-----------------------------------------------|
| `01-open-invoice.md`           | ✅     | gRPC OpenInvoice — idempotent, booking fee    |
| `02-close-invoice.md`          | ✅     | gRPC CloseInvoice — applies pricing engine    |
| `03-pricing-engine.md`         | ✅     | Pure-functional rules + 13-case test suite    |
| `04-cancel-fee-consumer.md`    | ✅     | RabbitMQ consumer applies cancel/no-show fee  |
| `05-outbox-publisher.md`       | ✅     | At-least-once event delivery to RabbitMQ      |

Plus: `GetInvoice` gRPC handler.

Legend: 📋 planned · ⏳ in progress · ✅ shipped · 🚫 deferred
