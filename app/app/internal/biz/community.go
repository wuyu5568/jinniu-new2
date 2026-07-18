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
// 同级且双方 ≥ min_peer_level：不跟该人算级差，改发其本人当日静态 × peer_pool_rate；
// 其下级仍计级差，对照比例为路径上最靠近领取人的有等级档（跳过与领取人同级）。更高档仍整段断档。
func CalcCommunityRewards(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) CommunityRewardSplit {
	selfLevel := levels[claimant]
	ru := RateForLevel(selfLevel)
	out := CommunityRewardSplit{}
	if !ru.IsPositive() {
		return out
	}

	legs := children[claimant]
	if len(legs) == 0 {
		return out
	}
	legVols := make([]decimal.Decimal, len(legs))
	for i, leg := range legs {
		legVols[i] = SubtreeVolume(leg, children, personal)
	}
	maxIdx := MaxLegIndex(legVols)
	peerRate := GetActiveParams().peerPoolRateDec()
	claimantPeerOK := PeerEligible(selfLevel)

	base := decimal.Zero
	peer := decimal.Zero
	for i, legRoot := range legs {
		if i == maxIdx {
			continue
		}
		ids := CollectSubtreeIDs(legRoot, children)
		for _, x := range ids {
			if underCommunityBreak(x, claimant, selfLevel, parent, levels) {
				continue
			}
			s := todayStatic[x]
			if !s.IsPositive() {
				continue
			}
			lv := levels[x]
			if lv == selfLevel && claimantPeerOK && PeerEligible(lv) {
				peer = peer.Add(s.Mul(peerRate))
				continue
			}
			govRate := nearestGradedRateBelowClaimant(x, claimant, selfLevel, parent, levels)
			diff := ru.Sub(govRate)
			if diff.IsPositive() {
				base = base.Add(s.Mul(diff))
			}
		}
	}
	out.Base = base.Round(8)
	out.Peer = peer.Round(8)
	return out
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
