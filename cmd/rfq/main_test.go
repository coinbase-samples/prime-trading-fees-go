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

package main

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseAndValidateFlags(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		side        string
		qty         string
		unit        string
		price       string
		autoAccept  bool
		wantErr     bool
		errContains string
		validate    func(*testing.T, *parsedFlags)
	}{
		// Happy path - buy with default unit
		{
			name:       "rfq buy with default unit",
			symbol:     "BTC-USD",
			side:       "buy",
			qty:        "10000",
			unit:       "",
			price:      "50000",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				if flags.symbol != "BTC-USD" {
					t.Errorf("expected symbol BTC-USD, got %s", flags.symbol)
				}
				if flags.side != "BUY" {
					t.Errorf("expected side BUY, got %s", flags.side)
				}
				if flags.unitType != "quote" {
					t.Errorf("expected unit quote, got %s", flags.unitType)
				}
				if !flags.quantity.Equal(decimal.NewFromInt(10000)) {
					t.Errorf("expected quantity 10000, got %s", flags.quantity)
				}
				if !flags.limitPrice.Equal(decimal.NewFromInt(50000)) {
					t.Errorf("expected price 50000, got %s", flags.limitPrice)
				}
				if flags.autoAccept {
					t.Error("expected autoAccept false")
				}
			},
		},
		// Happy path - sell with default unit
		{
			name:       "rfq sell with default unit",
			symbol:     "ETH-USD",
			side:       "sell",
			qty:        "1.5",
			unit:       "",
			price:      "3000",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				if flags.side != "SELL" {
					t.Errorf("expected side SELL, got %s", flags.side)
				}
				if flags.unitType != "base" {
					t.Errorf("expected unit base, got %s", flags.unitType)
				}
			},
		},
		// Happy path - with auto-accept
		{
			name:       "rfq with auto-accept",
			symbol:     "BTC-USD",
			side:       "buy",
			qty:        "1000",
			unit:       "quote",
			price:      "50000",
			autoAccept: true,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				if !flags.autoAccept {
					t.Error("expected autoAccept true")
				}
			},
		},
		// Happy path - explicit base unit on buy
		{
			name:       "buy with explicit base unit",
			symbol:     "BTC-USD",
			side:       "buy",
			qty:        "0.1",
			unit:       "base",
			price:      "50000",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				if flags.unitType != "base" {
					t.Errorf("expected explicit unit base, got %s", flags.unitType)
				}
			},
		},
		// Case insensitivity for unit
		{
			name:       "case insensitive unit",
			symbol:     "BTC-USD",
			side:       "buy",
			qty:        "1000",
			unit:       "QuOtE",
			price:      "50000",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				if flags.unitType != "quote" {
					t.Errorf("expected normalized unit quote, got %s", flags.unitType)
				}
			},
		},
		// Missing required: symbol
		{
			name:        "missing symbol",
			symbol:      "",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			price:       "50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--symbol is required",
		},
		// Missing required: side
		{
			name:        "missing side",
			symbol:      "BTC-USD",
			side:        "",
			qty:         "1000",
			unit:        "",
			price:       "50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--side is required",
		},
		// Missing required: qty
		{
			name:        "missing qty",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "",
			unit:        "",
			price:       "50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--qty is required",
		},
		// Missing required: price (specific to RFQ)
		{
			name:        "missing price",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			price:       "",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--price is required for RFQ",
		},
		// Invalid side
		{
			name:        "invalid side",
			symbol:      "BTC-USD",
			side:        "invalid",
			qty:         "1000",
			unit:        "",
			price:       "50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--side must be 'buy' or 'sell'",
		},
		// Invalid unit
		{
			name:        "invalid unit",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "invalid",
			price:       "50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--unit must be 'base' or 'quote'",
		},
		// Invalid quantity format
		{
			name:        "invalid quantity format",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "not-a-number",
			unit:        "",
			price:       "50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "invalid quantity",
		},
		// Invalid price format
		{
			name:        "invalid price format",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			price:       "not-a-number",
			autoAccept:  false,
			wantErr:     true,
			errContains: "invalid price",
		},
		// Zero price (not allowed for RFQ)
		{
			name:        "zero price",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			price:       "0",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--price must be positive",
		},
		// Negative price (not allowed for RFQ)
		{
			name:        "negative price",
			symbol:      "BTC-USD",
			side:        "buy",
			qty:         "1000",
			unit:        "",
			price:       "-50000",
			autoAccept:  false,
			wantErr:     true,
			errContains: "--price must be positive",
		},
		// Decimal quantity
		{
			name:       "decimal quantity",
			symbol:     "BTC-USD",
			side:       "buy",
			qty:        "0.00123456",
			unit:       "base",
			price:      "50000",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				expected := decimal.RequireFromString("0.00123456")
				if !flags.quantity.Equal(expected) {
					t.Errorf("expected quantity %s, got %s", expected, flags.quantity)
				}
			},
		},
		// Large decimal price
		{
			name:       "large decimal price",
			symbol:     "BTC-USD",
			side:       "buy",
			qty:        "1000",
			unit:       "quote",
			price:      "99999.99",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				expected := decimal.RequireFromString("99999.99")
				if !flags.limitPrice.Equal(expected) {
					t.Errorf("expected price %s, got %s", expected, flags.limitPrice)
				}
			},
		},
		// Very small price (should still be positive)
		{
			name:       "very small positive price",
			symbol:     "SHIB-USD",
			side:       "buy",
			qty:        "1000000",
			unit:       "base",
			price:      "0.00001",
			autoAccept: false,
			wantErr:    false,
			validate: func(t *testing.T, flags *parsedFlags) {
				if flags.limitPrice.IsZero() || flags.limitPrice.IsNegative() {
					t.Errorf("expected positive price, got %s", flags.limitPrice)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := parseAndValidateFlags(
				tt.symbol,
				tt.side,
				tt.qty,
				tt.unit,
				tt.price,
				tt.autoAccept,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if flags == nil {
				t.Error("expected non-nil flags")
				return
			}

			if tt.validate != nil {
				tt.validate(t, flags)
			}
		})
	}
}
