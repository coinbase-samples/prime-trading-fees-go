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
	"fmt"
)

// ValidateRfqRequest validates an RFQ request
func ValidateRfqRequest(req RfqRequest) error {
	if req.Product == "" {
		return fmt.Errorf("product is required")
	}

	if req.Side != "BUY" && req.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}

	if req.LimitPrice.IsZero() || req.LimitPrice.IsNegative() {
		return fmt.Errorf("limit price is required and must be positive")
	}

	if req.Unit == "quote" {
		if req.QuoteValue.IsZero() || req.QuoteValue.IsNegative() {
			return fmt.Errorf("quote value must be positive")
		}
	} else if req.Unit == "base" {
		if req.BaseQty.IsZero() || req.BaseQty.IsNegative() {
			return fmt.Errorf("base quantity must be positive")
		}
	} else {
		return fmt.Errorf("unit must be 'base' or 'quote'")
	}

	return nil
}
