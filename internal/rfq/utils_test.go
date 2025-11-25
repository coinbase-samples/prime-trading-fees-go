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

package rfq

import (
	"testing"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/shopspring/decimal"
)

func TestValidateRfqRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     RfqRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid quote order",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: false,
		},
		{
			name: "valid base order",
			req: RfqRequest{
				Product:    "ETH-USD",
				Side:       "SELL",
				BaseQty:    decimal.NewFromFloat(0.5),
				LimitPrice: decimal.NewFromFloat(2500.0),
				Unit:       "base",
			},
			wantErr: false,
		},
		{
			name: "missing product",
			req: RfqRequest{
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "product is required",
		},
		{
			name: "invalid side",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "HOLD",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "side must be BUY or SELL",
		},
		{
			name: "missing limit price",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "limit price is required and must be positive",
		},
		{
			name: "zero limit price",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.Zero,
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "limit price is required and must be positive",
		},
		{
			name: "negative limit price",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(-43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "limit price is required and must be positive",
		},
		{
			name: "zero quote value",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.Zero,
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "quote value must be positive",
		},
		{
			name: "negative quote value",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(-100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "quote value must be positive",
		},
		{
			name: "zero base quantity",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "SELL",
				BaseQty:    decimal.Zero,
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "base",
			},
			wantErr: true,
			errMsg:  "base quantity must be positive",
		},
		{
			name: "negative base quantity",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "SELL",
				BaseQty:    decimal.NewFromFloat(-0.5),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "base",
			},
			wantErr: true,
			errMsg:  "base quantity must be positive",
		},
		{
			name: "invalid unit",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "invalid",
			},
			wantErr: true,
			errMsg:  "unit must be 'base' or 'quote'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRfqRequest(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRfqRequest() expected error containing %q, got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("ValidateRfqRequest() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateRfqRequest() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNormalizeSide(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase buy", "BUY", "BUY"},
		{"lowercase buy", "buy", "BUY"},
		{"mixed case buy", "Buy", "Buy"}, // Returns unchanged
		{"uppercase sell", "SELL", "SELL"},
		{"lowercase sell", "sell", "SELL"},
		{"mixed case sell", "Sell", "Sell"}, // Returns unchanged
		{"invalid side", "invalid", "invalid"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.NormalizeSide(tt.input)
			if result != tt.expected {
				t.Errorf("common.NormalizeSide(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
