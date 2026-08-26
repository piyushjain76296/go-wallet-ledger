-- +goose Up
-- +goose StatementBegin
CREATE TABLE ledger_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_id VARCHAR(255) NOT NULL,
    reference_type VARCHAR(50) NOT NULL, -- TRANSFER, PAYMENT, REFUND, ADJUSTMENT, REVERSAL, FEE
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for querying transaction history by reference
CREATE INDEX idx_ledger_transactions_reference ON ledger_transactions(reference_id, reference_type);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES ledger_transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount BIGINT NOT NULL,
    direction VARCHAR(10) NOT NULL, -- DEBIT, CREDIT
    currency VARCHAR(3) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT positive_amount CHECK (amount > 0)
);

-- Index for querying account ledger history quickly
CREATE INDEX idx_ledger_entries_account_id ON ledger_entries(account_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE ledger_entries;
DROP TABLE ledger_transactions;
-- +goose StatementEnd
