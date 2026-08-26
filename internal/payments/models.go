package payments

import "time"

type PaymentStatus string

const (
	StatusCreated          PaymentStatus = "CREATED"
	StatusProcessing       PaymentStatus = "PROCESSING"
	StatusSucceeded        PaymentStatus = "SUCCEEDED"
	StatusFailed           PaymentStatus = "FAILED"
	StatusRefundRequested  PaymentStatus = "REFUND_REQUESTED"
	StatusPartiallyRefunded PaymentStatus = "PARTIALLY_REFUNDED"
	StatusRefunded         PaymentStatus = "REFUNDED"
	StatusReversed         PaymentStatus = "REVERSED"
)

type Payment struct {
	ID               string        `json:"id"`
	WalletID         string        `json:"wallet_id"`
	Amount           int64         `json:"amount"`
	Currency         string        `json:"currency"`
	Status           PaymentStatus `json:"status"`
	ProviderReference string       `json:"provider_reference"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type WebhookEventStatus string

const (
	WebhookStatusPending   WebhookEventStatus = "PENDING"
	WebhookStatusProcessed WebhookEventStatus = "PROCESSED"
	WebhookStatusFailed    WebhookEventStatus = "FAILED"
)
