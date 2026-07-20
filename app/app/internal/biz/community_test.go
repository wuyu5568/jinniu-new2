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
	parent := map[uint64]uint64{3: 1, 4: 3, 5: 3, 6: 5}
	levels := map[uint64]int{1: 5, 3: 0, 4: 0, 5: 6, 6: 0}
	if underCommunityBreak(4, 1, 5, parent, levels) {
		t.Fatal("sibling of break should not be under break")
	}
	if underCommunityBreak(5, 1, 5, parent, levels) {
		t.Fatal("higher node itself should not break (normal gap applies)")
	}
	if !underCommunityBreak(6, 1, 5, parent, levels) {
		t.Fatal("under higher break node")
	}
	if underCommunityBreak(3, 1, 5, parent, levels) {
		t.Fatal("ancestor of break should not be under break")
	}
	levelsSame := map[uint64]int{1: 5, 3: 0, 4: 0, 5: 5, 6: 0}
	if underCommunityBreak(5, 1, 5, parent, levelsSame) {
		t.Fatal("same-level node itself should not break")
	}
	if !underCommunityBreak(6, 1, 5, parent, levelsSame) {
		t.Fatal("under same-level peer should break")
	}
}

func TestCalcCommunityBase_UngradedFullRate(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3}
	levels := map[uint64]int{1: 3, 2: 0, 3: 0, 4: 0}
	today := map[uint64]decimal.Decimal{4: decimal.RequireFromString("100")}
	got := CalcCommunityBase(1, children, parent, levels, today)
	want := decimal.RequireFromString("30")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCalcCommunityBase_HigherBreakSubtreeOnly(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {4, 5}, 5: {6}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3, 5: 3, 6: 5}
	levels := map[uint64]int{1: 5, 2: 0, 3: 0, 4: 0, 5: 6, 6: 0}
	today := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("50"),
		4: decimal.RequireFromString("100"),
		5: decimal.RequireFromString("50"),
		6: decimal.RequireFromString("30"),
	}
	got := CalcCommunityBase(1, children, parent, levels, today)
	// 直推2:20；4中间无级对照0→40；5本人中间无级对照0→20；6在V6下断档0
	want := decimal.RequireFromString("80")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCalcCommunityBase_SingleLegCounts(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2}, 2: {3}}
	parent := map[uint64]uint64{2: 1, 3: 2}
	levels := map[uint64]int{1: 3, 2: 0, 3: 0}
	today := map[uint64]decimal.Decimal{3: decimal.RequireFromString("100")}
	got := CalcCommunityBase(1, children, parent, levels, today)
	want := decimal.RequireFromString("30")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

// A(V3)-B(V3)-C: 直推 B 全额级差 30；C 在同级 B 下对 A 断档为 0；A 平级=B 社区基础奖×10%。
func TestComputeAll_SameLevelPeerFromCommunityBase(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 1, 4: 3}
	levels := map[uint64]int{1: 3, 2: 0, 3: 3, 4: 0}
	today := map[uint64]decimal.Decimal{
		3: decimal.RequireFromString("100"),
		4: decimal.RequireFromString("100"),
	}
	all := ComputeAllCommunityRewards([]uint64{1, 2, 3, 4}, children, parent, levels, today)
	if !all[3].Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("B base got %s want 30", all[3].Base)
	}
	if !all[1].Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("A base got %s want 30 (direct full rate)", all[1].Base)
	}
	if !all[1].Peer.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("A peer got %s want 3", all[1].Peer)
	}
}

