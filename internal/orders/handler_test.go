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

package orders

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculateFeeSettlement_FullFill(t *testing.T) {
	handler := &DbOrderHandler{}

	// Scenario: User wants $10, we charge $0.05 fee, send $9.95 to Prime
	// Order fills 100% at expected price
	// Expected: We earned full $0.05, no rebate
	settlement := handler.calculateFeeSettlement(
		"0.00011718", // cumQty (filled quantity in BTC)
		"85036.73",   // avgPx (average price)
		"10",         // userRequestedAmount ($10)
		"0.05",       // markupAmount ($0.05 fee)
		"9.95",       // primeOrderAmount ($9.95 sent to Prime)
	)

	// Actual filled value = cumQty * avgPx = 0.00011718 * 85036.73 ≈ 9.96
	actualFilledDec, _ := decimal.NewFromString("0.00011718")
	avgPxDec, _ := decimal.NewFromString("85036.73")
	expectedFilledValue := actualFilledDec.Mul(avgPxDec)

	// Fee rate = 0.05 / 10 = 0.005 (0.5%)
	// Actual user cost = 9.96 / (1 - 0.005) = 9.96 / 0.995 ≈ 10.01
	// Actual earned fee = 10.01 * 0.005 ≈ 0.05
	// Rebate = 0.05 - 0.05 = 0

	if settlement.ActualFilledValue == "" {
		t.Error("ActualFilledValue should not be empty")
	}

	filledValue, err := decimal.NewFromString(settlement.ActualFilledValue)
	if err != nil {
		t.Fatalf("Failed to parse ActualFilledValue: %v", err)
	}

	// Should be close to expected filled value (allow small rounding difference)
	diff := filledValue.Sub(expectedFilledValue).Abs()
	if diff.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("ActualFilledValue = %s, want ~%s", settlement.ActualFilledValue, expectedFilledValue.String())
	}

	// Earned fee should be close to full markup
	earnedFee, err := decimal.NewFromString(settlement.ActualEarnedFee)
	if err != nil {
		t.Fatalf("Failed to parse ActualEarnedFee: %v", err)
	}
	markupDec, _ := decimal.NewFromString("0.05")
	earnedDiff := earnedFee.Sub(markupDec).Abs()
	if earnedDiff.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("ActualEarnedFee = %s, want ~0.05", settlement.ActualEarnedFee)
	}

	// Rebate should be ~0 (or very small)
	rebate, err := decimal.NewFromString(settlement.RebateAmount)
	if err != nil {
		t.Fatalf("Failed to parse RebateAmount: %v", err)
	}
	if rebate.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("RebateAmount = %s, want ~0", settlement.RebateAmount)
	}
}

func TestCalculateFeeSettlement_PartialFill(t *testing.T) {
	handler := &DbOrderHandler{}

	// Scenario: User wants $100, we charge $0.50 fee, send $99.50 to Prime
	// Order fills only 50% (gets $50 worth of crypto instead of $100)
	// Expected: We should only earn ~$0.25, rebate ~$0.25

	// 50% fill: If we ordered $99.50 worth and got 50%, we got $49.75 worth
	// cumQty needs to represent this amount at some price
	// Let's say price is $50,000 per BTC
	// $49.75 / $50,000 = 0.000995 BTC

	settlement := handler.calculateFeeSettlement(
		"0.000995", // cumQty (half of what was ordered)
		"50000",    // avgPx
		"100",      // userRequestedAmount
		"0.50",     // markupAmount
		"99.50",    // primeOrderAmount
	)

	// Actual filled value = 0.000995 * 50000 = 49.75
	expectedFilledValue := decimal.NewFromFloat(49.75)

	filledValue, err := decimal.NewFromString(settlement.ActualFilledValue)
	if err != nil {
		t.Fatalf("Failed to parse ActualFilledValue: %v", err)
	}

	diff := filledValue.Sub(expectedFilledValue).Abs()
	if diff.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("ActualFilledValue = %s, want ~49.75", settlement.ActualFilledValue)
	}

	// Fee rate = 0.50 / 100 = 0.005
	// Actual user cost = 49.75 / (1 - 0.005) ≈ 50
	// Earned fee = 50 * 0.005 = 0.25
	// Rebate = 0.50 - 0.25 = 0.25

	earnedFee, err := decimal.NewFromString(settlement.ActualEarnedFee)
	if err != nil {
		t.Fatalf("Failed to parse ActualEarnedFee: %v", err)
	}

	expectedEarned := decimal.NewFromFloat(0.25)
	earnedDiff := earnedFee.Sub(expectedEarned).Abs()
	if earnedDiff.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("ActualEarnedFee = %s, want ~0.25", settlement.ActualEarnedFee)
	}

	rebate, err := decimal.NewFromString(settlement.RebateAmount)
	if err != nil {
		t.Fatalf("Failed to parse RebateAmount: %v", err)
	}

	expectedRebate := decimal.NewFromFloat(0.25)
	rebateDiff := rebate.Sub(expectedRebate).Abs()
	if rebateDiff.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("RebateAmount = %s, want ~0.25", settlement.RebateAmount)
	}
}

