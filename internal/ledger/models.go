package ledger

import "time"

type Direction string

const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

type TransactionType string

const (
	TxTypeTransfer   TransactionType = "TRANSFER"
	TxTypePayment    TransactionType = "PAYMENT"
	TxTypeRefund     TransactionType = "REFUND"
	TxTypeAdjustment TransactionType = "ADJUSTMENT"
	TxTypeReversal   TransactionType = "REVERSAL"
	TxTypeFee        TransactionType = "FEE"
)

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "PENDING"
	StatusCompleted TransactionStatus = "COMPLETED"
	StatusFailed    TransactionStatus = "FAILED"
)

type Transaction struct {
	ID            string            `json:"id"`
	ReferenceID   string            `json:"reference_id"`
	ReferenceType TransactionType   `json:"reference_type"`
	Status        TransactionStatus `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	Entries       []Entry           `json:"entries"`
}

type Entry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	Amount        int64     `json:"amount"` // Absolute amount
	Direction     Direction `json:"direction"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}
