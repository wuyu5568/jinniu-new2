package biz

import "github.com/shopspring/decimal"

const (
	MinPeerLevel = 3
	PeerPoolRate = "0.10"
)

// LevelFromVolume returns V1–V9 (1–9) or 0 when below V1.
func LevelFromVolume(smallAreaVolume decimal.Decimal) int {
	if !smallAreaVolume.IsPositive() {
		return 0
	}
	best := 0
	for _, t := range GetActiveParams().CommunityTiers {
		minVol := decimal.RequireFromString(t.MinVolume)
		if smallAreaVolume.GreaterThanOrEqual(minVol) && t.Level > best {
			best = t.Level
		}
	}
	return best
}

// RateForLevel returns the community base rate fraction for a V level (0 if none).
func RateForLevel(level int) decimal.Decimal {
	if level <= 0 {
		return decimal.Zero
	}
	for _, t := range GetActiveParams().CommunityTiers {
		if t.Level == level {
			return decimal.RequireFromString(t.Rate)
		}
	}
	return decimal.Zero
}

// PeerEligible reports whether a level may receive 平级 (same-level) reward.
func PeerEligible(level int) bool {
	min := GetActiveParams().MinPeerLevel
	if min <= 0 {
		min = MinPeerLevel
	}
	return level >= min
}

// SmallAreaVolume is 小区业绩: sum of leg volumes excluding the largest leg.
func SmallAreaVolume(legVolumes []decimal.Decimal) decimal.Decimal {
	maxIdx := MaxLegIndex(legVolumes)
	if maxIdx < 0 || len(legVolumes) <= 1 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for i, v := range legVolumes {
		if i == maxIdx {
			continue
		}
		sum = sum.Add(v)
	}
	return sum
}

// MaxLegIndex returns the index of the largest leg volume (first on ties).
func MaxLegIndex(legVolumes []decimal.Decimal) int {
	if len(legVolumes) == 0 {
		return -1
	}
	maxIdx := 0
	for i := 1; i < len(legVolumes); i++ {
		if legVolumes[i].GreaterThan(legVolumes[maxIdx]) {
			maxIdx = i
		}
	}
	return maxIdx
}

// LegBlocked reports whether any listed level ≥ selfLevel (legacy whole-leg check).
// Prefer underCommunityBreak for 社区基础奖结算.
func LegBlocked(selfLevel int, downlineLevels []int) bool {
	if selfLevel <= 0 {
		return false
	}
	for _, lv := range downlineLevels {
		if lv >= selfLevel {
			return true
		}
	}
	return false
}

// underCommunityBreak reports 断档子树：路径上出现严格高于领取人的 V 级节点。
// 同级不断档（同级本人改发平级；其下级仍可计级差）。更高档节点及其下级静态不计级差、不发平级。
func underCommunityBreak(node, claimant uint64, claimantLevel int, parent map[uint64]uint64, levels map[uint64]int) bool {
	if claimantLevel <= 0 {
		return false
	}
	cur := node
	for cur != claimant {
		if levels[cur] > claimantLevel {
			return true
		}
		p, ok := parent[cur]
		if !ok {
			return false
		}
		cur = p
	}
	return false
}

// SubtreeVolume sums personal volumes of root and all descendants.
func SubtreeVolume(root uint64, children map[uint64][]uint64, personal map[uint64]decimal.Decimal) decimal.Decimal {
	sum := personal[root]
	stack := append([]uint64(nil), children[root]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		sum = sum.Add(personal[n])
		stack = append(stack, children[n]...)
	}
	return sum
}

// CollectSubtreeIDs returns root plus all descendants.
func CollectSubtreeIDs(root uint64, children map[uint64][]uint64) []uint64 {
	out := []uint64{root}
	stack := append([]uint64(nil), children[root]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, n)
		stack = append(stack, children[n]...)
	}
	return out
}

// CommunityRewardSplit is 级差 + 小区同级平级（本人静态×比例） for one claimant.
type CommunityRewardSplit struct {
	Base decimal.Decimal
	Peer decimal.Decimal
}

const (
	CommunityLineBase = "base"
	CommunityLinePeer = "peer"
)

// StaticPiece is one order's static yield within a settle run (for ledger source buy/rate).
type StaticPiece struct {
	UserID      uint64
	OrderID     uint64
	Yield       decimal.Decimal
	BuyAmount   decimal.Decimal
	RatePercent decimal.Decimal // e.g. 0.75
}

// CommunityRewardLine is one per-source community / peer credit (for ledger split).
type CommunityRewardLine struct {
	SourceUserID   uint64
	SourceOrderID  uint64
	Kind           string // CommunityLineBase | CommunityLinePeer
	Amount         decimal.Decimal
	SourceStatic   decimal.Decimal
	SourceBuy      decimal.Decimal
	SourceRatePct  decimal.Decimal // e.g. 0.75
	GapRate        decimal.Decimal // base: differential fraction; peer: peer pool rate
}

