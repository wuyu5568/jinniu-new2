package biz

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestLevelFromVolume(t *testing.T) {
	SetActiveParams(nil)
	cases := []struct {
		vol  string
		want int
	}{
		{"0", 0},
		{"4999.99", 0},
		{"5000", 1},
		{"20000", 2},
		{"80000", 3},
		{"250000", 4},
		{"500000", 5},
		{"1500000", 6},
		{"5000000", 7},
		{"10000000", 8},
		{"20000000", 9},
	}
	for _, tc := range cases {
		got := LevelFromVolume(decimal.RequireFromString(tc.vol))
		if got != tc.want {
			t.Fatalf("vol=%s got V%d want V%d", tc.vol, got, tc.want)
		}
	}
}

func TestSmallAreaVolume(t *testing.T) {
	legs := []decimal.Decimal{
		decimal.RequireFromString("100"),
		decimal.RequireFromString("500"),
		decimal.RequireFromString("200"),
	}
	got := SmallAreaVolume(legs)
	want := decimal.RequireFromString("300")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestLegBlocked(t *testing.T) {
	if !LegBlocked(3, []int{1, 3, 2}) {
		t.Fatal("same level should block")
	}
	if LegBlocked(5, []int{1, 3, 4}) {
		t.Fatal("all lower should not block")
	}
}

func TestUnderCommunityBreak(t *testing.T) {
	// 1(U) - 3 - 4(V0), 5(V5)-6
	parent := map[uint64]uint64{3: 1, 4: 3, 5: 3, 6: 5}
	levels := map[uint64]int{1: 5, 3: 0, 4: 0, 5: 5, 6: 0}
	if underCommunityBreak(4, 1, 5, parent, levels) {
		t.Fatal("sibling of break should not be under break")
	}
	if !underCommunityBreak(5, 1, 5, parent, levels) {
		t.Fatal("break node itself")
	}
	if !underCommunityBreak(6, 1, 5, parent, levels) {
		t.Fatal("under break node")
	}
	if underCommunityBreak(3, 1, 5, parent, levels) {
		t.Fatal("ancestor of break should not be under break")
	}
}

func TestCalcCommunityBase_UngradedFullRate(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3}
	levels := map[uint64]int{1: 3, 2: 0, 3: 0, 4: 0}
	personal := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("100000"),
		3: decimal.RequireFromString("1000"),
		4: decimal.RequireFromString("4000"),
	}
	today := map[uint64]decimal.Decimal{4: decimal.RequireFromString("100")}
	got := CalcCommunityBase(1, children, parent, levels, personal, today)
	want := decimal.RequireFromString("30")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCalcCommunityBase_BreakSubtreeOnly(t *testing.T) {
	SetActiveParams(nil)
	// 1 V5: max leg 2; small leg 3 with 4(V0) and 5(V5)->6
	children := map[uint64][]uint64{1: {2, 3}, 3: {4, 5}, 5: {6}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3, 5: 3, 6: 5}
	levels := map[uint64]int{1: 5, 2: 0, 3: 0, 4: 0, 5: 5, 6: 0}
	personal := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("1000000"),
		3: decimal.RequireFromString("100"),
		4: decimal.RequireFromString("1000"),
		5: decimal.RequireFromString("1000"),
		6: decimal.RequireFromString("1000"),
	}
	today := map[uint64]decimal.Decimal{
		4: decimal.RequireFromString("100"),
		5: decimal.RequireFromString("50"),
		6: decimal.RequireFromString("30"),
	}
	got := CalcCommunityBase(1, children, parent, levels, personal, today)
	// only node 4: 100 * 0.40 = 40; 5 and 6 under break skipped
	want := decimal.RequireFromString("40")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}
