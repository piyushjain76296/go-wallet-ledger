package transfers

import "time"

type TransferStatus string

const (
	StatusPending   TransferStatus = "PENDING"
	StatusCompleted TransferStatus = "COMPLETED"
	StatusFailed    TransferStatus = "FAILED"
)

type Transfer struct {
	ID                  string         `json:"id"`
	SourceWalletID      string         `json:"source_wallet_id"`
	DestinationWalletID string         `json:"destination_wallet_id"`
	Amount              int64          `json:"amount"`
	Currency            string         `json:"currency"`
	Status              TransferStatus `json:"status"`
	IdempotencyKey      string         `json:"idempotency_key"`
	CreatedAt           time.Time      `json:"created_at"`
}

type CreateTransferRequest struct {
	SourceWalletID      string `json:"source_wallet_id"`
	DestinationWalletID string `json:"destination_wallet_id"`
	Amount              int64  `json:"amount"`
	Currency            string `json:"currency"`
}
