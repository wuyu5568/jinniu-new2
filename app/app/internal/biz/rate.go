package biz

import (
	"github.com/shopspring/decimal"
)

const (
	RateDirectionUp   = "up"
	RateDirectionDown = "down"

	OrderStatusActive = "active"
	OrderStatusExited = "exited"
)

// MultiplierForAmount returns the exit multiplier for a subscription amount.
func MultiplierForAmount(amount decimal.Decimal) (decimal.Decimal, bool) {
	p := GetActiveParams()
	min := decimal.RequireFromString(p.MinSubscribeAmount)
	if amount.LessThan(min) {
		return decimal.Zero, false
	}
	for _, t := range p.MultiplierTiers {
		if t.Lt == "" {
			return decimal.RequireFromString(t.Multiplier), true
		}
		lt := decimal.RequireFromString(t.Lt)
		if amount.LessThan(lt) {
			return decimal.RequireFromString(t.Multiplier), true
		}
	}
	return decimal.Zero, false
}

// InitialRate returns the first-day static rate for a new order.
func InitialRate() (rate decimal.Decimal, direction string) {
	return GetActiveParams().rateMinDec(), RateDirectionUp
}

// AdvanceRate moves one day along the configured rate cycle.
func AdvanceRate(current decimal.Decimal, direction string) (decimal.Decimal, string) {
	p := GetActiveParams()
	rateMin := p.rateMinDec()
	rateMax := p.rateMaxDec()
	rateStep := p.rateStepDec()

	if direction != RateDirectionDown {
		direction = RateDirectionUp
	}
	if direction == RateDirectionUp {
		next := current.Add(rateStep)
		if next.GreaterThan(rateMax) {
			next = rateMax.Sub(rateStep)
			if next.LessThan(rateMin) {
				next = rateMin
			}
			return next, RateDirectionDown
		}
		if next.Equal(rateMax) {
			return rateMax, RateDirectionDown
		}
		return next, RateDirectionUp
	}
	next := current.Sub(rateStep)
	if next.LessThan(rateMin) {
		next = rateMin.Add(rateStep)
		if next.GreaterThan(rateMax) {
			next = rateMax
		}
		return next, RateDirectionUp
	}
	if next.Equal(rateMin) {
		return rateMin, RateDirectionUp
	}
	return next, RateDirectionDown
}

// ApplyExtractRateTurn applies withdraw rate turn from lastSettled (昨日结算利率).
// down / nil lastSettled → no change; lastSettled == rateMin → rewind to min/up;
// otherwise AdvanceRate(lastSettled, down). Caller skips when RateTurnPending is set.
func ApplyExtractRateTurn(direction string, lastSettled *decimal.Decimal) (rate decimal.Decimal, dir string, applied bool) {
	if direction != RateDirectionUp || lastSettled == nil {
		return decimal.Zero, direction, false
	}
	rateMin := GetActiveParams().rateMinDec()
	if lastSettled.Equal(rateMin) {
		return rateMin, RateDirectionUp, true
	}
	rate, dir = AdvanceRate(*lastSettled, RateDirectionDown)
	return rate, dir, true
}

// CalcStaticYield computes static yield: subscribeAmount * ratePercent / 100.
// Exit still uses exitTarget (= amount × multiplier); only the daily yield base is subscribe amount.
func CalcStaticYield(subscribeAmount, ratePercent decimal.Decimal) decimal.Decimal {
	return subscribeAmount.Mul(ratePercent).Div(decimal.NewFromInt(100)).Round(8)
}
