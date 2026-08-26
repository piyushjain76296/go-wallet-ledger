package reconciliation

import (
	"encoding/json"
	"time"
)

type RunStatus string

const (
	StatusInProgress RunStatus = "IN_PROGRESS"
	StatusPassed     RunStatus = "PASSED"
	StatusFailed     RunStatus = "FAILED"
)

type Run struct {
	ID                       string     `json:"id"`
	Status                   RunStatus  `json:"status"`
	WalletsChecked           int64      `json:"wallets_checked"`
	LedgerTransactionsChecked int64      `json:"ledger_transactions_checked"`
	MismatchesFound          int64      `json:"mismatches_found"`
	StartedAt                time.Time  `json:"started_at"`
	CompletedAt              *time.Time `json:"completed_at"`
}

type Item struct {
	ID           string          `json:"id"`
	RunID        string          `json:"run_id"`
	EntityType   string          `json:"entity_type"`
	EntityID     string          `json:"entity_id"`
	MismatchType string          `json:"mismatch_type"`
	Details      json.RawMessage `json:"details"`
	CreatedAt    time.Time       `json:"created_at"`
}
