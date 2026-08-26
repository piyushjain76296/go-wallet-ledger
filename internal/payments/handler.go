package payments

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/wallet-ledger/internal/middleware"
)

type Handler struct {
	service  Service
	provider *ProviderSimulator
}

func NewHandler(service Service, provider *ProviderSimulator) *Handler {
	return &Handler{service: service, provider: provider}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleCreatePayment)
	r.Post("/demo-fund", h.HandleDemoFund)
	r.Post("/webhook", h.HandleWebhook)
}

type CreatePaymentRequest struct {
	WalletID string `json:"wallet_id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func (h *Handler) HandleCreatePayment(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	payment, err := h.service.CreatePayment(r.Context(), req.WalletID, req.Amount, req.Currency)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(payment)
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if !h.provider.VerifyWebhookSignature(payload, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	err = h.service.ProcessWebhook(r.Context(), "simulator", payload)
	if err != nil {
		if errors.Is(err, ErrDuplicateWebhook) {
			// Duplicate webhook is a success for idempotency
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrInvalidStateTransition) || errors.Is(err, ErrPaymentNotFound) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleDemoFund(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Create payment
	payment, err := h.service.CreatePayment(r.Context(), req.WalletID, req.Amount, req.Currency)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// 2. Process Webhook directly (bypass signature)
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"event_id":     "demo-event-" + payment.ID,
		"event_type":   "payment.succeeded",
		"reference_id": payment.ProviderReference,
		"status":       StatusSucceeded,
	})

	if err := h.service.ProcessWebhook(r.Context(), "simulator", payloadBytes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "success"}`))
}
