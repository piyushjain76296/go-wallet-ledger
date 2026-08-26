package transfers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/wallet-ledger/internal/auth"
	"github.com/wallet-ledger/internal/idempotency"
	"github.com/wallet-ledger/internal/middleware"
	"github.com/wallet-ledger/internal/wallets"
)

type Handler struct {
	service     Service
	idemRepo    idempotency.Repository
	walletRepo  wallets.Repository
}

func NewHandler(service Service, idemRepo idempotency.Repository, walletRepo wallets.Repository) *Handler {
	return &Handler{
		service:    service,
		idemRepo:   idemRepo,
		walletRepo: walletRepo,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleTransfer)
}

func (h *Handler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	// Read raw body for idempotency store
	var req CreateTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	reqBytes, _ := json.Marshal(req)

	// Idempotency check
	idemRecord, err := h.idemRepo.TryAcquire(r.Context(), idemKey, reqBytes)
	if err != nil {
		if err == idempotency.ErrKeyInProgress {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if idemRecord != nil {
		// Already completed, return cached response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(idemRecord.ResponseStatus)
		_, _ = w.Write(idemRecord.ResponsePayload)
		return
	}

	// Ensure caller owns the source wallet
	sourceWallet, err := h.walletRepo.GetWalletByID(r.Context(), req.SourceWalletID)
	if err != nil || (sourceWallet.OwnerID != claims.UserID && claims.Role != auth.RoleSystem) {
		h.idemRepo.Complete(r.Context(), idemKey, []byte(`{"error":"unauthorized access to source wallet"}`), http.StatusForbidden)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Execute Transfer
	transfer, err := h.service.ExecuteTransfer(r.Context(), req, idemKey)
	if err != nil {
		respBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		h.idemRepo.Complete(r.Context(), idemKey, respBytes, http.StatusBadRequest)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respBytes, _ := json.Marshal(transfer)
	h.idemRepo.Complete(r.Context(), idemKey, respBytes, http.StatusCreated)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(respBytes)
}