func TestCalculateFeeSettlement_NoFill(t *testing.T) {
	handler := &DbOrderHandler{}

	// Scenario: Order cancelled with zero fills
	// Expected: Full rebate of markup amount
	settlement := handler.calculateFeeSettlement(
		"0",     // cumQty (no fill)
		"85000", // avgPx
		"10",    // userRequestedAmount
		"0.05",  // markupAmount
		"9.95",  // primeOrderAmount
	)

	if settlement.ActualFilledValue != "0" {
		t.Errorf("ActualFilledValue = %s, want 0", settlement.ActualFilledValue)
	}

	if settlement.ActualEarnedFee != "0" {
		t.Errorf("ActualEarnedFee = %s, want 0", settlement.ActualEarnedFee)
	}

	if settlement.RebateAmount != "0.05" {
		t.Errorf("RebateAmount = %s, want 0.05 (full rebate)", settlement.RebateAmount)
	}
}

func TestCalculateFeeSettlement_ZeroPrice(t *testing.T) {
	handler := &DbOrderHandler{}

	// Scenario: Invalid data - zero price
	// Expected: Full rebate (conservative approach)
	settlement := handler.calculateFeeSettlement(
		"0.001", // cumQty
		"0",     // avgPx (invalid)
		"10",    // userRequestedAmount
		"0.05",  // markupAmount
		"9.95",  // primeOrderAmount
	)

	if settlement.ActualFilledValue != "0" {
		t.Errorf("ActualFilledValue = %s, want 0", settlement.ActualFilledValue)
	}

	if settlement.ActualEarnedFee != "0" {
		t.Errorf("ActualEarnedFee = %s, want 0", settlement.ActualEarnedFee)
	}

	// Should refund full markup since we can't calculate properly
	if settlement.RebateAmount != "0.05" {
		t.Errorf("RebateAmount = %s, want 0.05 (full rebate on error)", settlement.RebateAmount)
	}
}

func TestCalculateFeeSettlement_NoMarkup(t *testing.T) {
	handler := &DbOrderHandler{}

	// Scenario: Order with no markup (shouldn't happen in practice)
	// Expected: No fee settlement needed
	settlement := handler.calculateFeeSettlement(
		"0.001", // cumQty
		"85000", // avgPx
		"10",    // userRequestedAmount
		"0",     // markupAmount (no markup)
		"10",    // primeOrderAmount
	)

	filledValue, _ := decimal.NewFromString(settlement.ActualFilledValue)
	expectedValue := decimal.NewFromFloat(0.001).Mul(decimal.NewFromInt(85000))

	if !filledValue.Equal(expectedValue) {
		t.Errorf("ActualFilledValue = %s, want %s", settlement.ActualFilledValue, expectedValue.String())
	}

	if settlement.ActualEarnedFee != "0" {
		t.Errorf("ActualEarnedFee = %s, want 0", settlement.ActualEarnedFee)
	}

	if settlement.RebateAmount != "0" {
		t.Errorf("RebateAmount = %s, want 0", settlement.RebateAmount)
	}
}

