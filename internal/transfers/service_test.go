package transfers_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wallet-ledger/internal/ledger"
	"github.com/wallet-ledger/internal/outbox"
	"github.com/wallet-ledger/internal/transfers"
	"github.com/wallet-ledger/internal/wallets"
)

// This test requires a running Postgres instance.
// Set DATABASE_URL environment variable to point to a test database.
// Example: DATABASE_URL="postgres://postgres:password@localhost:5432/wallet_test?sslmode=disable" go test ./...
func TestTransferService_Concurrency(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to db: %v", err)
	}
	defer db.Close()

	// Clear tables (order matters due to FK constraints)
	_, _ = db.Exec(ctx, "TRUNCATE outbox_events, ledger_entries, ledger_transactions, transfers, wallets, accounts, idempotency_keys, users CASCADE")

	// Dependencies
	walletRepo := wallets.NewRepository(db)
	ledgerRepo := ledger.NewRepository()
	outboxRepo := outbox.NewRepository(db)
	transferSvc := transfers.NewService(db, ledgerRepo, walletRepo, outboxRepo)

	// Create 2 test users (owner_id FK references users)
	var user1ID, user2ID string
	_ = db.QueryRow(ctx, "INSERT INTO users (email, password_hash, role) VALUES ('user1@test.com', 'hash', 'USER') RETURNING id").Scan(&user1ID)
	_ = db.QueryRow(ctx, "INSERT INTO users (email, password_hash, role) VALUES ('user2@test.com', 'hash', 'USER') RETURNING id").Scan(&user2ID)

	// Create 2 test accounts (schema has: id, type, currency, created_at)
	var account1ID, account2ID string
	_ = db.QueryRow(ctx, "INSERT INTO accounts (type, currency) VALUES ('USER', 'USD') RETURNING id").Scan(&account1ID)
	_ = db.QueryRow(ctx, "INSERT INTO accounts (type, currency) VALUES ('USER', 'USD') RETURNING id").Scan(&account2ID)

	// Create 2 test wallets (schema has: owner_id, account_id, currency, available_balance, locked_balance, status, version)
	var wallet1ID, wallet2ID string
	_ = db.QueryRow(ctx, "INSERT INTO wallets (owner_id, account_id, currency, available_balance, locked_balance, status, version) VALUES ($1, $2, 'USD', 1000, 0, 'ACTIVE', 1) RETURNING id", user1ID, account1ID).Scan(&wallet1ID)
	_ = db.QueryRow(ctx, "INSERT INTO wallets (owner_id, account_id, currency, available_balance, locked_balance, status, version) VALUES ($1, $2, 'USD', 1000, 0, 'ACTIVE', 1) RETURNING id", user2ID, account2ID).Scan(&wallet2ID)

	// Goal: Fire 100 concurrent transfers:
	// 50 from Wallet1 -> Wallet2 (amount = 10)
	// 50 from Wallet2 -> Wallet1 (amount = 5)
	// Expected Result:
	// Wallet1 Balance = 1000 - (50 * 10) + (50 * 5) = 1000 - 500 + 250 = 750
	// Wallet2 Balance = 1000 + (50 * 10) - (50 * 5) = 1000 + 500 - 250 = 1250

	var wg sync.WaitGroup
	workers := 100
	wg.Add(workers)

	start := time.Now()
	for i := 0; i < 50; i++ {
		// Worker for Wallet1 -> Wallet2
		go func(idx int) {
			defer wg.Done()
			req := transfers.CreateTransferRequest{
				SourceWalletID:      wallet1ID,
				DestinationWalletID: wallet2ID,
				Amount:              10,
				Currency:            "USD",
			}
			idemKey := fmt.Sprintf("w1-to-w2-%d", idx)
			_, _ = db.Exec(ctx, "INSERT INTO idempotency_keys (key, status, request_payload) VALUES ($1, 'IN_PROGRESS', '{}')", idemKey)
			_, err := transferSvc.ExecuteTransfer(ctx, req, idemKey)
			if err != nil {
				t.Errorf("Transfer failed: %v", err)
			}
		}(i)

		// Worker for Wallet2 -> Wallet1
		go func(idx int) {
			defer wg.Done()
			req := transfers.CreateTransferRequest{
				SourceWalletID:      wallet2ID,
				DestinationWalletID: wallet1ID,
				Amount:              5,
				Currency:            "USD",
			}
			idemKey := fmt.Sprintf("w2-to-w1-%d", idx)
			_, _ = db.Exec(ctx, "INSERT INTO idempotency_keys (key, status, request_payload) VALUES ($1, 'IN_PROGRESS', '{}')", idemKey)
			_, err := transferSvc.ExecuteTransfer(ctx, req, idemKey)
			if err != nil {
				t.Errorf("Transfer failed: %v", err)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Completed 100 concurrent transfers in %v", time.Since(start))

	// Assert Final Balances
	w1, _ := walletRepo.GetWalletByID(ctx, wallet1ID)
	w2, _ := walletRepo.GetWalletByID(ctx, wallet2ID)

	if w1.AvailableBalance != 750 {
		t.Errorf("Expected Wallet1 balance to be 750, got %d", w1.AvailableBalance)
	}
	if w2.AvailableBalance != 1250 {
		t.Errorf("Expected Wallet2 balance to be 1250, got %d", w2.AvailableBalance)
	}

	// Assert Ledger Invariant
	var totalDebits, totalCredits int64
	_ = db.QueryRow(ctx, `
		SELECT 
			SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END),
			SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END)
		FROM ledger_entries
	`).Scan(&totalDebits, &totalCredits)

	if totalDebits != totalCredits {
		t.Errorf("Ledger imbalanced! Debits: %d, Credits: %d", totalDebits, totalCredits)
	}
}
