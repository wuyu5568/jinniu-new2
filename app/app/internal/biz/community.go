package biz

import "github.com/shopspring/decimal"

const (
	MinPeerLevel    = 3
	PeerPoolRate    = "0.10"
	MaxPeerPerChain = 5
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

// PeerEligible reports whether a level joins 平级分红.
func PeerEligible(level int) bool {
	min := GetActiveParams().MinPeerLevel
	if min <= 0 {
		min = MinPeerLevel
	}
	return level >= min
}

// SmallAreaVolume is 小区业绩: sum of leg volumes excluding the largest leg.
func SmallAreaVolume(legVolumes []decimal.Decimal) decimal.Decimal {
	if len(legVolumes) <= 1 {
		return decimal.Zero
	}
	maxIdx := 0
	for i := 1; i < len(legVolumes); i++ {
		if legVolumes[i].GreaterThan(legVolumes[maxIdx]) {
			maxIdx = i
		}
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

// underCommunityBreak reports 断档子树：node 自身或到 claimant 之前的祖先上出现 V ≥ claimant 等级。
// 断档节点及其下级静态不发给 claimant；同腿其他分支仍可发。
func underCommunityBreak(node, claimant uint64, claimantLevel int, parent map[uint64]uint64, levels map[uint64]int) bool {
	if claimantLevel <= 0 {
		return false
	}
	cur := node
	for cur != claimant {
		if levels[cur] >= claimantLevel {
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

// CalcCommunityBase computes 社区基础奖 (级差) for claimant.
func CalcCommunityBase(
	claimant uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) decimal.Decimal {
	selfLevel := levels[claimant]
	ru := RateForLevel(selfLevel)
	if !ru.IsPositive() {
		return decimal.Zero
	}

	legs := children[claimant]
	if len(legs) == 0 {
		return decimal.Zero
	}
	legVols := make([]decimal.Decimal, len(legs))
	for i, leg := range legs {
		legVols[i] = SubtreeVolume(leg, children, personal)
	}
	maxIdx := MaxLegIndex(legVols)

	total := decimal.Zero
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
			govRate := deepestGradedRateBelow(x, claimant, parent, levels)
			diff := ru.Sub(govRate)
			if diff.IsPositive() {
				total = total.Add(s.Mul(diff))
			}
		}
	}
	return total.Round(8)
}

func deepestGradedRateBelow(node, claimant uint64, parent map[uint64]uint64, levels map[uint64]int) decimal.Decimal {
	cur := node
	for {
		if lv := levels[cur]; lv > 0 {
			return RateForLevel(lv)
		}
		p, ok := parent[cur]
		if !ok || p == claimant {
			return decimal.Zero
		}
		cur = p
	}
}

func legRootToward(claimant, source uint64, parent map[uint64]uint64) uint64 {
	cur := source
	for {
		p, ok := parent[cur]
		if !ok {
			return 0
		}
		if p == claimant {
			return cur
		}
		cur = p
	}
}

func sourceInSmallArea(
	claimant, source uint64,
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	personal map[uint64]decimal.Decimal,
) bool {
	legs := children[claimant]
	if len(legs) < 2 {
		return false
	}
	legRoot := legRootToward(claimant, source, parent)
	if legRoot == 0 {
		return false
	}
	legVols := make([]decimal.Decimal, len(legs))
	legIdx := -1
	for i, leg := range legs {
		legVols[i] = SubtreeVolume(leg, children, personal)
		if leg == legRoot {
			legIdx = i
		}
	}
	if legIdx < 0 {
		return false
	}
	return legIdx != MaxLegIndex(legVols)
}

// CalcPeerRewards returns 上行链平级分红 keyed by recipient user id.
func CalcPeerRewards(
	children map[uint64][]uint64,
	parent map[uint64]uint64,
	levels map[uint64]int,
	personal map[uint64]decimal.Decimal,
	todayStatic map[uint64]decimal.Decimal,
) map[uint64]decimal.Decimal {
	rate := GetActiveParams().peerPoolRateDec()
	out := map[uint64]decimal.Decimal{}
	for source, static := range todayStatic {
		if !static.IsPositive() {
			continue
		}
		lastLevel := 0
		peerCount := 0
		cur := source
		for {
			p, ok := parent[cur]
			if !ok {
				break
			}
			if !sourceInSmallArea(p, source, children, parent, personal) {
				cur = p
				continue
			}
			lv := levels[p]
			if lv <= 0 {
				cur = p
				continue
			}
			if lv < lastLevel {
				cur = p
				continue
			}
			if lv == lastLevel {
				if PeerEligible(lv) && peerCount < MaxPeerPerChain {
					amt := static.Mul(rate).Round(8)
					if amt.IsPositive() {
						out[p] = out[p].Add(amt)
						peerCount++
					}
				}
				cur = p
				continue
			}
			lastLevel = lv
			cur = p
		}
	}
	return out
}
