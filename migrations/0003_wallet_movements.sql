CREATE TABLE wallet_movements (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  wallet_id             UUID NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
  type                  TEXT NOT NULL,
  kind                  TEXT NOT NULL CHECK (kind IN ('real','promo')),
  amount_cents          BIGINT NOT NULL CHECK (amount_cents <> 0),
  reason                TEXT NOT NULL,
  performed_by_user_id  UUID REFERENCES users(id),
  performed_by_admin    BOOLEAN NOT NULL DEFAULT FALSE,
  performed_by_label    TEXT,
  idempotency_key       TEXT NOT NULL UNIQUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wallet_movements_wallet ON wallet_movements(wallet_id, created_at DESC);

CREATE OR REPLACE FUNCTION wallet_movements_immutable()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'wallet_movements is append-only';
END;
$$;

CREATE TRIGGER wallet_movements_no_update BEFORE UPDATE ON wallet_movements
  FOR EACH ROW EXECUTE FUNCTION wallet_movements_immutable();

CREATE TRIGGER wallet_movements_no_delete BEFORE DELETE ON wallet_movements
  FOR EACH ROW EXECUTE FUNCTION wallet_movements_immutable();
