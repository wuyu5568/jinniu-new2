package biz

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestUnlockedGenerations(t *testing.T) {
	SetActiveParams(nil)
	cases := []struct {
		direct int
		want   int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{10, 10},
		{15, 10},
	}
	for _, tc := range cases {
		if got := UnlockedGenerations(tc.direct); got != tc.want {
			t.Fatalf("direct=%d got %d want %d", tc.direct, got, tc.want)
		}
	}
}

func TestCanEarnGeneration(t *testing.T) {
	if !CanEarnGeneration(3, 1) || !CanEarnGeneration(3, 3) {
		t.Fatal("expected gen1-3 unlocked")
	}
	if CanEarnGeneration(3, 4) {
		t.Fatal("gen4 should be locked")
	}
}

func TestCalcGenerationReward(t *testing.T) {
	SetActiveParams(nil)
	got := CalcGenerationReward(decimal.RequireFromString("0.60"))
	want := decimal.RequireFromString("0.03")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}