func TestSameLevelBlocksSubtreeButNotSelf(t *testing.T) {
	SetActiveParams(nil)
	// A(V9)-B(V5)-C(V9)-D(0): 直推 B 全额 60；C 本人正常级差 20；D 断档 0。
	children := map[uint64][]uint64{1: {2}, 2: {3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 2, 4: 3}
	levels := map[uint64]int{1: 9, 2: 5, 3: 9, 4: 0}
	today := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("100"),
		3: decimal.RequireFromString("100"),
		4: decimal.RequireFromString("100"),
	}
	got := CalcCommunityBase(1, children, parent, levels, today)
	want := decimal.RequireFromString("80")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

// A(V9)-B(V5)-C(V3)-D(V0): 对照取中间最高级（此处与「最靠近领取人」结果相同）。
func TestCalcCommunityRewards_MaxGradeBetween(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {6, 4}, 4: {7, 5}}
	parent := map[uint64]uint64{2: 1, 3: 1, 6: 3, 4: 3, 7: 4, 5: 4}
	levels := map[uint64]int{1: 9, 2: 0, 3: 5, 6: 0, 4: 3, 7: 0, 5: 0}
	today := map[uint64]decimal.Decimal{5: decimal.RequireFromString("100")}

	gotC := CalcCommunityRewards(4, children, parent, levels, today)
	if !gotC.Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("C got base=%s want 30", gotC.Base)
	}
	gotB := CalcCommunityRewards(3, children, parent, levels, today)
	if !gotB.Base.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("B got base=%s want 10", gotB.Base)
	}
	gotA := CalcCommunityRewards(1, children, parent, levels, today)
	if !gotA.Base.Equal(decimal.RequireFromString("20")) {
		t.Fatalf("A got base=%s want 20", gotA.Base)
	}
}

// A(V8)-B(V3)-C(V5)-D(V9)-E(V1): D 对照取中间最高 C=40%，级差 15%；E 断档。
func TestCalcCommunityRewards_MaxBetweenIgnoresSourceHigher(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2}, 2: {3}, 3: {4}, 4: {5}}
	parent := map[uint64]uint64{2: 1, 3: 2, 4: 3, 5: 4}
	levels := map[uint64]int{1: 8, 2: 3, 3: 5, 4: 9, 5: 1}
	today := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("100"),
		3: decimal.RequireFromString("100"),
		4: decimal.RequireFromString("100"),
		5: decimal.RequireFromString("100"),
	}
	got := CalcCommunityBase(1, children, parent, levels, today)
	// B直推全额55；C中间B→25；D中间max(B,C)=C→15；E断档0 → 95
	want := decimal.RequireFromString("95")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

// A(V9)-B(V3)-C(V0): B 30%, A 30%.
func TestCalcCommunityRewards_V9V3V0(t *testing.T) {
	SetActiveParams(nil)
	children := map[uint64][]uint64{1: {2, 3}, 3: {6, 4}}
	parent := map[uint64]uint64{2: 1, 3: 1, 6: 3, 4: 3}
	levels := map[uint64]int{1: 9, 2: 0, 3: 3, 6: 0, 4: 0}
	today := map[uint64]decimal.Decimal{4: decimal.RequireFromString("100")}
	gotB := CalcCommunityRewards(3, children, parent, levels, today)
	if !gotB.Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("B got %s want 30", gotB.Base)
	}
	gotA := CalcCommunityRewards(1, children, parent, levels, today)
	if !gotA.Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("A got %s want 30", gotA.Base)
	}
}

func TestPeerDoesNotCascadeOnPeer(t *testing.T) {
	SetActiveParams(nil)
	// A(V3)-B(V3)-C(V3)-D(0): C 从 D 拿级差；B 从 C 拿平级；A 不因 B 的平级再拿平级（B 无社区基础奖时 A 平级为 0）。
	children := map[uint64][]uint64{1: {2}, 2: {3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 2, 4: 3}
	levels := map[uint64]int{1: 3, 2: 3, 3: 3, 4: 0}
	today := map[uint64]decimal.Decimal{4: decimal.RequireFromString("100")}
	all := ComputeAllCommunityRewards([]uint64{1, 2, 3, 4}, children, parent, levels, today)
	if !all[3].Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("C base %s", all[3].Base)
	}
	if !all[2].Base.IsZero() {
		t.Fatalf("B base %s want 0 (C has no static)", all[2].Base)
	}
	if !all[2].Peer.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("B peer %s want 3", all[2].Peer)
	}
	if !all[1].Peer.IsZero() {
		t.Fatalf("A peer %s want 0 (B has no community-base to peer)", all[1].Peer)
	}
}

