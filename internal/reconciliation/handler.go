package reconciliation

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wallet-ledger/internal/auth"
	"github.com/wallet-ledger/internal/middleware"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole(auth.RoleSystem, auth.RoleAdmin))
		r.Get("/runs", h.HandleGetRuns)
	})
}

func (h *Handler) HandleGetRuns(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, status, wallets_checked, ledger_transactions_checked, mismatches_found, started_at, completed_at
		FROM reconciliation_runs
		ORDER BY started_at DESC
		LIMIT 20
	`
	rows, err := h.db.Query(r.Context(), query)
	if err != nil {
		http.Error(w, "failed to query runs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var run Run
		if err := rows.Scan(
			&run.ID, &run.Status, &run.WalletsChecked, &run.LedgerTransactionsChecked,
			&run.MismatchesFound, &run.StartedAt, &run.CompletedAt,
		); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		runs = append(runs, run)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}
