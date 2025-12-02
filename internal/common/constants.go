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

// ============================================================================
// Order Status Constants
// ============================================================================

// Order statuses from Prime WebSocket API
const (
	OrderStatusFilled    = "FILLED"
	OrderStatusCancelled = "CANCELLED"
	OrderStatusRejected  = "REJECTED"
	OrderStatusOpen      = "OPEN"
)

// ============================================================================
// Default Values
// ============================================================================

const (
	DefaultZeroString = "0"
	UnknownEventType  = "unknown"
)

// ============================================================================
// Currency Precision Constants
// ============================================================================

// Quote currency precision for rounding (decimal places)
const (
	PrecisionUSD     = 2 // USD, USDC, USDT
	PrecisionBTC     = 8 // Bitcoin
	PrecisionETH     = 8 // Ethereum
	PrecisionDefault = 8 // Default for unknown currencies
)
