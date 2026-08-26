package wallets

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrWalletNotFound = errors.New("wallet not found")
)

type Repository interface {
	CreateWalletAndAccount(ctx context.Context, ownerID string, accountType string, currency string) (*Wallet, error)
	GetWalletByID(ctx context.Context, id string) (*Wallet, error)
	UpdateWalletStatus(ctx context.Context, id string, status WalletStatus) error
	GetTransactions(ctx context.Context, accountID string) ([]Transaction, error)
	GetWalletsByOwner(ctx context.Context, ownerID string) ([]Wallet, error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// CreateWalletAndAccount creates both the ledger account and the wallet atomically.
func (r *postgresRepository) CreateWalletAndAccount(ctx context.Context, ownerID string, accountType string, currency string) (*Wallet, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var accountID string
	err = tx.QueryRow(ctx, `
		INSERT INTO accounts (type, currency)
		VALUES ($1, $2)
		RETURNING id
	`, accountType, currency).Scan(&accountID)
	if err != nil {
		return nil, err
	}

	wallet := &Wallet{}
	err = tx.QueryRow(ctx, `
		INSERT INTO wallets (owner_id, account_id, currency, available_balance, locked_balance, status, version)
		VALUES ($1, $2, $3, 0, 0, $4, 1)
		RETURNING id, owner_id, account_id, currency, available_balance, locked_balance, status, version, created_at, updated_at
	`, ownerID, accountID, currency, StatusActive).Scan(
		&wallet.ID, &wallet.OwnerID, &wallet.AccountID, &wallet.Currency,
		&wallet.AvailableBalance, &wallet.LockedBalance, &wallet.Status,
		&wallet.Version, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return wallet, nil
}

func (r *postgresRepository) GetWalletByID(ctx context.Context, id string) (*Wallet, error) {
	query := `
		SELECT id, owner_id, account_id, currency, available_balance, locked_balance, status, version, created_at, updated_at
		FROM wallets
		WHERE id = $1
	`
	wallet := &Wallet{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&wallet.ID, &wallet.OwnerID, &wallet.AccountID, &wallet.Currency,
		&wallet.AvailableBalance, &wallet.LockedBalance, &wallet.Status,
		&wallet.Version, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}

	return wallet, nil
}

func (r *postgresRepository) UpdateWalletStatus(ctx context.Context, id string, status WalletStatus) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE wallets
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, status, id)

	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrWalletNotFound
	}

	return nil
}

func (r *postgresRepository) GetTransactions(ctx context.Context, accountID string) ([]Transaction, error) {
	query := `
		SELECT 
			e.id, 
			t.reference_id, 
			t.reference_type, 
			e.amount, 
			e.direction, 
			e.currency, 
			t.status, 
			e.created_at
		FROM ledger_entries e
		JOIN ledger_transactions t ON e.transaction_id = t.id
		WHERE e.account_id = $1
		ORDER BY e.created_at DESC
		LIMIT 100
	`
	
	rows, err := r.db.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var txn Transaction
		if err := rows.Scan(
			&txn.ID, &txn.ReferenceID, &txn.ReferenceType,
			&txn.Amount, &txn.Direction, &txn.Currency,
			&txn.Status, &txn.CreatedAt,
		); err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if transactions == nil {
		transactions = []Transaction{}
	}

	return transactions, nil
}

func (r *postgresRepository) GetWalletsByOwner(ctx context.Context, ownerID string) ([]Wallet, error) {
	query := `
		SELECT id, owner_id, account_id, currency, available_balance, locked_balance, status, version, created_at, updated_at
		FROM wallets
		WHERE owner_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []Wallet
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(
			&w.ID, &w.OwnerID, &w.AccountID, &w.Currency,
			&w.AvailableBalance, &w.LockedBalance, &w.Status,
			&w.Version, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if wallets == nil {
		wallets = []Wallet{}
	}

	return wallets, nil
}
