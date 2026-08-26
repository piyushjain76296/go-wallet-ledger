package wallets

import "time"

type WalletStatus string

const (
	StatusActive WalletStatus = "ACTIVE"
	StatusFrozen WalletStatus = "FROZEN"
	StatusClosed WalletStatus = "CLOSED"
)

type Wallet struct {
	ID               string       `json:"id"`
	OwnerID          string       `json:"owner_id"`
	AccountID        string       `json:"account_id"`
	Currency         string       `json:"currency"`
	AvailableBalance int64        `json:"available_balance"`
	LockedBalance    int64        `json:"locked_balance"`
	Status           WalletStatus `json:"status"`
	Version          int64        `json:"version"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

type CreateWalletRequest struct {
	Currency string `json:"currency"`
}

// BalanceResponse provides a unified view of the balance
type BalanceResponse struct {
	WalletID         string `json:"wallet_id"`
	Currency         string `json:"currency"`
	AvailableBalance int64  `json:"available_balance"`
	LockedBalance    int64  `json:"locked_balance"`
	LedgerBalance    int64  `json:"ledger_balance"` // sum of available and locked
}

type Transaction struct {
	ID            string    `json:"id"`
	ReferenceID   string    `json:"reference_id"`
	ReferenceType string    `json:"reference_type"`
	Amount        int64     `json:"amount"`
	Direction     string    `json:"direction"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