func TestCalculateFeeSettlement_SmallOrder(t *testing.T) {
	handler := &DbOrderHandler{}

	// Scenario: Smaller order ($5 with $0.025 fee = 0.5%)
	// 100% fill
	// Expected: Full fee earned, minimal rebate
	settlement := handler.calculateFeeSettlement(
		"0.0000588", // cumQty (amount of BTC)
		"85000",     // avgPx
		"5",         // userRequestedAmount ($5)
		"0.025",     // markupAmount ($0.025 fee = 0.5%)
		"4.975",     // primeOrderAmount
	)

	// Filled value = 0.0000588 * 85000 ≈ 4.998
	filledValue, _ := decimal.NewFromString(settlement.ActualFilledValue)
	if filledValue.LessThan(decimal.NewFromFloat(4.9)) || filledValue.GreaterThan(decimal.NewFromFloat(5.1)) {
		t.Errorf("ActualFilledValue = %s, want ~5", settlement.ActualFilledValue)
	}

	// Should earn close to full fee (allow reasonable tolerance)
	earnedFee, _ := decimal.NewFromString(settlement.ActualEarnedFee)
	expectedFee := decimal.NewFromFloat(0.025)
	if earnedFee.Sub(expectedFee).Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("ActualEarnedFee = %s, want ~0.025", settlement.ActualEarnedFee)
	}

	// Rebate should be very small (near 0, but allow some variance)
	rebate, _ := decimal.NewFromString(settlement.RebateAmount)
	if rebate.GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("RebateAmount = %s, want ~0", settlement.RebateAmount)
	}
}

func TestMetadataStore(t *testing.T) {
	store := NewMetadataStore()

	// Test Set and Get
	testData := map[string]interface{}{
		"order1": "data1",
		"order2": "data2",
	}

	for orderId, data := range testData {
		store.Set(orderId, data)
	}

	// Test Get existing
	for orderId, expectedData := range testData {
		data, exists := store.Get(orderId)
		if !exists {
			t.Errorf("Get(%q) exists = false, want true", orderId)
		}
		if data != expectedData {
			t.Errorf("Get(%q) = %v, want %v", orderId, data, expectedData)
		}
	}

	// Test Get non-existing
	_, exists := store.Get("non-existent")
	if exists {
		t.Error("Get(non-existent) exists = true, want false")
	}

	// Test Delete
	store.Delete("order1")
	_, exists = store.Get("order1")
	if exists {
		t.Error("After Delete, Get(order1) exists = true, want false")
	}

	// order2 should still exist
	_, exists = store.Get("order2")
	if !exists {
		t.Error("Get(order2) exists = false, want true (should not be deleted)")
	}

	// Test Delete non-existing (should not panic)
	store.Delete("non-existent")
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "existing string",
			m:        map[string]interface{}{"key": "value"},
			key:      "key",
			expected: "value",
		},
		{
			name:     "missing key",
			m:        map[string]interface{}{"other": "value"},
			key:      "key",
			expected: "",
		},
		{
			name:     "non-string value",
			m:        map[string]interface{}{"key": 123},
			key:      "key",
			expected: "",
		},
		{
			name:     "empty map",
			m:        map[string]interface{}{},
			key:      "key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.m, tt.key)
			if result != tt.expected {
				t.Errorf("getString(%v, %q) = %q, want %q", tt.m, tt.key, result, tt.expected)
			}
		})
	}
}

func TestRoundPriceString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid price", "100.556", "100.56"},
		{"already rounded", "100.50", "100.5"},
		{"zero", "0", "0"},
		{"empty string", "", ""},          // Returns unchanged
		{"invalid", "invalid", "invalid"}, // Returns unchanged
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundPriceString(tt.input)
			if result != tt.expected {
				t.Errorf("roundPriceString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRoundQtyString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid quantity", "0.123456789", "0.12345679"},
		{"already rounded", "0.12345678", "0.12345678"},
		{"zero", "0", "0"},
		{"empty string", "", ""},          // Returns unchanged
		{"invalid", "invalid", "invalid"}, // Returns unchanged
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundQtyString(tt.input)
			if result != tt.expected {
				t.Errorf("roundQtyString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
