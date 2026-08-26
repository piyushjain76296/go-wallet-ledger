package ledger

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var (
	ErrImbalancedLedger = errors.New("ledger transaction is imbalanced (debits != credits)")
	ErrEmptyEntries     = errors.New("ledger transaction requires at least 2 entries")
)

type Repository interface {
	// CreateTransaction inserts a balanced transaction into the ledger.
	// It relies on being passed an active pgx.Tx to ensure it commits atomically
	// with the wallet/balance changes that happen in the Transfer engine.
	CreateTransaction(ctx context.Context, tx pgx.Tx, txn *Transaction) error
}

type postgresRepository struct{}

func NewRepository() Repository {
	return &postgresRepository{}
}

func (r *postgresRepository) CreateTransaction(ctx context.Context, tx pgx.Tx, txn *Transaction) error {
	if len(txn.Entries) < 2 {
		return ErrEmptyEntries
	}

	// 1. Verify Balance Invariant (Debits == Credits)
	var totalDebits, totalCredits int64
	currency := txn.Entries[0].Currency
	
	for _, entry := range txn.Entries {
		if entry.Currency != currency {
			return errors.New("multi-currency ledger transactions are not supported")
		}
		if entry.Amount <= 0 {
			return errors.New("entry amount must be positive")
		}

		if entry.Direction == DirectionDebit {
			totalDebits += entry.Amount
		} else if entry.Direction == DirectionCredit {
			totalCredits += entry.Amount
		} else {
			return errors.New("invalid entry direction")
		}
	}

	if totalDebits != totalCredits {
		return ErrImbalancedLedger
	}

	// 2. Insert Transaction
	err := tx.QueryRow(ctx, `
		INSERT INTO ledger_transactions (reference_id, reference_type, status)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, txn.ReferenceID, txn.ReferenceType, txn.Status).Scan(&txn.ID, &txn.CreatedAt)
	
	if err != nil {
		return err
	}

	// 3. Insert Entries
	// Using bulk insert for performance
	b := &pgx.Batch{}
	for i := range txn.Entries {
		txn.Entries[i].TransactionID = txn.ID // Link the entry to the transaction
		b.Queue(`
			INSERT INTO ledger_entries (transaction_id, account_id, amount, direction, currency)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at
		`, txn.ID, txn.Entries[i].AccountID, txn.Entries[i].Amount, txn.Entries[i].Direction, txn.Entries[i].Currency)
	}

	br := tx.SendBatch(ctx, b)
	defer br.Close()

	for i := range txn.Entries {
		err := br.QueryRow().Scan(&txn.Entries[i].ID, &txn.Entries[i].CreatedAt)
		if err != nil {
			return err
		}
	}

	return nil
}
