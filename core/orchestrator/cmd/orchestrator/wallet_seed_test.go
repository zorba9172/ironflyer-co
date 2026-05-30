package main

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/business/wallet"
	"ironflyer/core/orchestrator/internal/customer/auth"
)

func TestSeedDevSuperuserWalletUsesCanonicalTenant(t *testing.T) {
	ctx := context.Background()
	walletSvc := wallet.NewMemoryService()
	superuser := auth.User{ID: "user-1", OrgID: "org-1"}

	tenant, err := seedDevSuperuserWallet(ctx, walletSvc, superuser, 50)
	if err != nil {
		t.Fatalf("seedDevSuperuserWallet() error = %v", err)
	}
	if tenant != "org-1" {
		t.Fatalf("tenant = %q, want org-1", tenant)
	}

	orgWallet, err := walletSvc.Get(ctx, "org-1")
	if err != nil {
		t.Fatalf("Get(org wallet) error = %v", err)
	}
	if want := decimal.NewFromInt(50); !orgWallet.BalanceUSD.Equal(want) {
		t.Fatalf("org wallet balance = %s, want %s", orgWallet.BalanceUSD, want)
	}

	userWallet, err := walletSvc.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get(user wallet) error = %v", err)
	}
	if !userWallet.BalanceUSD.IsZero() {
		t.Fatalf("user wallet balance = %s, want 0", userWallet.BalanceUSD)
	}

	if _, err := seedDevSuperuserWallet(ctx, walletSvc, superuser, 50); err != nil {
		t.Fatalf("second seedDevSuperuserWallet() error = %v", err)
	}
	orgWallet, err = walletSvc.Get(ctx, "org-1")
	if err != nil {
		t.Fatalf("Get(org wallet after second seed) error = %v", err)
	}
	if want := decimal.NewFromInt(50); !orgWallet.BalanceUSD.Equal(want) {
		t.Fatalf("org wallet balance after second seed = %s, want idempotent %s", orgWallet.BalanceUSD, want)
	}
}

func TestEnsureDevSuperuserWalletFloorRefillsCanonicalTenant(t *testing.T) {
	ctx := context.Background()
	walletSvc := wallet.NewMemoryService()
	superuser := auth.User{ID: "user-1", OrgID: "org-1"}

	if err := walletSvc.TopUp(ctx, "org-1", decimal.NewFromInt(20), "manual"); err != nil {
		t.Fatalf("TopUp() error = %v", err)
	}

	tenant, credited, err := ensureDevSuperuserWalletFloor(ctx, walletSvc, superuser, 50)
	if err != nil {
		t.Fatalf("ensureDevSuperuserWalletFloor() error = %v", err)
	}
	if tenant != "org-1" {
		t.Fatalf("tenant = %q, want org-1", tenant)
	}
	if want := decimal.NewFromInt(30); !credited.Equal(want) {
		t.Fatalf("credited = %s, want %s", credited, want)
	}

	orgWallet, err := walletSvc.Get(ctx, "org-1")
	if err != nil {
		t.Fatalf("Get(org wallet) error = %v", err)
	}
	if want := decimal.NewFromInt(50); !orgWallet.AvailableUSD().Equal(want) {
		t.Fatalf("org wallet available = %s, want %s", orgWallet.AvailableUSD(), want)
	}

	_, credited, err = ensureDevSuperuserWalletFloor(ctx, walletSvc, superuser, 50)
	if err != nil {
		t.Fatalf("second ensureDevSuperuserWalletFloor() error = %v", err)
	}
	if !credited.IsZero() {
		t.Fatalf("second credited = %s, want 0", credited)
	}
	orgWallet, err = walletSvc.Get(ctx, "org-1")
	if err != nil {
		t.Fatalf("Get(org wallet after second floor) error = %v", err)
	}
	if want := decimal.NewFromInt(50); !orgWallet.AvailableUSD().Equal(want) {
		t.Fatalf("org wallet available after second floor = %s, want %s", orgWallet.AvailableUSD(), want)
	}
}
