# Prime Fee Passthrough Integration Guide

A comprehensive guide for integrating Coinbase Prime with custom fee markup.

## Table of Contents

- [Overview](#overview)
- [Business Model](#business-model)
- [Fee Calculation Fundamentals](#fee-calculation-fundamentals)
- [Implementation](#implementation)
  - [1. Market Data with Fee-Adjusted Prices](#1-market-data-with-fee-adjusted-prices)
  - [2. Order Preview](#2-order-preview)
  - [3. Order Placement & Tracking](#3-order-placement--tracking)
  - [4. Request For Quote (RFQ)](#4-request-for-quote-rfq)
  - [5. Fee Settlement (Partial Fills)](#5-fee-settlement-partial-fills)
- [Complete Integration Example](#complete-integration-example)
- [Testing & Verification](#testing--verification)

---

## Overview

This guide teaches you how to build a **fee passthrough layer** on top of Coinbase Prime by:

1. **Adjusting market prices** - Show fee-inclusive prices to users
2. **Applying fees** - Charge your markup using one of two models
3. **Tracking execution** - Monitor fills via WebSocket
4. **Settling partial fills** - Calculate fair rebates when needed

---

## Business Model

### Two Fee Models (Depends on Order Type)

Your fee calculation depends on how the user specifies their order:

#### Quote Orders: "Buy $100 of BTC" → Fee Hold Model

**Deduct fee BEFORE sending to Prime:**
```
1. User provides: $100
2. You hold as fee: $0.50 (0.5%)
3. Send to Prime: $99.50
4. User receives: ~$99.50 worth of BTC
```

**Why deduct upfront?** Because the user specified a dollar amount. If you charged after, you'd need to either:
- Split the BTC fractionally, or
- Bill them separately

#### Base Orders: "Buy 1 BTC" → Add-On Model

**Charge fee AFTER Prime execution:**
```
1. User requests: 1 BTC
2. Send to Prime: 1 BTC
3. Prime fills at: $43,250
4. Your fee: $43,250 × 0.005 = $216.25
5. User pays: $43,466.25 total
```

**Why charge after?** Because the user specified a crypto amount. You know exactly what they're getting (1 BTC), so you just add your fee on top of Prime's cost.

### Industry Convention: Buys in Quote, Sells in Base

In practice, platforms who have integrated Prime typically default to:
- **Buy orders** → Quote-denominated ("Buy $100 of BTC")
- **Sell orders** → Base-denominated ("Sell 0.5 BTC")

**Why this convention?** It aligns with how users naturally think about trading:
- When buying crypto, users ask: *"How much USD do I want to spend?"*
- When selling crypto, users ask: *"How much BTC do I want to sell?"*

**Bonus benefit:** This convention means each order type consistently uses one fee model:
- Buys (quote) → Always use Fee Hold Model
- Sells (base) → Always use Add-On Model

This simplification makes the user experience more predictable and the implementation cleaner.

---

## Fee Calculation Fundamentals

### Percentage-Based Fees

This reference application uses **percentage-based fees only** - the industry standard for crypto trading.

**Formula:**
```
fee = notional_value × fee_percent
```

**Example:** 50 basis points (0.5%)
- Order: $10,000
- Fee: $10,000 × 0.005 = $50

**Implementation:** See `internal/fees/strategy.go:FeeStrategy`

### Configuration

```bash
# In .env file:
FEE_PERCENT=0.005  # 50 bps (0.5%)
```

---

## Implementation

### 1. Market Data with Fee-Adjusted Prices

**Purpose:** Show users prices that **include your fees** so they know the true cost upfront.

#### How It Works

Prime provides raw bid/ask prices. You adjust them to include your markup:

**For BUY orders (user buying crypto):**
- **Ask price goes UP** (user pays more)
- Formula: `adjusted_ask = prime_ask × (1 + fee_rate)`

**For SELL orders (user selling crypto):**
- **Bid price goes DOWN** (user receives less)
- Formula: `adjusted_bid = prime_bid × (1 - fee_rate)`

#### The Math

**Example with 50 bps (0.5%) fee:**

Prime's Raw Order Book:
```
BTC-USD
Ask: $43,250.00 (0.5 BTC available)
Bid: $43,245.00 (0.75 BTC available)
```

Your Adjusted Order Book (shown to users):
```
BTC-USD
Ask: $43,466.25  ($43,250 × 1.005)
Bid: $43,028.78  ($43,245 × 0.995)
```

#### Implementation

**Key files:**
- `internal/fees/strategy.go` - Fee adjustment logic
  - `AdjustBidPrice()` - Reduces bid price by fee amount
  - `AdjustAskPrice()` - Increases ask price by fee amount
- `internal/marketdata/websocket.go` - Prime WebSocket client
- `internal/marketdata/orderbook.go` - Order book storage

**Key logic:**
1. Connect to Prime WebSocket feed
2. Store raw order book data
3. Apply fee adjustments when displaying prices to users
4. For buys: increase ask price by fee
5. For sells: decrease bid price by fee

#### Running the Code

```bash
go run cmd/stream/main.go --symbols=BTC-USD,ETH-USD
```

**Output:**
```
═══════════════════════════════════════════════════
  BTC-USD Order Book @ 14:32:15
═══════════════════════════════════════════════════

  ASK SIZE    PRIME PRICE    YOUR PRICE (incl. fee)
  --------    -----------    ----------------------
  0.5000      43250.00       43466.25  (+0.5%)
  1.2000      43248.50       43464.54

  BID SIZE    PRIME PRICE    YOUR PRICE (incl. fee)
  --------    -----------    ----------------------
  0.7500      43245.00       43028.78  (-0.5%)
  1.5000      43243.50       43027.28
```

---

### 2. Order Preview

**Purpose:** Coinbase Prime includes a REST API that allows users to see execution details before placing orders.

#### Two Order Types: Base vs Quote

**Quote-Denominated:** "Buy $100 worth of BTC"
- User specifies dollar amount
- Fee deducted BEFORE sending to Prime
- Example: Hold $0.50 fee, send $99.50 to Prime

**Base-Denominated:** "Buy 1 BTC"
- User specifies crypto amount
- Fee charged AFTER execution (on top of Prime's cost)
- Example: Prime fills 1 BTC for $43,250 + $43.25 fee = User pays $43,293.25

#### Quote-Denominated Example

**User Request:** "Buy $10 of BTC" (50 bps fee)

**Processing:**
1. Calculate fee hold: `$10 × 0.005 = $0.05`
2. Send to Prime preview: `$9.95`
3. Prime returns execution details for $9.95
4. Calculate effective price including your fee

#### Base-Denominated Example

**User Request:** "Buy 1 BTC" (50 bps fee)

**Processing:**
1. Send full quantity to Prime: `1 BTC`
2. Prime returns execution cost: `$43,250`
3. Calculate your fee: `$43,250 × 0.005 = $216.25`
4. User pays total: `$43,250 + $216.25 = $43,466.25`

**Key Difference:** With base orders, you charge the fee on TOP of Prime's execution cost. No upfront hold needed.

#### Response Structure

```json
{
  "product": "BTC-USD",
  "side": "BUY",
  "type": "MARKET",
  "order_unit": "quote",
  "user_requested_amount": "10.00",
  "timestamp": "2025-01-15T10:30:45Z",

  "raw_prime_preview": {
    "quantity": "0.00022989",
    "average_filled_price": "43251.25",
    "total_value": "9.95",
    "commission": "0.10"
  },

  "custom_fee_overlay": {
    "fee_amount": "0.05",
    "fee_percent": "0.50",
    "effective_price": "43684.21"
  }
}
```

#### The Effective Price Calculation

**Effective Price** = Total cost to user / Crypto received

The effective price accounts for:
1. The execution price from Prime
2. Prime's commission fees
3. Your custom markup fee

**Formula:**
```
total_cost = (base_qty × execution_price) + prime_commission + custom_fee
effective_price = total_cost / base_qty
```

**Example:**
```
User requested: $10.00
Your fee (50 bps): $0.05
Sent to Prime: $9.95
Prime filled: 0.00022989 BTC at $43,251.25
Prime commission: $0.10
Custom fee: $0.05

Total cost = (0.00022989 × 43251.25) + 0.10 + 0.05 = $10.10
Effective price = $10.10 / 0.00022989 = $43,932.18/BTC
```

This is higher than Prime's execution price because it includes both Prime's commission and your markup.

---

### 3. Order Placement & Tracking

**Purpose:** Place actual orders and track execution via WebSocket.

#### Placing an Order

```bash
# Quote-denominated market buy
go run cmd/order/main.go --symbol=BTC-USD --side=buy --unit=quote --qty=10 --type=market

# Quote-denominated limit buy
go run cmd/order/main.go --symbol=BTC-USD --side=buy --unit=quote --qty=10 --price 40000 --type=limit           

# Base-denominated market sell
go run cmd/order/main.go --symbol=BTC-USD --side=sell --qty=0.001 --type=market
```

Note that on buys, the default unit is `quote` and on sells, the default unit is `base`. This aligns with common implementation patterns. Additionally, the default order type is always `market` unless type is specified otherwise. 

#### Implementation

**Key files:**
- `internal/order/preview.go:PlaceOrder()` - Order placement logic
- `internal/order/utils.go:PrepareOrderRequest()` - Fee deduction (same as preview)
- `internal/orders/handler.go` - Metadata storage

**Order flow:**
1. **Prepare** - Apply fee logic (deduct for quote, add for base)
2. **Send to Prime** - Place order with Prime API
3. **Store metadata** - Save fee details for settlement

#### Tracking Order Execution

Start the websocket client to receive real-time order updates:

```bash
go run cmd/orders-stream/main.go --symbols=BTC-USD,ETH-USD
```

**Key files:**
- `internal/orders/websocket.go` - WebSocket client for order updates
- `internal/orders/handler.go:HandleOrderUpdate()` - Processes order events

**Websocket handler flow:**
1. Receives order updates from Prime (status, fills, etc.)
2. Retrieves metadata from in-memory store or database
3. For terminal states (FILLED/CANCELLED/REJECTED):
   - Calculates fee settlement based on actual fill
   - Stores results in database
4. Upserts order record with execution details

---

### 4. Request For Quote (RFQ)

**Purpose:** Get a guaranteed price quote before executing a trade. Unlike market orders that execute immediately, RFQ provides price certainty. Common implementation path for clients who wish to avoid maintaining an order book or care most about clean user execution. 

#### How RFQ Works

1. **Create Quote** - Request a guaranteed price from Prime with a limit price
2. **Review Quote** - Receive quote with best price, total cost, and expiration time (~2.5s from receipt of quote)
3. **Accept Quote** (optional) - Execute the trade at the quoted price before expiration

#### Creating a Quote

**Command:**
```bash
# Quote-denominated: Buy $1000 of BTC at limit price $88,000
go run cmd/rfq/main.go --symbol=BTC-USD --side=buy --qty=1000 --price=88000

# Base-denominated: Sell 0.5 BTC at limit price $43,000
go run cmd/rfq/main.go --symbol=BTC-USD --side=sell --qty=0.5 --price=43000 --auto-accept
```

**Fee Application (Same as Regular Orders):**

**Quote Orders:** Fee held upfront
```
User requests: $1000
Fee held: $5 (0.5%)
Sent to Prime: $995
```

**Base Orders:** Fee added on top
```
User requests: 0.5 BTC
Sent to Prime: 0.5 BTC
Prime cost: $21,500
Your fee: $107.50 (0.5% of $21,500)
Total user pays: $21,607.50
```

#### Response Structure

```json
{
  "quote_id": "uuid",
  "product": "BTC-USD",
  "side": "BUY",
  "expiration_time": "2025-01-15T10:35:45Z",
  "unit": "quote",
  "user_requested_amount": "1000",
  "raw_prime_quote": {
    "best_price": "87950.00",
    "order_total": "995.00",
    "price_inclusive_of_fees": "88100.00"
  },
  "custom_fee_overlay": {
    "fee_amount": "5.00",
    "fee_percent": "0.50",
    "effective_price": "88396.23",
    "total_cost": "1000.00"
  }
}
```

#### Implementation

**Key files:**
- `internal/rfq/service.go` - RFQ quote creation and acceptance
- `internal/rfq/models.go` - RFQ request/response structures
- `cmd/rfq/main.go` - CLI command with `--auto-accept` flag

**RFQ flow:**
1. **Validate** - Ensure limit price is provided and marketable
2. **Apply fees** - Deduct for quote orders, full amount for base orders
3. **Create quote** - Call Prime RFQ API with adjusted amounts
4. **Build response** - Calculate effective price including markup
5. **Accept (optional)** - If `--auto-accept`, immediately execute trade

**Note:** RFQ does not use WebSocket tracking. Quote acceptance returns an order ID that can be tracked via REST API.

---

### 5. Fee Settlement - Releasing the Hold

**Applies to:** Quote-denominated orders only (base orders charge on top, so no handling needed)

**The Problem:** You held a fee based on the full order, but partial fills mean you overcharged.

**Example:** $10 order, 50 bps fee, 50% fill

##### Scenario A: 100% Fill

```
Prime fills: $9.95 worth of BTC (100%)
Fee held: $0.05
Fee earned: $0.05 (0.5% of $10 transaction)
Released back: $0 ✓
```

##### Scenario B: 50% Fill

```
Prime fills: $4.975 worth of BTC (50%)
Fee held: $0.05
Fee earned: $0.025 (0.5% of $5 actual transaction)
Released back: $0.025 to customer ✓
```

**Why release $0.025?**
Customer only got $4.975 of BTC, so they should only pay $5 total ($4.975 + $0.025 fee). You held $0.05 but only earned $0.025.

##### Scenario C: 0% Fill (Order Cancelled)

```
Prime fills: $0
Fee held: $0.05
Fee earned: $0 (no transaction occurred)
Released back: Full $0.05 to customer ✓
```

#### Settlement Calculation

When an order reaches a **terminal state** (FILLED, CANCELLED, or REJECTED):

1. **Actual Filled Value:** `cum_qty × avg_px` (from Prime)
2. **Fee Rate:** `fee_held / user_requested_amount`
3. **Actual User Cost:** `actual_filled_value / (1 - fee_rate)`
4. **Fee Earned:** `actual_user_cost × fee_rate`
5. **Release Amount:** `fee_held - fee_earned`

#### Implementation

**Key file:** `internal/orders/handler.go:calculateFeeSettlement()`

**Algorithm:**
1. Calculate actual filled value: `actualFilledValue = cumQty × avgPx`
2. Calculate fee rate from original order: `feeRate = feeHeld / userRequestedAmount`
3. Calculate what user should pay: `actualUserCost = actualFilledValue / (1 - feeRate)`
4. Calculate fee earned: `feeEarned = actualUserCost × feeRate`
5. Cap at held amount: `min(feeEarned, feeHeld)`
6. Calculate release: `releaseAmount = feeHeld - feeEarned`

#### Math Verification

**50% Fill with 50 bps fee:**

```
User requested: $10.00
Fee held: $0.05 (0.5%)
Sent to Prime: $9.95
Prime filled: $4.975 (50% fill)

Settlement calculation:
  fee_rate = $0.05 / $10.00 = 0.005
  actual_filled_value = $4.975
  actual_user_cost = $4.975 / (1 - 0.005) = $4.975 / 0.995 = $5.00
  fee_earned = $5.00 × 0.005 = $0.025
  release_amount = $0.05 - $0.025 = $0.025

Result: ✓
  User pays: $5.00 total ($4.975 BTC + $0.025 fee)
  You earn: $0.025 (0.5% of $5.00)
  You rebate: $5.00 to user ($4.975 unfilled + $0.025 fee rebate)
```

#### Database Schema

The `orders` table includes fee settlement fields:

```sql
CREATE TABLE orders (
    order_id TEXT PRIMARY KEY,

    -- User's original request
    user_requested_amount TEXT DEFAULT '0',
    markup_amount TEXT DEFAULT '0',
    prime_order_amount TEXT DEFAULT '0',

    -- Fee settlement (calculated at terminal state)
    actual_filled_value TEXT DEFAULT '0',
    actual_earned_fee TEXT DEFAULT '0',
    rebate_amount TEXT DEFAULT '0',
    fee_settled BOOLEAN DEFAULT FALSE,

    -- ... other fields
);
```

#### Querying Settled Orders

```sql
-- Orders that have rebates owed
SELECT
    order_id,
    status,
    user_requested_amount,
    markup_amount,
    actual_filled_value,
    actual_earned_fee,
    rebate_amount
FROM orders
WHERE fee_settled = TRUE
  AND CAST(rebate_amount AS REAL) > 0
ORDER BY last_updated_at DESC;
```
---

## Testing & Verification

### Manual Testing Checklist

**Market Data:**
- [ ] Raw Prime prices stream correctly
- [ ] Buy prices adjusted UP by fee percentage
- [ ] Sell prices adjusted DOWN by fee percentage
- [ ] Spreads widen appropriately

**Order Preview & Placement:**
- [ ] Quote orders: fee held upfront, reduced amount sent to Prime
- [ ] Response shows user_requested_amount correctly
- [ ] Effective price includes all fees

**Fee Settlement:**
- [ ] 100% fills earn full fee, no rebate
- [ ] Partial fills earn proportional fee
- [ ] Cancelled orders rebate full amount
- [ ] Database tracks settlement correctly

### SQL Verification Queries

```sql
-- Verify fee math for recent orders
SELECT
    order_id,
    user_requested_amount,
    markup_amount,
    actual_filled_value,
    actual_earned_fee,
    rebate_amount,
    -- Verify math: earned_fee + rebate = markup
    CASE
        WHEN ABS(
            CAST(markup_amount AS REAL) -
            (CAST(actual_earned_fee AS REAL) + CAST(rebate_amount AS REAL))
        ) < 0.01 THEN 'OK'
        ELSE 'ERROR'
    END as math_check
FROM orders
WHERE fee_settled = TRUE
ORDER BY last_updated_at DESC
LIMIT 10;
```

### API Documentation

- [Prime API Docs](https://docs.cdp.coinbase.com/prime/docs)
- [Prime Go SDK](https://github.com/coinbase-samples/prime-sdk-go)
