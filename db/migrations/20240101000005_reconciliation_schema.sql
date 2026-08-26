-- +goose Up
-- +goose StatementBegin
CREATE TABLE reconciliation_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(50) NOT NULL, -- IN_PROGRESS, PASSED, FAILED
    wallets_checked BIGINT NOT NULL DEFAULT 0,
    ledger_transactions_checked BIGINT NOT NULL DEFAULT 0,
    mismatches_found BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE reconciliation_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES reconciliation_runs(id),
    entity_type VARCHAR(50) NOT NULL, -- SYSTEM, WALLET
    entity_id VARCHAR(255) NOT NULL,  -- e.g. "GLOBAL" or wallet UUID
    mismatch_type VARCHAR(100) NOT NULL,
    details JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reconciliation_runs_status ON reconciliation_runs(status);
CREATE INDEX idx_reconciliation_items_run ON reconciliation_items(run_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE reconciliation_items;
DROP TABLE reconciliation_runs;
-- +goose StatementEnd
