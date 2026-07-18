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
	// 1(V5) - 3 - 4(V0), 5(V6)-6  — only strictly higher breaks
	parent := map[uint64]uint64{3: 1, 4: 3, 5: 3, 6: 5}
	levels := map[uint64]int{1: 5, 3: 0, 4: 0, 5: 6, 6: 0}
	if underCommunityBreak(4, 1, 5, parent, levels) {
		t.Fatal("sibling of break should not be under break")
	}
	if !underCommunityBreak(5, 1, 5, parent, levels) {
		t.Fatal("higher break node itself")
	}
	if !underCommunityBreak(6, 1, 5, parent, levels) {
		t.Fatal("under higher break node")
	}
	if underCommunityBreak(3, 1, 5, parent, levels) {
		t.Fatal("ancestor of break should not be under break")
	}
	// same level does not break
	levelsSame := map[uint64]int{1: 5, 3: 0, 4: 0, 5: 5, 6: 0}
	if underCommunityBreak(5, 1, 5, parent, levelsSame) {
		t.Fatal("same level should not break")
	}
	if underCommunityBreak(6, 1, 5, parent, levelsSame) {
		t.Fatal("under same-level peer should not break")
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

func TestCalcCommunityBase_HigherBreakSubtreeOnly(t *testing.T) {
	SetActiveParams(nil)
	// 1 V5: max leg 2; small leg 3 with 4(V0) and 5(V6)->6
	children := map[uint64][]uint64{1: {2, 3}, 3: {4, 5}, 5: {6}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3, 5: 3, 6: 5}
	levels := map[uint64]int{1: 5, 2: 0, 3: 0, 4: 0, 5: 6, 6: 0}
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
	// only node 4: 100 * 0.40 = 40; 5 and 6 under higher break skipped
	want := decimal.RequireFromString("40")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

// A V3, max leg 2, small leg B(V3)->C(ungraded). Peer on B static; C still differential skipping B.
func TestCalcCommunityRewards_SameLevelPeerAndSkipGov(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3}
	levels := map[uint64]int{1: 3, 2: 0, 3: 3, 4: 0}
	personal := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("200000"),
		3: decimal.RequireFromString("1000"),
		4: decimal.RequireFromString("80000"),
	}
	today := map[uint64]decimal.Decimal{
		3: decimal.RequireFromString("100"),
		4: decimal.RequireFromString("100"),
	}
	got := CalcCommunityRewards(1, children, parent, levels, personal, today)
	// peer: 100 * 0.10 = 10; base: C 100 * 0.30 = 30 (skip B as gov)
	if !got.Peer.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("peer got %s want 10", got.Peer)
	}
	if !got.Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("base got %s want 30", got.Base)
	}
}

// A(V9)-B(V5)-C(V3)-D(V0): layered gap vs nearest grade below claimant.
func TestCalcCommunityRewards_NearestGradeBelowClaimant(t *testing.T) {
	SetActiveParams(nil)
	// Each of A/B/C needs ≥2 legs so the chain leg is not the sole (max) leg.
	// 1=A, 2=A bigleg, 3=B; 6=B bigleg, 4=C; 7=C bigleg, 5=D
	children := map[uint64][]uint64{1: {2, 3}, 3: {6, 4}, 4: {7, 5}}
	parent := map[uint64]uint64{2: 1, 3: 1, 6: 3, 4: 3, 7: 4, 5: 4}
	levels := map[uint64]int{1: 9, 2: 0, 3: 5, 6: 0, 4: 3, 7: 0, 5: 0}
	personal := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("5000000"),
		3: decimal.RequireFromString("100"),
		6: decimal.RequireFromString("200000"),
		4: decimal.RequireFromString("100"),
		7: decimal.RequireFromString("100000"),
		5: decimal.RequireFromString("1000"),
	}
	today := map[uint64]decimal.Decimal{5: decimal.RequireFromString("100")}

	// C(V3) on D: 30%
	gotC := CalcCommunityRewards(4, children, parent, levels, personal, today)
	if !gotC.Base.Equal(decimal.RequireFromString("30")) || !gotC.Peer.IsZero() {
		t.Fatalf("C got base=%s peer=%s want base=30", gotC.Base, gotC.Peer)
	}
	// B(V5) on D: 40%-30%=10%
	gotB := CalcCommunityRewards(3, children, parent, levels, personal, today)
	if !gotB.Base.Equal(decimal.RequireFromString("10")) || !gotB.Peer.IsZero() {
		t.Fatalf("B got base=%s peer=%s want base=10", gotB.Base, gotB.Peer)
	}
	// A(V9) on D: 60%-40%=20% (gov=B, not C)
	gotA := CalcCommunityRewards(1, children, parent, levels, personal, today)
	if !gotA.Base.Equal(decimal.RequireFromString("20")) || !gotA.Peer.IsZero() {
		t.Fatalf("A got base=%s peer=%s want base=20", gotA.Base, gotA.Peer)
	}
}

// A(V9)-B(V3)-C(V0): B 30%, A 30%.
func TestCalcCommunityRewards_V9V3V0(t *testing.T) {
	SetActiveParams(nil)
	// B needs a filler leg so C is in B's small area.
	children := map[uint64][]uint64{1: {2, 3}, 3: {6, 4}}
	parent := map[uint64]uint64{2: 1, 3: 1, 6: 3, 4: 3}
	levels := map[uint64]int{1: 9, 2: 0, 3: 3, 6: 0, 4: 0}
	personal := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("5000000"),
		3: decimal.RequireFromString("100"),
		6: decimal.RequireFromString("200000"),
		4: decimal.RequireFromString("80000"),
	}
	today := map[uint64]decimal.Decimal{4: decimal.RequireFromString("100")}
	gotB := CalcCommunityRewards(3, children, parent, levels, personal, today)
	if !gotB.Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("B got %s want 30", gotB.Base)
	}
	gotA := CalcCommunityRewards(1, children, parent, levels, personal, today)
	if !gotA.Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("A got %s want 30", gotA.Base)
	}
}
