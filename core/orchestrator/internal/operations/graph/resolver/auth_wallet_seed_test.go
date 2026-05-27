package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/customer/auth"
	"ironflyer/core/orchestrator/internal/operations/graph/model"
)

func TestSignUpDevWalletSeedIsScopedToCreatedUser(t *testing.T) {
	ctx := context.Background()
	userStore := auth.NewMemoryUserStore()
	walletSvc := wallet.NewMemoryService()
	resolver := &Resolver{
		Auth:             auth.NewService(userStore, []byte("test-secret"), "test", time.Hour),
		WalletSvc:        walletSvc,
		DevEnv:           "dev",
		DevWalletSeedUSD: 12.5,
		Logger:           zerolog.Nop(),
	}

	session, err := (&mutationResolver{resolver}).SignUp(ctx, model.SignUpInput{
		Email:    "private-budget@ironflyer.dev",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("SignUp() error = %v", err)
	}

	userWallet, err := walletSvc.Get(ctx, session.User.ID)
	if err != nil {
		t.Fatalf("Get(user wallet) error = %v", err)
	}
	if want := decimal.RequireFromString("12.5"); !userWallet.BalanceUSD.Equal(want) {
		t.Fatalf("user wallet balance = %s, want %s", userWallet.BalanceUSD, want)
	}

	demoWallet, err := walletSvc.Get(ctx, "demo")
	if err != nil {
		t.Fatalf("Get(demo wallet) error = %v", err)
	}
	if !demoWallet.BalanceUSD.IsZero() {
		t.Fatalf("demo wallet balance = %s, want 0", demoWallet.BalanceUSD)
	}
}
