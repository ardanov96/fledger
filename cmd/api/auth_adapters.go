// auth_adapters.go — wire AuthService to handler.AuthAPI + tx runner adapter.
package main

import (
	"context"

	"github.com/runut/fmcg-wallet/internal/domain/auth"
	"github.com/runut/fmcg-wallet/internal/handler"
	platformauth "github.com/runut/fmcg-wallet/internal/platform/auth"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// authTxAdapter wraps DB.RunInTxAuthDomain as usecase.AuthTxRunner.
type authTxAdapter struct {
	db *postgres.DB
}

func (a *authTxAdapter) RunInTxAuthDomain(ctx context.Context, fn func(auth.Tx) error) error {
	return a.db.RunInTxAuthDomain(ctx, fn)
}

// authAPIAdapter is the handler-side adapter for usecase.AuthService.
type authAPIAdapter struct {
	svc *usecase.AuthService
}

func (a *authAPIAdapter) Login(ctx context.Context, in usecase.LoginInput) (*usecase.LoginResult, error) {
	return a.svc.Login(ctx, in)
}

func (a *authAPIAdapter) VerifyMFA(ctx context.Context, in usecase.VerifyMFAInput) (*usecase.RefreshResult, error) {
	return a.svc.VerifyMFA(ctx, in)
}

func (a *authAPIAdapter) Refresh(ctx context.Context, in usecase.RefreshInput) (*usecase.RefreshResult, error) {
	return a.svc.Refresh(ctx, in)
}

func (a *authAPIAdapter) Logout(ctx context.Context, in usecase.LogoutInput) error {
	return a.svc.Logout(ctx, in)
}

func (a *authAPIAdapter) SetupMFA(ctx context.Context, in usecase.SetupMFAInput) (*usecase.SetupMFAResult, error) {
	return a.svc.SetupMFA(ctx, in)
}

var _ handler.AuthAPI = (*authAPIAdapter)(nil)

// =============================================================================
// helpers (singleton instances of platform/auth implementations)
// =============================================================================

var (
	passwordHasher = platformauth.NewBcryptPasswordHasher()
	tokenGenerator = platformauth.NewDefaultTokenGenerator()
	totpGenerator  = platformauth.NewTOTPGenerator()
)

var (
	_ usecase.PasswordHasherDep = (*platformauth.BcryptPasswordHasher)(nil)
	_ usecase.TokenGeneratorDep = (*platformauth.DefaultTokenGenerator)(nil)
	_ usecase.TOTPGeneratorDep  = (*platformauth.TOTPGenerator)(nil)
)
