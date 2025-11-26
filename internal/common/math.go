/**
 * Copyright 2025-present Coinbase Global, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Fee Strategy
// ============================================================================

// FeeStrategy calculates percentage-based trading fees
type FeeStrategy struct {
	Percent decimal.Decimal // e.g., 0.001 for 0.1% (10 bps)
}

// NewFeeStrategy creates a new percentage-based fee strategy
func NewFeeStrategy(percent decimal.Decimal) *FeeStrategy {
	return &FeeStrategy{Percent: percent}
}

// CreateFeeStrategy creates a percentage-based fee strategy from configuration
func CreateFeeStrategy(feePercent string) (*FeeStrategy, error) {
	percent, err := decimal.NewFromString(feePercent)
	if err != nil {
		return nil, fmt.Errorf("invalid fee percent: %w", err)
	}
	if percent.IsNegative() {
		return nil, fmt.Errorf("fee percent cannot be negative")
	}
	return NewFeeStrategy(percent), nil
}

// ============================================================================
// Fee Calculations
// ============================================================================

// CalculateFee calculates the fee for a given quantity and price
func CalculateFee(qty, price, feePercent decimal.Decimal) decimal.Decimal {
	notional := qty.Mul(price)
	return notional.Mul(feePercent)
}

// CalculateFeeFromNotional calculates the fee from a notional value (qty * price)
// This is used for quote-denominated orders where we already know the total value
func CalculateFeeFromNotional(notional, feePercent decimal.Decimal) decimal.Decimal {
	return notional.Mul(feePercent)
}

// Compute calculates the fee for a given quantity and price (FeeStrategy method)
func (s *FeeStrategy) Compute(qty, price decimal.Decimal) decimal.Decimal {
	return CalculateFee(qty, price, s.Percent)
}

// ComputeFromNotional calculates the fee from a notional value (FeeStrategy method)
func (s *FeeStrategy) ComputeFromNotional(notional decimal.Decimal) decimal.Decimal {
	return CalculateFeeFromNotional(notional, s.Percent)
}

// ============================================================================
// Price Adjustments
// ============================================================================

// AdjustBidPrice reduces bid price to account for fee when user is selling
func AdjustBidPrice(price, qty, feePercent decimal.Decimal) decimal.Decimal {
	if qty.IsZero() {
		return price
	}

	fee := CalculateFee(qty, price, feePercent)
	notional := qty.Mul(price)
	adjustedNotional := notional.Sub(fee)

	return adjustedNotional.Div(qty)
}

// AdjustAskPrice increases ask price to account for fee when user is buying
func AdjustAskPrice(price, qty, feePercent decimal.Decimal) decimal.Decimal {
	if qty.IsZero() {
		return price
	}

	fee := CalculateFee(qty, price, feePercent)
	notional := qty.Mul(price)
	adjustedNotional := notional.Add(fee)

	return adjustedNotional.Div(qty)
}

// ============================================================================
// Order Preview Calculations
// ============================================================================

// CalculateTotalCost computes the total cost including all fees
func CalculateTotalCost(baseQty, executionPrice, primeFee, customFee decimal.Decimal) decimal.Decimal {
	return baseQty.Mul(executionPrice).Add(primeFee).Add(customFee)
}

// CalculateEffectivePrice computes the effective price per unit including all fees
func CalculateEffectivePrice(totalCost, baseQty decimal.Decimal) decimal.Decimal {
	if baseQty.IsZero() {
		return decimal.Zero
	}
	return totalCost.Div(baseQty)
}

// CalculateFeePercent converts a fee amount to a percentage of notional
func CalculateFeePercent(feeAmount, notional decimal.Decimal) decimal.Decimal {
	if notional.IsZero() {
		return decimal.Zero
	}
	return feeAmount.Div(notional).Mul(decimal.NewFromInt(100))
}

// CalculateNotional computes the notional value (quantity * price)
func CalculateNotional(qty, price decimal.Decimal) decimal.Decimal {
	return qty.Mul(price)
}

// ============================================================================
// RFQ Calculations
// ============================================================================

// CalculateRfqQuoteAmount computes the amount to send to Prime after deducting markup
func CalculateRfqQuoteAmount(userRequestedAmount, feeAmount decimal.Decimal) decimal.Decimal {
	return userRequestedAmount.Sub(feeAmount)
}

// CalculateRfqTotalCost computes the total cost for RFQ including fees
func CalculateRfqTotalCost(primeTotal, feeAmount decimal.Decimal) decimal.Decimal {
	return primeTotal.Add(feeAmount)
}

// CalculateRfqEffectivePrice computes effective price for RFQ orders
func CalculateRfqEffectivePrice(totalCost, quantity decimal.Decimal) decimal.Decimal {
	if quantity.IsZero() {
		return decimal.Zero
	}
	return totalCost.Div(quantity)
}

// ============================================================================
// Percentage Conversions
// ============================================================================

// ToPercentageDisplay converts a decimal to percentage for display (0.005 -> 0.5)
func ToPercentageDisplay(decimalValue decimal.Decimal) decimal.Decimal {
	return decimalValue.Mul(decimal.NewFromInt(100))
}

// FromPercentageDisplay converts a percentage to decimal (0.5 -> 0.005)
func FromPercentageDisplay(percentValue decimal.Decimal) decimal.Decimal {
	return percentValue.Div(decimal.NewFromInt(100))
}

// ============================================================================
// PriceAdjuster (wrapper for backward compatibility)
// ============================================================================

// PriceAdjuster applies fee strategy to market prices
type PriceAdjuster struct {
	FeeStrategy *FeeStrategy
}

// NewPriceAdjuster creates a new price adjuster with a fee strategy
func NewPriceAdjuster(feeStrategy *FeeStrategy) *PriceAdjuster {
	return &PriceAdjuster{
		FeeStrategy: feeStrategy,
	}
}

// AdjustBidPrice reduces bid price to account for fee when user is selling
func (a *PriceAdjuster) AdjustBidPrice(price, qty decimal.Decimal) decimal.Decimal {
	return AdjustBidPrice(price, qty, a.FeeStrategy.Percent)
}

// AdjustAskPrice increases ask price to account for fee when user is buying
func (a *PriceAdjuster) AdjustAskPrice(price, qty decimal.Decimal) decimal.Decimal {
	return AdjustAskPrice(price, qty, a.FeeStrategy.Percent)
}

// ComputeFee calculates the fee for a given quantity and price
func (a *PriceAdjuster) ComputeFee(qty, price decimal.Decimal) decimal.Decimal {
	return a.FeeStrategy.Compute(qty, price)
}
