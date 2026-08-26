package reconciliation

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateRun(ctx context.Context) (*Run, error)
	CompleteRun(ctx context.Context, id string, status RunStatus, walletsChecked, txnsChecked, mismatches int64) error
	RecordMismatch(ctx context.Context, runID, entityType, entityID, mismatchType string, details interface{}) error
	
	// Aggregations
	CheckGlobalLedgerBalance(ctx context.Context) (totalDebits, totalCredits int64, txnsCount int64, err error)
	CheckWalletBalances(ctx context.Context) (walletsChecked int64, mismatches []map[string]interface{}, err error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateRun(ctx context.Context) (*Run, error) {
	run := &Run{Status: StatusInProgress}
	err := r.db.QueryRow(ctx, `
		INSERT INTO reconciliation_runs (status)
		VALUES ($1)
		RETURNING id, started_at
	`, run.Status).Scan(&run.ID, &run.StartedAt)
	return run, err
}

func (r *postgresRepository) CompleteRun(ctx context.Context, id string, status RunStatus, walletsChecked, txnsChecked, mismatches int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE reconciliation_runs
		SET status = $1, wallets_checked = $2, ledger_transactions_checked = $3, mismatches_found = $4, completed_at = NOW()
		WHERE id = $5
	`, status, walletsChecked, txnsChecked, mismatches, id)
	return err
}

func (r *postgresRepository) RecordMismatch(ctx context.Context, runID, entityType, entityID, mismatchType string, details interface{}) error {
	detailsBytes, _ := json.Marshal(details)
	_, err := r.db.Exec(ctx, `
		INSERT INTO reconciliation_items (run_id, entity_type, entity_id, mismatch_type, details)
		VALUES ($1, $2, $3, $4, $5)
	`, runID, entityType, entityID, mismatchType, detailsBytes)
	return err
}

func (r *postgresRepository) CheckGlobalLedgerBalance(ctx context.Context) (int64, int64, int64, error) {
	var debits, credits, txnCount int64
	err := r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) as total_debits,
			COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) as total_credits,
			COUNT(DISTINCT transaction_id) as txn_count
		FROM ledger_entries
	`).Scan(&debits, &credits, &txnCount)
	return debits, credits, txnCount, err
}

func (r *postgresRepository) CheckWalletBalances(ctx context.Context) (int64, []map[string]interface{}, error) {
	// Compare Wallet (Available + Locked) to Ledger net sum for that account
	// Note: Platform accounts won't be caught here unless they have a linked wallet
	query := `
		WITH ledger_totals AS (
			SELECT 
				account_id,
				SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END) -
				SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END) AS net_ledger_balance
			FROM ledger_entries
			GROUP BY account_id
		)
		SELECT 
			w.id as wallet_id,
			w.available_balance + w.locked_balance as wallet_total,
			COALESCE(lt.net_ledger_balance, 0) as ledger_total
		FROM wallets w
		LEFT JOIN ledger_totals lt ON w.account_id = lt.account_id
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var walletsChecked int64
	var mismatches []map[string]interface{}

	for rows.Next() {
		walletsChecked++
		var walletID string
		var walletTotal, ledgerTotal int64
		
		if err := rows.Scan(&walletID, &walletTotal, &ledgerTotal); err != nil {
			return 0, nil, err
		}

		if walletTotal != ledgerTotal {
			mismatches = append(mismatches, map[string]interface{}{
				"wallet_id":    walletID,
				"wallet_total": walletTotal,
				"ledger_total": ledgerTotal,
				"difference":   walletTotal - ledgerTotal,
			})
		}
	}

	return walletsChecked, mismatches, nil
}
