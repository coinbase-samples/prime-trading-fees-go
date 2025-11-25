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
	"github.com/shopspring/decimal"
)

// RfqRequest represents a request for quote from the user's perspective
type RfqRequest struct {
	Product    string          // e.g., "BTC-USD"
	Side       string          // "BUY" or "SELL"
	BaseQty    decimal.Decimal // For base-denominated RFQs
	QuoteValue decimal.Decimal // For quote-denominated RFQs
	LimitPrice decimal.Decimal // Optional limit price
	Unit       string          // "base" or "quote"
}

// RfqResponse represents the quote response shown to the user
type RfqResponse struct {
	QuoteId             string `json:"quote_id"`
	Product             string `json:"product"`
	Side                string `json:"side"`
	ExpirationTime      string `json:"expiration_time"`
	Unit                string `json:"unit"`
	UserRequestedAmount string `json:"user_requested_amount"`
	Timestamp           string `json:"timestamp"`
	RawPrimeQuote       struct {
		BestPrice            string `json:"best_price"`
		OrderTotal           string `json:"order_total"`
		PriceInclusiveOfFees string `json:"price_inclusive_of_fees"`
	} `json:"raw_prime_quote"`
	CustomFeeOverlay struct {
		FeeAmount      string `json:"fee_amount"`
		FeePercent     string `json:"fee_percent"`
		EffectivePrice string `json:"effective_price"`
		TotalCost      string `json:"total_cost"`
	} `json:"custom_fee_overlay"`
}

// AcceptRfqRequest represents the request to accept a quote
type AcceptRfqRequest struct {
	QuoteId       string
	Product       string
	Side          string
	ClientOrderId string
}

// AcceptRfqResponse represents the response after accepting a quote
type AcceptRfqResponse struct {
	OrderId       string `json:"order_id"`
	QuoteId       string `json:"quote_id"`
	ClientOrderId string `json:"client_order_id"`
	Product       string `json:"product"`
	Side          string `json:"side"`
}
