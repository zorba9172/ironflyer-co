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

func TestSignInDevWalletFloorRefillsExistingUser(t *testing.T) {
	ctx := context.Background()
	userStore := auth.NewMemoryUserStore()
	walletSvc := wallet.NewMemoryService()
	resolver := &Resolver{
		Auth:              auth.NewService(userStore, []byte("test-secret"), "test", time.Hour),
		WalletSvc:         walletSvc,
		DevEnv:            "dev",
		DevWalletFloorUSD: 20,
		Logger:            zerolog.Nop(),
	}

	session, err := (&mutationResolver{resolver}).SignUp(ctx, model.SignUpInput{
		Email:    "builder-credit@ironflyer.dev",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("SignUp() error = %v", err)
	}

	if err := walletSvc.Hold(ctx, session.User.ID, decimal.NewFromInt(18)); err != nil {
		t.Fatalf("Hold() error = %v", err)
	}
	before, err := walletSvc.Get(ctx, session.User.ID)
	if err != nil {
		t.Fatalf("Get(wallet before signin) error = %v", err)
	}
	if want := decimal.NewFromInt(2); !before.AvailableUSD().Equal(want) {
		t.Fatalf("available before signin = %s, want %s", before.AvailableUSD(), want)
	}

	if _, err := (&mutationResolver{resolver}).SignIn(ctx, model.SignInInput{
		Email:    "builder-credit@ironflyer.dev",
		Password: "password123",
	}); err != nil {
		t.Fatalf("SignIn() error = %v", err)
	}

	after, err := walletSvc.Get(ctx, session.User.ID)
	if err != nil {
		t.Fatalf("Get(wallet after signin) error = %v", err)
	}
	if want := decimal.NewFromInt(20); !after.AvailableUSD().Equal(want) {
		t.Fatalf("available after signin = %s, want %s", after.AvailableUSD(), want)
	}
	if want := decimal.NewFromInt(38); !after.BalanceUSD.Equal(want) {
		t.Fatalf("balance after signin = %s, want %s", after.BalanceUSD, want)
	}
}
