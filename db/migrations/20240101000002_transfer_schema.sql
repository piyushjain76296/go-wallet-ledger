-- +goose Up
-- +goose StatementBegin
CREATE TABLE idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    status VARCHAR(50) NOT NULL, -- IN_PROGRESS, COMPLETED
    request_payload JSONB NOT NULL,
    response_payload JSONB,
    response_status INT,
    locked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_wallet_id UUID NOT NULL REFERENCES wallets(id),
    destination_wallet_id UUID NOT NULL REFERENCES wallets(id),
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(50) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL REFERENCES idempotency_keys(key),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT positive_transfer_amount CHECK (amount > 0),
    CONSTRAINT different_wallets CHECK (source_wallet_id != destination_wallet_id)
);

CREATE INDEX idx_transfers_source ON transfers(source_wallet_id);
CREATE INDEX idx_transfers_destination ON transfers(destination_wallet_id);
CREATE INDEX idx_transfers_idempotency ON transfers(idempotency_key);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transfers;
DROP TABLE idempotency_keys;
-- +goose StatementEnd
