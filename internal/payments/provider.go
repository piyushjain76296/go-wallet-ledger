package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"time"
)

// ProviderSimulator simulates an external payment gateway (e.g., Stripe, Razorpay)
type ProviderSimulator struct {
	webhookSecret string
}

func NewProviderSimulator(secret string) *ProviderSimulator {
	return &ProviderSimulator{webhookSecret: secret}
}

type ProviderResponse struct {
	ReferenceID string
	Status      PaymentStatus
}

type WebhookPayload struct {
	EventID     string        `json:"event_id"`
	EventType   string        `json:"event_type"` // e.g., "payment.succeeded", "payment.failed"
	ReferenceID string        `json:"reference_id"`
	Status      PaymentStatus `json:"status"`
}

func (p *ProviderSimulator) ProcessPayment(amount int64, currency string) ProviderResponse {
	// Simulate network delay
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(200)+50))

	refID := "ext_" + generateRandomString(12)

	// Randomly fail 10% of synchronous requests
	if rand.Float32() < 0.10 {
		return ProviderResponse{ReferenceID: refID, Status: StatusFailed}
	}

	return ProviderResponse{ReferenceID: refID, Status: StatusProcessing}
}

func (p *ProviderSimulator) GenerateWebhookSignature(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (p *ProviderSimulator) VerifyWebhookSignature(payload []byte, signature string) bool {
	expectedSignature := p.GenerateWebhookSignature(payload)
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
