-- 001_init: invoice + invoice_line + outbox_event + idempotency_key

BEGIN;

DO $$ BEGIN
  CREATE TYPE invoice_status AS ENUM ('OPEN','CLOSED','PAID','VOID');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE line_kind AS ENUM ('BOOKING','HOURLY','OVERNIGHT','CANCELLATION','NOSHOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS invoice (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reservation_id  uuid UNIQUE NOT NULL,
  driver_id       text NOT NULL,
  status          invoice_status NOT NULL DEFAULT 'OPEN',
  total_idr       bigint NOT NULL DEFAULT 0,
  idempotency_key text UNIQUE,
  created_at      timestamptz NOT NULL DEFAULT now(),
  closed_at       timestamptz,
  paid_at         timestamptz
);

CREATE TABLE IF NOT EXISTS invoice_line (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id  uuid NOT NULL REFERENCES invoice(id) ON DELETE CASCADE,
  kind        line_kind NOT NULL,
  amount_idr  bigint NOT NULL,
  metadata    jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_line_invoice ON invoice_line(invoice_id);
-- For one-off fee lines (CANCELLATION, NOSHOW) we want at most one per invoice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_line_one_off
  ON invoice_line(invoice_id, kind)
  WHERE kind IN ('CANCELLATION','NOSHOW');

CREATE TABLE IF NOT EXISTS outbox_event (
  id              bigserial PRIMARY KEY,
  aggregate_type  text NOT NULL,
  aggregate_id    text NOT NULL,
  event_type      text NOT NULL,
  payload         jsonb NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  published_at    timestamptz
);
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_event(created_at) WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS idempotency_key (
  scope             text NOT NULL,
  key               text NOT NULL,
  response_payload  bytea,
  status_code       int,
  created_at        timestamptz NOT NULL DEFAULT now(),
  expires_at        timestamptz NOT NULL,
  PRIMARY KEY (scope, key)
);
CREATE INDEX IF NOT EXISTS idx_idem_expires ON idempotency_key(expires_at);

COMMIT;
