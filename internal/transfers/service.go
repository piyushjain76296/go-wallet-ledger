package transfers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wallet-ledger/internal/ledger"
	"github.com/wallet-ledger/internal/observability"
	"github.com/wallet-ledger/internal/outbox"
	"github.com/wallet-ledger/internal/wallets"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrSameWallet        = errors.New("source and destination wallets must be different")
	ErrCurrencyMismatch  = errors.New("currency mismatch between wallets")
)

type Service interface {
	ExecuteTransfer(ctx context.Context, req CreateTransferRequest, idempotencyKey string) (*Transfer, error)
}

type serviceImpl struct {
	db         *pgxpool.Pool
	ledgerRepo ledger.Repository
	walletRepo wallets.Repository
	outboxRepo outbox.Repository
}

func NewService(db *pgxpool.Pool, ledgerRepo ledger.Repository, walletRepo wallets.Repository, outboxRepo outbox.Repository) Service {
	return &serviceImpl{
		db:         db,
		ledgerRepo: ledgerRepo,
		walletRepo: walletRepo,
		outboxRepo: outboxRepo,
	}
}

func (s *serviceImpl) ExecuteTransfer(ctx context.Context, req CreateTransferRequest, idempotencyKey string) (*Transfer, error) {
	// Defer recording the metric. We will assume failure unless we reach the success path.
	success := false
	defer func() {
		observability.RecordTransfer(success)
	}()

	if req.SourceWalletID == req.DestinationWalletID {
		return nil, ErrSameWallet
	}
	if req.Amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	// Begin Postgres Transaction
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Deadlock Prevention: Order wallet IDs to ensure consistent lock acquisition
	firstLock, secondLock := req.SourceWalletID, req.DestinationWalletID
	if firstLock > secondLock {
		firstLock, secondLock = secondLock, firstLock
	}

	// 2. Acquire Locks (SELECT FOR UPDATE)
	var firstWallet, secondWallet wallets.Wallet
	err = tx.QueryRow(ctx, `
		SELECT id, owner_id, account_id, currency, available_balance, status 
		FROM wallets WHERE id = $1 FOR UPDATE
	`, firstLock).Scan(&firstWallet.ID, &firstWallet.OwnerID, &firstWallet.AccountID, &firstWallet.Currency, &firstWallet.AvailableBalance, &firstWallet.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to lock wallet %s: %w", firstLock, err)
	}

	err = tx.QueryRow(ctx, `
		SELECT id, owner_id, account_id, currency, available_balance, status 
		FROM wallets WHERE id = $1 FOR UPDATE
	`, secondLock).Scan(&secondWallet.ID, &secondWallet.OwnerID, &secondWallet.AccountID, &secondWallet.Currency, &secondWallet.AvailableBalance, &secondWallet.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to lock wallet %s: %w", secondLock, err)
	}

	// Map them back to source/dest for business logic
	var sourceWallet, destWallet *wallets.Wallet
	if firstWallet.ID == req.SourceWalletID {
		sourceWallet = &firstWallet
		destWallet = &secondWallet
	} else {
		sourceWallet = &secondWallet
		destWallet = &firstWallet
	}

	// 3. Validation
	if sourceWallet.Status != wallets.StatusActive || destWallet.Status != wallets.StatusActive {
		return nil, errors.New("one or both wallets are not active")
	}
	if sourceWallet.Currency != req.Currency || destWallet.Currency != req.Currency {
		return nil, ErrCurrencyMismatch
	}
	if sourceWallet.AvailableBalance < req.Amount {
		return nil, ErrInsufficientFunds
	}

	// 4. Update Balances
	_, err = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance - $1 WHERE id = $2`, req.Amount, sourceWallet.ID)
	if err != nil {
		return nil, err
	}
	
	_, err = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $1 WHERE id = $2`, req.Amount, destWallet.ID)
	if err != nil {
		return nil, err
	}

	// 5. Create Transfer Record
	transfer := &Transfer{
		SourceWalletID:      req.SourceWalletID,
		DestinationWalletID: req.DestinationWalletID,
		Amount:              req.Amount,
		Currency:            req.Currency,
		Status:              StatusCompleted,
		IdempotencyKey:      idempotencyKey,
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO transfers (source_wallet_id, destination_wallet_id, amount, currency, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, transfer.SourceWalletID, transfer.DestinationWalletID, transfer.Amount, transfer.Currency, transfer.Status, transfer.IdempotencyKey).
		Scan(&transfer.ID, &transfer.CreatedAt)
	if err != nil {
		return nil, err
	}

	// 6. Create Ledger Transaction
	ledgerTxn := &ledger.Transaction{
		ReferenceID:   transfer.ID,
		ReferenceType: ledger.TxTypeTransfer,
		Status:        ledger.StatusCompleted,
		Entries: []ledger.Entry{
			{AccountID: sourceWallet.AccountID, Amount: req.Amount, Direction: ledger.DirectionDebit, Currency: req.Currency},
			{AccountID: destWallet.AccountID, Amount: req.Amount, Direction: ledger.DirectionCredit, Currency: req.Currency},
		},
	}

	if err := s.ledgerRepo.CreateTransaction(ctx, tx, ledgerTxn); err != nil {
		return nil, fmt.Errorf("failed to create ledger transaction: %w", err)
	}

	// 7. Insert into outbox_events table for Kafka processing
	eventPayload := map[string]interface{}{
		"transfer_id": transfer.ID,
		"status":      transfer.Status,
		"amount":      transfer.Amount,
		"currency":    transfer.Currency,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
	if err := s.outboxRepo.InsertEvent(ctx, tx, "transfer.events", eventPayload); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// 8. Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	success = true
	return transfer, nil
}
