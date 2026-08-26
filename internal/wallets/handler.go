package wallets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/wallet-ledger/internal/auth"
	"github.com/wallet-ledger/internal/middleware"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleCreateWallet)
	r.Get("/", h.HandleGetWallets)
	r.Get("/{id}", h.HandleGetWallet)
	r.Get("/{id}/balance", h.HandleGetBalance)
	r.Get("/{id}/transactions", h.HandleGetTransactions)
	
	// Administrative routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole(auth.RoleSystem, auth.RoleAdmin))
		r.Post("/{id}/freeze", h.HandleFreezeWallet)
		r.Post("/{id}/unfreeze", h.HandleUnfreezeWallet)
	})
}

func (h *Handler) HandleCreateWallet(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	wallet, err := h.service.CreateWallet(r.Context(), req, claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wallet)
}

func (h *Handler) HandleGetWallet(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	wallet, err := h.service.GetWallet(r.Context(), id, claims.UserID, claims.Role)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			http.Error(w, "wallet not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrUnauthorizedWalletAccess) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wallet)
}

func (h *Handler) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	balance, err := h.service.GetBalance(r.Context(), id, claims.UserID, claims.Role)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			http.Error(w, "wallet not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrUnauthorizedWalletAccess) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balance)
}

func (h *Handler) HandleFreezeWallet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.FreezeWallet(r.Context(), id); err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			http.Error(w, "wallet not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleUnfreezeWallet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.UnfreezeWallet(r.Context(), id); err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			http.Error(w, "wallet not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleGetTransactions(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	transactions, err := h.service.GetTransactions(r.Context(), id, claims.UserID, claims.Role)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			http.Error(w, "wallet not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrUnauthorizedWalletAccess) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

func (h *Handler) HandleGetWallets(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	wallets, err := h.service.GetWallets(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wallets)
}