// CalcCommunityBase computes 社区基础奖 (级差 only) for claimant.
func CalcCommunityBase(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) decimal.Decimal {
	return CalcCommunityRewards(claimant, children, parent, levels, personal, todayStatic).Base
}

// CalcCommunityRewards computes 级差 + 小区同级平级 for claimant.
func CalcCommunityRewards(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) CommunityRewardSplit {
	out := CommunityRewardSplit{}
	for _, line := range ListCommunityRewardLines(claimant, children, parent, levels, personal, todayStatic) {
		switch line.Kind {
		case CommunityLineBase:
			out.Base = out.Base.Add(line.Amount)
		case CommunityLinePeer:
			out.Peer = out.Peer.Add(line.Amount)
		}
	}
	out.Base = out.Base.Round(8)
	out.Peer = out.Peer.Round(8)
	return out
}

// ListCommunityRewardLines returns per-source 级差 / 平级 lines (user-aggregated static).
func ListCommunityRewardLines(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) []CommunityRewardLine {
	pieces := make(map[uint64][]StaticPiece, len(todayStatic))
	for uid, s := range todayStatic {
		if s.IsPositive() {
			pieces[uid] = []StaticPiece{{UserID: uid, Yield: s}}
		}
	}
	return ListCommunityRewardLinesFromPieces(claimant, children, parent, levels, personal, pieces)
}

// ListCommunityRewardLinesFromPieces returns 级差 / 平级 lines per source order static piece.
// 同级且双方 ≥ min_peer_level：不跟该人算级差，改发其本人当日静态 × peer_pool_rate；
// 其下级仍计级差，对照比例为路径上最靠近领取人的有等级档（跳过与领取人同级）。更高档仍整段断档。
func ListCommunityRewardLinesFromPieces(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	pieces map[uint64][]StaticPiece,
) []CommunityRewardLine {
	selfLevel := levels[claimant]
	ru := RateForLevel(selfLevel)
	if !ru.IsPositive() {
		return nil
	}

	legs := children[claimant]
	if len(legs) == 0 {
		return nil
	}
	legVols := make([]decimal.Decimal, len(legs))
	for i, leg := range legs {
		legVols[i] = SubtreeVolume(leg, children, personal)
	}
	maxIdx := MaxLegIndex(legVols)
	peerRate := GetActiveParams().peerPoolRateDec()
	claimantPeerOK := PeerEligible(selfLevel)

	var lines []CommunityRewardLine
	for i, legRoot := range legs {
		if i == maxIdx {
			continue
		}
		ids := CollectSubtreeIDs(legRoot, children)
		for _, x := range ids {
			if underCommunityBreak(x, claimant, selfLevel, parent, levels) {
				continue
			}
			for _, piece := range pieces[x] {
				s := piece.Yield
				if !s.IsPositive() {
					continue
				}
				lv := levels[x]
				if lv == selfLevel && claimantPeerOK && PeerEligible(lv) {
					amt := s.Mul(peerRate).Round(8)
					if amt.IsPositive() {
						lines = append(lines, CommunityRewardLine{
							SourceUserID:  x,
							SourceOrderID: piece.OrderID,
							Kind:          CommunityLinePeer,
							Amount:        amt,
							SourceStatic:  s,
							SourceBuy:     piece.BuyAmount,
							SourceRatePct: piece.RatePercent,
							GapRate:       peerRate,
						})
					}
					continue
				}
				govRate := nearestGradedRateBelowClaimant(x, claimant, selfLevel, parent, levels)
				diff := ru.Sub(govRate)
				if diff.IsPositive() {
					amt := s.Mul(diff).Round(8)
					if amt.IsPositive() {
						lines = append(lines, CommunityRewardLine{
							SourceUserID:  x,
							SourceOrderID: piece.OrderID,
							Kind:          CommunityLineBase,
							Amount:        amt,
							SourceStatic:  s,
							SourceBuy:     piece.BuyAmount,
							SourceRatePct: piece.RatePercent,
							GapRate:       diff,
						})
					}
				}
			}
		}
	}
	return lines
}

// nearestGradedRateBelowClaimant walks from node up to claimant and returns the rate of the
// graded node nearest to the claimant (last non-peer-same-level grade on the path).
// Example: A(V9)-B(V5)-C(V3)-D → for A on D's static, gov is B's 40% (not C's 30%).
func nearestGradedRateBelowClaimant(node, claimant uint64, claimantLevel int, parent map[uint64]uint64, levels map[uint64]int) decimal.Decimal {
	cur := node
	last := decimal.Zero
	for cur != claimant {
		if lv := levels[cur]; lv > 0 && lv != claimantLevel {
			last = RateForLevel(lv)
		}
		p, ok := parent[cur]
		if !ok {
			return last
		}
		if p == claimant {
			return last
		}
		cur = p
	}
	return last
}
