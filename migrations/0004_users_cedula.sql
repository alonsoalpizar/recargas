-- Refactor: cedula-first registration with GoMeta validation.
-- Wipe-and-recreate authorized by user (datos eran de prueba).

DROP TRIGGER IF EXISTS wallet_movements_no_update ON wallet_movements;
DROP TRIGGER IF EXISTS wallet_movements_no_delete ON wallet_movements;
DROP FUNCTION IF EXISTS wallet_movements_immutable();

DROP TABLE IF EXISTS wallet_movements CASCADE;
DROP TABLE IF EXISTS wallets CASCADE;
DROP TABLE IF EXISTS users CASCADE;

CREATE TABLE users (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cedula            TEXT NOT NULL UNIQUE,
  cedula_type       TEXT NOT NULL CHECK (cedula_type IN ('fisica','juridica','dimex')),
  nombre            TEXT,
  apellido          TEXT,
  nombre_completo   TEXT NOT NULL,
  email             TEXT NOT NULL UNIQUE,
  telefono          TEXT NOT NULL UNIQUE,
  password_hash     TEXT NOT NULL,
  is_admin          BOOLEAN NOT NULL DEFAULT FALSE,
  direccion         TEXT,
  provincia         TEXT,
  canton            TEXT,
  distrito          TEXT,
  fecha_nacimiento  DATE,
  metadata          JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE wallets (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
  frozen     BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

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
