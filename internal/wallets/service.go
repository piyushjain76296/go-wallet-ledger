package wallets

import (
	"context"
	"errors"

	"github.com/wallet-ledger/internal/auth"
)

var (
	ErrUnauthorizedWalletAccess = errors.New("unauthorized to access this wallet")
)

type Service interface {
	CreateWallet(ctx context.Context, req CreateWalletRequest, ownerID string, role auth.Role) (*Wallet, error)
	GetWallet(ctx context.Context, id string, callerID string, role auth.Role) (*Wallet, error)
	GetBalance(ctx context.Context, id string, callerID string, role auth.Role) (*BalanceResponse, error)
	FreezeWallet(ctx context.Context, id string) error
	UnfreezeWallet(ctx context.Context, id string) error
	GetTransactions(ctx context.Context, id string, callerID string, role auth.Role) ([]Transaction, error)
	GetWallets(ctx context.Context, callerID string) ([]Wallet, error)
}

type serviceImpl struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &serviceImpl{repo: repo}
}

func (s *serviceImpl) CreateWallet(ctx context.Context, req CreateWalletRequest, ownerID string, role auth.Role) (*Wallet, error) {
	if req.Currency == "" {
		return nil, errors.New("currency is required")
	}
	
	// Map auth role to account type
	accountType := string(role)
	return s.repo.CreateWalletAndAccount(ctx, ownerID, accountType, req.Currency)
}

func (s *serviceImpl) GetWallet(ctx context.Context, id string, callerID string, role auth.Role) (*Wallet, error) {
	wallet, err := s.repo.GetWalletByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only owner or admin/system can view
	if wallet.OwnerID != callerID && role != auth.RoleSystem && role != auth.RoleAdmin {
		return nil, ErrUnauthorizedWalletAccess
	}

	return wallet, nil
}

func (s *serviceImpl) GetBalance(ctx context.Context, id string, callerID string, role auth.Role) (*BalanceResponse, error) {
	w, err := s.GetWallet(ctx, id, callerID, role)
	if err != nil {
		return nil, err
	}

	return &BalanceResponse{
		WalletID:         w.ID,
		Currency:         w.Currency,
		AvailableBalance: w.AvailableBalance,
		LockedBalance:    w.LockedBalance,
		LedgerBalance:    w.AvailableBalance + w.LockedBalance,
	}, nil
}

func (s *serviceImpl) FreezeWallet(ctx context.Context, id string) error {
	// Status changes usually restricted via handlers + middleware (System/Admin)
	return s.repo.UpdateWalletStatus(ctx, id, StatusFrozen)
}

func (s *serviceImpl) UnfreezeWallet(ctx context.Context, id string) error {
	return s.repo.UpdateWalletStatus(ctx, id, StatusActive)
}

func (s *serviceImpl) GetTransactions(ctx context.Context, id string, callerID string, role auth.Role) ([]Transaction, error) {
	w, err := s.GetWallet(ctx, id, callerID, role)
	if err != nil {
		return nil, err
	}
	
	return s.repo.GetTransactions(ctx, w.AccountID)
}

func (s *serviceImpl) GetWallets(ctx context.Context, callerID string) ([]Wallet, error) {
	return s.repo.GetWalletsByOwner(ctx, callerID)
}
