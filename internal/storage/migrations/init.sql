-- +goose Up
-- +goose StatementBegin
BEGIN;

CREATE SCHEMA IF NOT EXISTS content;

CREATE TABLE IF NOT EXISTS content.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    coins INTEGER NOT NULL DEFAULT 1000 CHECK (coins >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS content.merch (
    id SERIAL PRIMARY KEY,
    merch_name VARCHAR(100) NOT NULL UNIQUE,
    price INTEGER NOT NULL CHECK (price > 0)
);

CREATE TABLE IF NOT EXISTS content.merch_purchases (
    id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    merch_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_user_purchase FOREIGN KEY (user_id)
        REFERENCES content.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_merch_purchase FOREIGN KEY (merch_id)
        REFERENCES content.merch (id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS content.coin_transfers (
    id BIGSERIAL PRIMARY KEY,
    from_user_id INT NOT NULL,
    to_user_id INT NOT NULL,
    amount INTEGER NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_from_user FOREIGN KEY (from_user_id)
        REFERENCES content.users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_to_user FOREIGN KEY (to_user_id)
        REFERENCES content.users (id) ON DELETE RESTRICT,
    CONSTRAINT chk_different_users CHECK (from_user_id <> to_user_id)
);

CREATE INDEX IF NOT EXISTS idx_merch_purchases_user_id ON content.merch_purchases(user_id);
CREATE INDEX IF NOT EXISTS idx_coin_transfers_from_user_id ON content.coin_transfers(from_user_id);
CREATE INDEX IF NOT EXISTS idx_coin_transfers_to_user_id ON content.coin_transfers(to_user_id);

CREATE OR REPLACE FUNCTION content.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_updated_at ON content.users;
CREATE TRIGGER trg_update_updated_at
BEFORE UPDATE ON content.users
FOR EACH ROW
EXECUTE FUNCTION content.update_updated_at_column();

INSERT INTO content.merch (merch_name, price) VALUES
    ('t-shirt', 80),
    ('cup', 20),
    ('book', 50),
    ('pen', 10),
    ('powerbank', 200),
    ('hoody', 300),
    ('umbrella', 200),
    ('socks', 10),
    ('wallet', 50),
    ('pink-hoody', 500)
ON CONFLICT (merch_name) DO NOTHING;

COMMIT;
-- +goose StatementEnd

-- -- +goose Down
-- -- +goose StatementBegin
-- BEGIN;

-- DROP TRIGGER IF EXISTS trg_update_updated_at ON content.users;
-- DROP FUNCTION IF EXISTS content.update_updated_at_column();

-- DROP TABLE IF EXISTS content.coin_transfers;
-- DROP TABLE IF EXISTS content.merch_purchases;
-- DROP TABLE IF EXISTS content.merch;
-- DROP TABLE IF EXISTS content.users;

-- COMMIT;
-- -- +goose StatementEnd
