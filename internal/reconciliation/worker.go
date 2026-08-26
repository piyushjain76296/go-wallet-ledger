package reconciliation

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Worker struct {
	repo Repository
}

func NewWorker(repo Repository) *Worker {
	return &Worker{repo: repo}
}

func (w *Worker) Start(ctx context.Context) {
	// Run immediately on startup, then every hour
	w.RunReconciliation(ctx)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Reconciliation worker shutting down")
			return
		case <-ticker.C:
			w.RunReconciliation(ctx)
		}
	}
}

func (w *Worker) RunReconciliation(ctx context.Context) {
	slog.Info("Starting financial reconciliation run")
	
	run, err := w.repo.CreateRun(ctx)
	if err != nil {
		slog.Error("Failed to create reconciliation run", "error", err)
		return
	}

	var mismatchesFound int64
	status := StatusPassed

	// 1. Global Ledger Invariants
	debits, credits, txnCount, err := w.repo.CheckGlobalLedgerBalance(ctx)
	if err != nil {
		slog.Error("Reconciliation global ledger check failed", "error", err)
		w.repo.CompleteRun(ctx, run.ID, StatusFailed, 0, 0, 0)
		return
	}

	if debits != credits {
		status = StatusFailed
		mismatchesFound++
		details := map[string]interface{}{
			"total_debits":  debits,
			"total_credits": credits,
			"difference":    debits - credits,
		}
		w.repo.RecordMismatch(ctx, run.ID, "SYSTEM", "GLOBAL_LEDGER", "IMBALANCED_LEDGER", details)
		slog.Error("CRITICAL: Global ledger imbalance detected!", "debits", debits, "credits", credits)
	}

	// 2. Wallet vs Ledger Mismatches
	walletsChecked, walletMismatches, err := w.repo.CheckWalletBalances(ctx)
	if err != nil {
		slog.Error("Reconciliation wallet check failed", "error", err)
		w.repo.CompleteRun(ctx, run.ID, StatusFailed, walletsChecked, txnCount, mismatchesFound)
		return
	}

	for _, mismatch := range walletMismatches {
		status = StatusFailed
		mismatchesFound++
		walletID := fmt.Sprintf("%v", mismatch["wallet_id"])
		w.repo.RecordMismatch(ctx, run.ID, "WALLET", walletID, "BALANCE_MISMATCH", mismatch)
		slog.Error("CRITICAL: Wallet balance mismatch detected!", "wallet_id", walletID, "mismatch", mismatch)
	}

	// 3. Complete Run
	if err := w.repo.CompleteRun(ctx, run.ID, status, walletsChecked, txnCount, mismatchesFound); err != nil {
		slog.Error("Failed to complete reconciliation run", "error", err)
	}

	slog.Info("Reconciliation run completed", "status", status, "wallets_checked", walletsChecked, "txns_checked", txnCount, "mismatches", mismatchesFound)
}
