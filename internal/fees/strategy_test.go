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

package fees

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestFeeStrategy(t *testing.T) {
	tests := []struct {
		name     string
		percent  string
		qty      string
		price    string
		expected string
	}{
		{
			name:     "0.1% fee on BTC trade",
			percent:  "0.001",
			qty:      "1.0",
			price:    "50000.00",
			expected: "50.00",
		},
		{
			name:     "0.5% fee on larger trade",
			percent:  "0.005",
			qty:      "2.5",
			price:    "40000.00",
			expected: "500.00",
		},
		{
			name:     "1% fee on small trade",
			percent:  "0.01",
			qty:      "0.1",
			price:    "1000.00",
			expected: "1.00",
		},
		{
			name:     "zero quantity yields zero fee",
			percent:  "0.001",
			qty:      "0",
			price:    "50000.00",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			percent := decimal.RequireFromString(tt.percent)
			qty := decimal.RequireFromString(tt.qty)
			price := decimal.RequireFromString(tt.price)
			expected := decimal.RequireFromString(tt.expected)

			strategy := NewFeeStrategy(percent)
			result := strategy.Compute(qty, price)

			if !result.Equal(expected) {
				t.Errorf("expected %s, got %s", expected, result)
			}
		})
	}
}

func TestPriceAdjuster_AdjustBidPrice(t *testing.T) {
	// Fee is 0.1%
	feeStrategy := NewFeeStrategy(decimal.NewFromFloat(0.001))

	adjuster := NewPriceAdjuster(feeStrategy)

	tests := []struct {
		name     string
		price    string
		qty      string
		expected string
	}{
		{
			name:     "adjust bid down by 0.1%",
			price:    "50000.00",
			qty:      "1.0",
			expected: "49950.00", // Notional 50000, fee 50, adjusted notional 49950
		},
		{
			name:     "adjust smaller bid",
			price:    "1000.00",
			qty:      "1.0",
			expected: "999.00", // Notional 1000, fee 1, adjusted notional 999
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := decimal.RequireFromString(tt.price)
			qty := decimal.RequireFromString(tt.qty)
			expected := decimal.RequireFromString(tt.expected)

			result := adjuster.AdjustBidPrice(price, qty)

			if !result.Equal(expected) {
				t.Errorf("expected %s, got %s", expected, result)
			}
		})
	}
}

func TestPriceAdjuster_AdjustAskPrice(t *testing.T) {
	// Fee is 0.1%
	feeStrategy := NewFeeStrategy(decimal.NewFromFloat(0.001))

	adjuster := NewPriceAdjuster(feeStrategy)

	tests := []struct {
		name     string
		price    string
		qty      string
		expected string
	}{
		{
			name:     "adjust ask up by 0.1%",
			price:    "50000.00",
			qty:      "1.0",
			expected: "50050.00", // Notional 50000, fee 50, adjusted notional 50050
		},
		{
			name:     "adjust smaller ask",
			price:    "1000.00",
			qty:      "1.0",
			expected: "1001.00", // Notional 1000, fee 1, adjusted notional 1001
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := decimal.RequireFromString(tt.price)
			qty := decimal.RequireFromString(tt.qty)
			expected := decimal.RequireFromString(tt.expected)

			result := adjuster.AdjustAskPrice(price, qty)

			if !result.Equal(expected) {
				t.Errorf("expected %s, got %s", expected, result)
			}
		})
	}
}

func TestPriceAdjuster_ComputeFee(t *testing.T) {
	feeStrategy := NewFeeStrategy(decimal.NewFromFloat(0.001)) // 0.1%

	adjuster := NewPriceAdjuster(feeStrategy)

	tests := []struct {
		name     string
		qty      string
		price    string
		expected string
	}{
		{
			name:     "standard fee",
			qty:      "1.0",
			price:    "50000.00",
			expected: "50.00", // 0.1% of 50000
		},
		{
			name:     "larger trade",
			qty:      "2.0",
			price:    "40000.00",
			expected: "80.00", // 0.1% of 80000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty := decimal.RequireFromString(tt.qty)
			price := decimal.RequireFromString(tt.price)
			expected := decimal.RequireFromString(tt.expected)

			result := adjuster.ComputeFee(qty, price)

			if !result.Equal(expected) {
				t.Errorf("expected %s, got %s", expected, result)
			}
		})
	}
}

func BenchmarkFeeStrategy(b *testing.B) {
	strategy := NewFeeStrategy(decimal.NewFromFloat(0.001))
	qty := decimal.NewFromInt(1)
	price := decimal.NewFromInt(50000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Compute(qty, price)
	}
}
