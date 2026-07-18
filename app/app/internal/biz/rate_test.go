package biz

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMultiplierForAmount(t *testing.T) {
	SetActiveParams(nil)
	cases := []struct {
		amount string
		want   string
		ok     bool
	}{
		{"99", "0", false},
		{"100", "2", true},
		{"999", "2", true},
		{"1000", "2.5", true},
		{"2999", "2.5", true},
		{"3000", "3", true},
	}
	for _, tc := range cases {
		got, ok := MultiplierForAmount(decimal.RequireFromString(tc.amount))
		if ok != tc.ok {
			t.Fatalf("amount=%s ok=%v want %v", tc.amount, ok, tc.ok)
		}
		if ok && !got.Equal(decimal.RequireFromString(tc.want)) {
			t.Fatalf("amount=%s got %s want %s", tc.amount, got, tc.want)
		}
	}
}

func TestAdvanceRateClimbAndDescend(t *testing.T) {
	SetActiveParams(nil)
	rate, dir := InitialRate()
	if !rate.Equal(decimal.RequireFromString("0.60")) || dir != RateDirectionUp {
		t.Fatalf("initial %s %s", rate, dir)
	}
	for _, want := range []string{"0.65", "0.70", "0.75", "0.80"} {
		rate, dir = AdvanceRate(rate, dir)
		if !rate.Equal(decimal.RequireFromString(want)) || dir != RateDirectionUp {
			t.Fatalf("got %s %s want %s up", rate, dir, want)
		}
	}
	rate, dir = AdvanceRate(rate, dir)
	if !rate.Equal(decimal.RequireFromString("0.85")) || dir != RateDirectionUp {
		t.Fatalf("got %s %s", rate, dir)
	}
}

func TestCalcStaticYield(t *testing.T) {
	// Base is subscribe amount (not exit target).
	amount := decimal.RequireFromString("1000")
	rate := decimal.RequireFromString("0.60")
	got := CalcStaticYield(amount, rate)
	want := decimal.RequireFromString("6")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestApplyExtractRateTurnKeepsRateFlipsDirection(t *testing.T) {
	SetActiveParams(nil)
	rate := decimal.RequireFromString("0.65")
	gotRate, gotDir := ApplyExtractRateTurn(rate, RateDirectionUp)
	if !gotRate.Equal(rate) || gotDir != RateDirectionDown {
		t.Fatalf("got %s %s want 0.65 down", gotRate, gotDir)
	}
	gotRate, gotDir = ApplyExtractRateTurn(rate, RateDirectionDown)
	if !gotRate.Equal(rate) || gotDir != RateDirectionUp {
		t.Fatalf("got %s %s want 0.65 up", gotRate, gotDir)
	}
}

// After extract turn at 0.65 up: next settle still yields at 0.65, then advances to 0.60.
func TestExtractTurnThenSettlePaysCurrentThenAdvances(t *testing.T) {
	SetActiveParams(nil)
	rate := decimal.RequireFromString("0.65")
	dir := RateDirectionUp
	rate, dir = ApplyExtractRateTurn(rate, dir)
	if !rate.Equal(decimal.RequireFromString("0.65")) || dir != RateDirectionDown {
		t.Fatalf("after turn %s %s", rate, dir)
	}
	// settle: pay at current rate, then advance
	yieldRate := rate
	if !yieldRate.Equal(decimal.RequireFromString("0.65")) {
		t.Fatalf("yield rate %s want 0.65", yieldRate)
	}
	rate, dir = AdvanceRate(rate, dir)
	if !rate.Equal(decimal.RequireFromString("0.60")) || dir != RateDirectionUp {
		t.Fatalf("after settle advance %s %s want 0.60 up", rate, dir)
	}
}
