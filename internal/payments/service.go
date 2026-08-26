package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wallet-ledger/internal/ledger"
	"github.com/wallet-ledger/internal/observability"
	"github.com/wallet-ledger/internal/outbox"
)

var (
	ErrInvalidStateTransition = errors.New("invalid payment state transition")
	ErrPaymentNotFound        = errors.New("payment not found")
	ErrDuplicateWebhook       = errors.New("duplicate webhook event")
)

type Service interface {
	CreatePayment(ctx context.Context, walletID string, amount int64, currency string) (*Payment, error)
	ProcessWebhook(ctx context.Context, provider string, payload []byte) error
}

type serviceImpl struct {
	db         *pgxpool.Pool
	ledgerRepo ledger.Repository
	provider   *ProviderSimulator
	outboxRepo outbox.Repository
}

func NewService(db *pgxpool.Pool, ledgerRepo ledger.Repository, provider *ProviderSimulator, outboxRepo outbox.Repository) Service {
	return &serviceImpl{
		db:         db,
		ledgerRepo: ledgerRepo,
		provider:   provider,
		outboxRepo: outboxRepo,
	}
}

func (s *serviceImpl) CreatePayment(ctx context.Context, walletID string, amount int64, currency string) (*Payment, error) {
	// 1. Call provider
	resp := s.provider.ProcessPayment(amount, currency)

	// 2. Save payment
	payment := &Payment{
		WalletID:          walletID,
		Amount:            amount,
		Currency:          currency,
		Status:            resp.Status, // E.g., PROCESSING or FAILED
		ProviderReference: resp.ReferenceID,
	}

	err := s.db.QueryRow(ctx, `
		INSERT INTO payments (wallet_id, amount, currency, status, provider_reference)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`, payment.WalletID, payment.Amount, payment.Currency, payment.Status, payment.ProviderReference).
		Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)

	if err != nil {
		return nil, err
	}

	observability.RecordPayment(string(payment.Status))
	return payment, nil
}

func (s *serviceImpl) ProcessWebhook(ctx context.Context, provider string, payload []byte) error {
	var event WebhookPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Webhook Deduplication
	_, err = tx.Exec(ctx, `
		INSERT INTO webhook_events (provider, event_id, event_type, payload, status)
		VALUES ($1, $2, $3, $4, $5)
	`, provider, event.EventID, event.EventType, payload, WebhookStatusProcessed)

	if err != nil {
		// Check for unique constraint violation using pgconn error code
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			return ErrDuplicateWebhook
		}
		return err
	}

	// 2. Lock Payment
	var p Payment
	var accountID string
	err = tx.QueryRow(ctx, `
		SELECT p.id, p.wallet_id, p.amount, p.currency, p.status, w.account_id
		FROM payments p
		JOIN wallets w ON p.wallet_id = w.id
		WHERE p.provider_reference = $1
		FOR UPDATE OF p
	`, event.ReferenceID).Scan(&p.ID, &p.WalletID, &p.Amount, &p.Currency, &p.Status, &accountID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPaymentNotFound
		}
		return err
	}

	// 3. State Machine check
	if !isValidTransition(p.Status, event.Status) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStateTransition, p.Status, event.Status)
	}

	// 4. Update Payment
	_, err = tx.Exec(ctx, `UPDATE payments SET status = $1, updated_at = NOW() WHERE id = $2`, event.Status, p.ID)
	if err != nil {
		return err
	}
	observability.RecordPayment(string(event.Status))

	// 5. If successful, credit the wallet and write ledger
	if event.Status == StatusSucceeded {
		// Lock wallet safely (since we only lock one wallet, no deadlock risk against other single-locks, 
		// but must be careful if interacting with transfers. Transfers lock in order, this locks one).
		_, err = tx.Exec(ctx, `
			UPDATE wallets SET available_balance = available_balance + $1 WHERE id = $2
		`, p.Amount, p.WalletID)
		if err != nil {
			return err
		}

		ledgerTxn := &ledger.Transaction{
			ReferenceID:   p.ID,
			ReferenceType: ledger.TxTypePayment,
			Status:        ledger.StatusCompleted,
			Entries: []ledger.Entry{
				// Payment implies money coming from external system (Platform Liability Account) to User Wallet
				// We need a platform account ID. For brevity, assuming a global placeholder or system account.
				{AccountID: "00000000-0000-0000-0000-000000000000", Amount: p.Amount, Direction: ledger.DirectionDebit, Currency: p.Currency}, // External
				{AccountID: accountID, Amount: p.Amount, Direction: ledger.DirectionCredit, Currency: p.Currency},
			},
		}
		if err := s.ledgerRepo.CreateTransaction(ctx, tx, ledgerTxn); err != nil {
			return fmt.Errorf("ledger error: %w", err)
		}
	}

	eventPayload := map[string]interface{}{
		"payment_id": p.ID,
		"status":     event.Status,
		"amount":     p.Amount,
		"currency":   p.Currency,
	}
	if err := s.outboxRepo.InsertEvent(ctx, tx, "payment.events", eventPayload); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return tx.Commit(ctx)
}

func isValidTransition(current, next PaymentStatus) bool {
	// Simplify state machine for example
	switch current {
	case StatusProcessing:
		return next == StatusSucceeded || next == StatusFailed
	case StatusCreated:
		return next == StatusProcessing || next == StatusFailed
	}
	return false
}
