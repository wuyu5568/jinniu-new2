package biz

import "github.com/shopspring/decimal"

const MaxGenerationDepth = 10

func maxGenerationDepth() int {
	d := GetActiveParams().MaxGenerationDepth
	if d <= 0 {
		return MaxGenerationDepth
	}
	return d
}

// UnlockedGenerations returns how many generations a user can earn from.
func UnlockedGenerations(directCount int) int {
	max := maxGenerationDepth()
	if directCount <= 0 {
		return 0
	}
	if directCount >= max {
		return max
	}
	return directCount
}

// CanEarnGeneration reports whether an ancestor at depth (1=direct) is unlocked.
func CanEarnGeneration(directCount, depth int) bool {
	max := maxGenerationDepth()
	if depth < 1 || depth > max {
		return false
	}
	return depth <= UnlockedGenerations(directCount)
}

// CalcGenerationReward returns generation_rate of the subordinate's static yield.
func CalcGenerationReward(staticYield decimal.Decimal) decimal.Decimal {
	if !staticYield.IsPositive() {
		return decimal.Zero
	}
	return staticYield.Mul(GetActiveParams().generationRateDec()).Round(8)
}
