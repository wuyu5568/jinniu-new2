package conf

import "testing"

func TestApplyEnvOverrides_Payout(t *testing.T) {
	t.Setenv(EnvPayoutEnabled, "1")
	t.Setenv(EnvHotWalletKey, "aabb")
	t.Setenv(EnvBscRPC, "https://example.invalid/")

	bc := &Bootstrap{}
	bc.App.PayoutEnabled = false
	bc.App.HotWalletKey = ""
	bc.App.BscRPC = "https://bsc-dataseed.binance.org/"
	applyEnvOverrides(bc)

	if !bc.App.PayoutEnabled {
		t.Fatal("expected payout enabled from env")
	}
	if bc.App.HotWalletKey != "aabb" {
		t.Fatalf("hot key: got %q", bc.App.HotWalletKey)
	}
	if bc.App.BscRPC != "https://example.invalid/" {
		t.Fatalf("rpc: got %q", bc.App.BscRPC)
	}
}

func TestApplyEnvOverrides_PayoutOff(t *testing.T) {
	t.Setenv(EnvPayoutEnabled, "0")
	bc := &Bootstrap{}
	bc.App.PayoutEnabled = true
	applyEnvOverrides(bc)
	if bc.App.PayoutEnabled {
		t.Fatal("expected payout disabled when env=0")
	}
}

func TestApplyEnvOverrides_PayoutMax(t *testing.T) {
	t.Setenv(EnvPayoutMaxUSDT, "1")
	bc := &Bootstrap{}
	applyEnvOverrides(bc)
	if bc.App.PayoutMaxUSDT != 1 {
		t.Fatalf("max: got %v", bc.App.PayoutMaxUSDT)
	}
}

func TestValidateAppSafety_PayoutNeedsMax(t *testing.T) {
	err := ValidateAppSafety(&App{PayoutEnabled: true, PayoutMaxUSDT: 0})
	if err == nil {
		t.Fatal("expected error when payout on without max")
	}
	if err := ValidateAppSafety(&App{PayoutEnabled: true, PayoutMaxUSDT: 1}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAppSafety(&App{PayoutEnabled: false, PayoutMaxUSDT: 0}); err != nil {
		t.Fatal(err)
	}
}

func TestEnvTruthy(t *testing.T) {
	for _, v := range []string{"1", "TRUE", "yes", "on"} {
		if !envTruthy(v) {
			t.Fatalf("%q should be truthy", v)
		}
	}
	for _, v := range []string{"0", "false", "", "no"} {
		if envTruthy(v) {
			t.Fatalf("%q should be falsy", v)
		}
	}
}