func TestPeerFromDirectChildCommunityBase(t *testing.T) {
	SetActiveParams(nil)
	// A-B-C-D，C 有静态使 B 有社区基础奖，则 A 可拿 B 的平级（基数不含平级本身）。
	children := map[uint64][]uint64{1: {2}, 2: {3}, 3: {4}}
	parent := map[uint64]uint64{2: 1, 3: 2, 4: 3}
	levels := map[uint64]int{1: 3, 2: 3, 3: 3, 4: 0}
	today := map[uint64]decimal.Decimal{
		3: decimal.RequireFromString("100"),
		4: decimal.RequireFromString("100"),
	}
	all := ComputeAllCommunityRewards([]uint64{1, 2, 3, 4}, children, parent, levels, today)
	if !all[2].Base.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("B base %s want 30", all[2].Base)
	}
	if !all[2].Peer.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("B peer %s want 3", all[2].Peer)
	}
	if !all[1].Peer.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("A peer %s want 3 (from B base 30, not from B peer)", all[1].Peer)
	}
}

func TestDirectFullRateEvenIfHigherLevel(t *testing.T) {
	SetActiveParams(nil)
	// A(V5) 直推 B(V9)：A 仍拿 B 静态 ×40%；B 下级仍断档。
	children := map[uint64][]uint64{1: {2}, 2: {3}}
	parent := map[uint64]uint64{2: 1, 3: 2}
	levels := map[uint64]int{1: 5, 2: 9, 3: 0}
	today := map[uint64]decimal.Decimal{
		2: decimal.RequireFromString("100"),
		3: decimal.RequireFromString("100"),
	}
	got := CalcCommunityBase(1, children, parent, levels, today)
	want := decimal.RequireFromString("40")
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestPeerDirectParentLevelLE(t *testing.T) {
	SetActiveParams(nil)
	// A(V5)-B(V9)-C(0): B 拿 C 社区基础奖；A 因 5≤9 拿 B 的平级；A 也拿 B 直推全额级差。
	children := map[uint64][]uint64{1: {2}, 2: {3}}
	parent := map[uint64]uint64{2: 1, 3: 2}
	levels := map[uint64]int{1: 5, 2: 9, 3: 0}
	today := map[uint64]decimal.Decimal{3: decimal.RequireFromString("100")}
	all := ComputeAllCommunityRewards([]uint64{1, 2, 3}, children, parent, levels, today)
	if !all[2].Base.Equal(decimal.RequireFromString("60")) {
		t.Fatalf("B base %s want 60", all[2].Base)
	}
	if !all[1].Base.IsZero() {
		t.Fatalf("A base %s want 0 (no direct static)", all[1].Base)
	}
	if !all[1].Peer.Equal(decimal.RequireFromString("6")) {
		t.Fatalf("A peer %s want 6", all[1].Peer)
	}
}

func TestPeerSkippedWhenParentHigher(t *testing.T) {
	SetActiveParams(nil)
	// A(V9)-B(V5)-C(0): B 有社区基础奖，但 A 等级 > B，不发平级给 A。
	children := map[uint64][]uint64{1: {2}, 2: {3}}
	parent := map[uint64]uint64{2: 1, 3: 2}
	levels := map[uint64]int{1: 9, 2: 5, 3: 0}
	today := map[uint64]decimal.Decimal{3: decimal.RequireFromString("100")}
	all := ComputeAllCommunityRewards([]uint64{1, 2, 3}, children, parent, levels, today)
	if !all[2].Base.Equal(decimal.RequireFromString("40")) {
		t.Fatalf("B base %s want 40", all[2].Base)
	}
	if !all[1].Peer.IsZero() {
		t.Fatalf("A peer %s want 0 (parent higher)", all[1].Peer)
	}
}

func TestNearestSameLevelUpline(t *testing.T) {
	parent := map[uint64]uint64{2: 1, 3: 2, 4: 3}
	levels := map[uint64]int{1: 5, 2: 3, 3: 3, 4: 0}
	if got := NearestSameLevelUpline(3, parent, levels); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
	if got := NearestSameLevelUpline(4, parent, levels); got != 0 {
		t.Fatalf("ungraded got %d want 0", got)
	}
}
