// Package usecase — AccountService orchestrates account CRUD and ledger entry queries.
package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
)

// AccountService implements ledger.AccountService.
type AccountService struct {
	accounts ledger.AccountRepository
	entries  ledger.EntryRepository
}

// NewAccountService creates a new account service.
func NewAccountService(accounts ledger.AccountRepository, entries ledger.EntryRepository) *AccountService {
	return &AccountService{accounts: accounts, entries: entries}
}

// Create creates a new account.
func (s *AccountService) Create(ctx context.Context, account ledger.Account) (ledger.Account, error) {
	if account.ID == "" {
		account.ID = uuid.NewString()
	}
	if account.Status == "" {
		account.Status = ledger.AccountStatusActive
	}
	if account.TenantID == "" {
		account.TenantID = uuid.NewString()
	}

	if err := s.accounts.Create(ctx, account); err != nil {
		return ledger.Account{}, fmt.Errorf("create account: %w", err)
	}
	return account, nil
}

// GetByID returns an account by ID.
func (s *AccountService) GetByID(ctx context.Context, id string) (ledger.Account, error) {
	return s.accounts.GetByID(ctx, id)
}

// GetByCode returns an account by code.
func (s *AccountService) GetByCode(ctx context.Context, code string) (ledger.Account, error) {
	return s.accounts.GetByCode(ctx, code)
}

// List returns accounts matching the filter.
func (s *AccountService) List(ctx context.Context, filter ledger.AccountFilter) ([]ledger.Account, error) {
	return s.accounts.List(ctx, filter)
}

// ListEntries returns the ledger entries for an account, newest first.
func (s *AccountService) ListEntries(ctx context.Context, accountID string, limit int) ([]ledger.Entry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	return s.entries.ListByAccount(ctx, accountID, ledger.EntryFilter{Limit: limit})
}

// Freeze marks an account as frozen.
func (s *AccountService) Freeze(ctx context.Context, id, reason string) error {
	a, err := s.accounts.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("freeze: %w", err)
	}
	a.Status = ledger.AccountStatusFrozen
	return s.accounts.Update(ctx, a)
}

// Close marks an account as closed.
func (s *AccountService) Close(ctx context.Context, id, reason string) error {
	a, err := s.accounts.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	a.Status = ledger.AccountStatusClosed
	return s.accounts.Update(ctx, a)
}
