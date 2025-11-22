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
  - [4. Fee Settlement (Partial Fills)](#4-fee-settlement-partial-fills)
- [Complete Integration Example](#complete-integration-example)
- [Testing & Verification](#testing--verification)

---

## Overview

This guide teaches you how to build a **fee passthrough layer** on top of Coinbase Prime. You'll learn how to:

1. Add custom markup fees to Prime's wholesale pricing
2. Show fee-adjusted prices to your users
3. Preview orders with upfront fee deduction
4. Track order execution via websocket
5. Handle partial fills fairly with fee settlement

**The Challenge:** Prime provides institutional pricing, but you need to add your own markup while maintaining transparency and fairness.

**The Solution:** Layer your fees on top of Prime's execution by:
- Adjusting displayed prices to include your markup
- Deducting fees upfront before sending orders to Prime
- Settling fees proportionally for partial fills

---

## Business Model

### The Value Chain

```
End User → Your Application → Coinbase Prime → Crypto Markets
          (Add Markup)        (Wholesale)
```

**Your Application's Role:**
1. User requests: "Buy $100 of BTC"
2. You calculate: "$100 - $0.50 (50 bps fee) = $99.50 to send to Prime"
3. Prime executes: $99.50 market order
4. User receives: BTC worth $99.50 (paid $100 total including your $0.50 fee)

### Why This Model Works

✅ **Transparent** - Users see prices including your fees upfront
✅ **Fair** - Fees proportional to executed amount
✅ **Scalable** - Prime handles institutional liquidity
✅ **Simple** - No order book management required

---

## Fee Calculation Fundamentals

### Fee Strategies

#### 1. Percent Fee (Most Common)

Charge a percentage of the order value.

**Formula:**
```
fee = notional_value × fee_percent
```

**Example:** 50 basis points (0.5%)
- Order: $10,000
- Fee: $10,000 × 0.005 = $50

**Code:**
```go
type PercentFeeStrategy struct {
    Percent decimal.Decimal // e.g., 0.005 for 50 bps
}

func (s *PercentFeeStrategy) Compute(qty, price decimal.Decimal) decimal.Decimal {
    notional := qty.Mul(price)
    return notional.Mul(s.Percent)
}
```

#### 2. Flat Fee

Fixed fee per trade regardless of size.

**Formula:**
```
fee = fixed_amount
```

**Example:** $2.99 per trade
- Order: Any size
- Fee: $2.99

**Code:**
```go
type FlatFeeStrategy struct {
    Amount decimal.Decimal
}

func (s *FlatFeeStrategy) Compute(qty, price decimal.Decimal) decimal.Decimal {
    return s.Amount
}
```

### Configuration

```yaml
fees:
  type: percent
  percent: "0.005"  # 50 bps (0.5%)
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
Ask: $43,466.25  ($43,250 × 1.005)  ← User buying pays MORE
Bid: $43,028.78  ($43,245 × 0.995)  ← User selling gets LESS
```

#### Code Implementation

```go
// Load config and create fee strategy
cfg, _ := config.LoadConfig("config.yaml")
feeStrategy, _ := fees.CreateFeeStrategy(cfg.Fees)
adjuster := fees.NewPriceAdjuster(feeStrategy)

// Connect to Prime WebSocket
wsClient := marketdata.NewWebSocketClient(wsConfig, orderBookStore)
go wsClient.Connect()

// Display fee-adjusted prices
ticker := time.NewTicker(5 * time.Second)
for range ticker.C {
    book := orderBookStore.GetOrderBook("BTC-USD")

    // Adjust bid prices (for sells - user receives less)
    for _, bid := range book.Bids {
        qty := decimal.NewFromString(bid.Quantity)
        price := decimal.NewFromString(bid.Price)
        adjustedPrice := adjuster.AdjustBidPrice(price, qty)

        fmt.Printf("Bid: %s → %s (adjusted)\n", price, adjustedPrice)
    }

    // Adjust ask prices (for buys - user pays more)
    for _, ask := range book.Asks {
        qty := decimal.NewFromString(ask.Quantity)
        price := decimal.NewFromString(ask.Price)
        adjustedPrice := adjuster.AdjustAskPrice(price, qty)

        fmt.Printf("Ask: %s → %s (adjusted)\n", price, adjustedPrice)
    }
}
```

#### Running the Code

```bash
go run cmd/stream/main.go
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

**Purpose:** Let users preview order execution **before** placing it, with all fees disclosed upfront.

#### Base vs Quote Denominated Orders

**Base-denominated:** Specify quantity in crypto units
- Example: "Buy 0.5 BTC"
- Simple: Just add fee on top

**Quote-denominated:** Specify quantity in fiat units
- Example: "Buy $100 worth of BTC"
- Complex: Must deduct fee upfront

#### Quote-Denominated Orders (The Important One)

This is the tricky case that requires upfront fee deduction.

**User Request:** "I want to buy $10 worth of BTC"

**What Happens:**

1. Calculate your markup: `$10.00 × 0.005 = $0.05`
2. Deduct markup: `$10.00 - $0.05 = $9.95`
3. Call Prime preview API with `$9.95`
4. Prime responds with execution details for $9.95
5. Return to user showing both Prime's response and your overlay

**Key Insight:** You hold the full $10 from the user, send $9.95 to Prime, keep $0.05 as fee.

#### Response Structure

```json
{
  "product": "BTC-USD",
  "side": "BUY",
  "type": "MARKET",

  "user_requested_amount": "10.00",

  "raw_prime_preview": {
    "order_total": "9.95",
    "best_bid": "43245.50",
    "best_ask": "43250.00",
    "average_filled_price": "43251.25",
    "total_fees": "0.10",
    "filled_value": "9.85"
  },

  "custom_fee_overlay": {
    "strategy_name": "Percent Fee (0.5000%)",
    "markup_amount": "0.05",
    "prime_order_amount": "9.95"
  },

  "final_breakdown": {
    "user_pays_total": "10.00",
    "crypto_value": "9.85",
    "prime_fees": "0.10",
    "your_markup": "0.05",
    "effective_price": "43899.37"
  }
}
```

#### The Effective Price Calculation

**Effective Price** = Total user paid / Crypto received

```
User paid: $10.00
BTC received: 0.000228 BTC (worth $9.85 at Prime's fill price)
Effective price: $10.00 / 0.000228 = $43,859/BTC
```

This is higher than Prime's execution price because it includes both Prime's fees and your markup.

#### Code Implementation

```go
func PrepareOrderRequest(
    req OrderRequest,
    portfolioId string,
    priceAdjuster *fees.PriceAdjuster,
) (*PreparedOrder, error) {

    // For quote-denominated orders, deduct fee upfront
    if req.Unit == "quote" && !req.QuoteValue.IsZero() {
        feeStrategy := getFeeStrategy(priceAdjuster, req.Side)

        // Calculate markup on user's requested amount
        userRequestedAmount := req.QuoteValue
        markupAmount := feeStrategy.ComputeFromNotional(req.QuoteValue)

        // Deduct markup from amount sent to Prime
        primeOrderAmount := req.QuoteValue.Sub(markupAmount)

        // Build Prime API request with reduced amount
        primeReq.Order.QuoteValue = primeOrderAmount.String()

        // Store metadata for later settlement
        metadata := &OrderMetadata{
            UserRequestedAmount: userRequestedAmount,
            MarkupAmount:        markupAmount,
            PrimeOrderAmount:    primeOrderAmount,
        }

        return &PreparedOrder{
            PrimeRequest: primeReq,
            Metadata:     metadata,
        }, nil
    }

    // ... handle base-denominated orders
}
```

---

### 3. Order Placement & Tracking

**Purpose:** Place actual orders and track execution via websocket.

#### Placing an Order

```bash
# Quote-denominated buy (default for buys)
go run cmd/order/main.go \
  --symbol=BTC-USD \
  --side=buy \
  --qty=100 \
  --type=market \
  --mode=execute

# Base-denominated sell (default for sells)
go run cmd/order/main.go \
  --symbol=ETH-USD \
  --side=sell \
  --qty=0.5 \
  --type=market \
  --mode=execute
```

#### What Happens Behind the Scenes

1. **Order Preparation:**
   ```go
   userRequestedAmount := $100.00
   markupAmount := $100.00 × 0.005 = $0.50
   primeOrderAmount := $100.00 - $0.50 = $99.50
   ```

2. **Send to Prime:**
   ```go
   primeResp, err := ordersSvc.CreateOrder(ctx, &prime.CreateOrderRequest{
       PortfolioId: portfolioId,
       Order: &prime.Order{
           ProductId:  "BTC-USD",
           Side:       "BUY",
           Type:       "MARKET",
           QuoteValue: "99.50",  // ← Reduced amount
       },
   })
   ```

3. **Store Metadata:**
   ```go
   db.UpsertOrder(&database.OrderRecord{
       OrderId:             primeResp.OrderId,
       UserRequestedAmount: "100.00",
       MarkupAmount:        "0.50",
       PrimeOrderAmount:    "99.50",
       Status:              "PENDING",
   })
   ```

#### Tracking Order Execution

Start the websocket client to receive real-time order updates:

```bash
go run cmd/orders-stream/main.go --symbols=BTC-USD,ETH-USD
```

The websocket handler:
1. Receives order updates from Prime
2. Calculates fee settlement for terminal states
3. Stores execution details in database

```go
func (h *DbOrderHandler) HandleOrderUpdate(update map[string]interface{}) error {
    // Parse Prime's order update
    orderId := getString(update["order_id"])
    status := getString(update["status"])
    cumQty := getString(update["cum_qty"])
    avgPx := getString(update["avg_px"])

    // Get metadata (upfront amounts)
    metadata, _ := h.metadataStore.Get(orderId)

    // If terminal state, calculate fee settlement
    if status == "FILLED" || status == "CANCELLED" || status == "REJECTED" {
        settlement := h.calculateFeeSettlement(
            cumQty, avgPx,
            metadata.UserRequestedAmount,
            metadata.MarkupAmount,
            metadata.PrimeOrderAmount,
        )

        // Store settlement in database
        orderRecord.ActualFilledValue = settlement.ActualFilledValue
        orderRecord.ActualEarnedFee = settlement.ActualEarnedFee
        orderRecord.RebateAmount = settlement.RebateAmount
        orderRecord.FeeSettled = true
    }

    db.UpsertOrder(orderRecord)
}
```

---

### 4. Fee Settlement (Partial Fills)

**The Problem:** For quote-denominated orders, you deduct the fee upfront. If the order only partially fills, you've overcharged the customer.

#### Example Scenario

**User Request:** "Buy $10 worth of BTC"
**Your Fee:** 50 basis points (0.5%)
**Upfront Deduction:** $10.00 × 0.005 = $0.05
**Sent to Prime:** $10.00 - $0.05 = $9.95

##### Scenario A: 100% Fill (Fair)

```
Prime fills: $9.95 worth of BTC
User paid: $10.00 total ($9.95 BTC + $0.05 fee) ✓
You earned: $0.05
Rebate owed: $0
```

##### Scenario B: 50% Fill (UNFAIR without settlement!)

```
Prime fills: $4.975 worth of BTC (50% of $9.95)
User paid: $10.00 but only got $4.975 of BTC ✗

Fair calculation:
- User should pay: $5.00 total ($4.975 BTC + $0.025 fee)
- You should earn: $0.025 (0.5% of $5.00)
- Rebate owed: $5.00 ($4.975 unfilled + $0.025 fee rebate)
```

##### Scenario C: 0% Fill (Order Cancelled)

```
Prime fills: $0
User paid: $10.00 but got nothing ✗
You earned: $0
Rebate owed: $10.00 (full refund including fee)
```

#### The Solution

When an order reaches a **terminal state** (FILLED, CANCELLED, or REJECTED), calculate:

1. **Actual Filled Value:** `cum_qty × avg_px` (from Prime)
2. **Fee Rate:** `markup_amount / user_requested_amount`
3. **Actual User Cost:** `actual_filled_value / (1 - fee_rate)`
4. **Actual Earned Fee:** `actual_user_cost × fee_rate`
5. **Rebate Amount:** `markup_amount - actual_earned_fee`

#### Implementation

```go
func (h *DbOrderHandler) calculateFeeSettlement(
    cumQty, avgPx,
    userRequestedAmount, markupAmount,
    primeOrderAmount string,
) FeeSettlement {

    // Parse inputs
    cumQtyDec, _ := decimal.NewFromString(cumQty)
    avgPxDec, _ := decimal.NewFromString(avgPx)
    markupAmountDec, _ := decimal.NewFromString(markupAmount)
    userRequestedDec, _ := decimal.NewFromString(userRequestedAmount)

    // Calculate actual filled notional value
    actualFilledValue := cumQtyDec.Mul(avgPxDec)

    // Calculate the fee rate from the original order
    feeRate := markupAmountDec.Div(userRequestedDec)

    // Calculate what the user should actually pay (including our fee)
    // If fee_rate = 0.005 and filled = $4.975:
    // actual_user_cost = $4.975 / (1 - 0.005) = $4.975 / 0.995 = $5.00
    oneMinusFeeRate := decimal.NewFromInt(1).Sub(feeRate)
    actualUserCost := actualFilledValue.Div(oneMinusFeeRate)

    // Calculate the actual fee we earned
    // actual_earned_fee = $5.00 × 0.005 = $0.025
    actualEarnedFee := actualUserCost.Mul(feeRate)

    // Cap earned fee at markup amount (can't earn more than we held)
    if actualEarnedFee.GreaterThan(markupAmountDec) {
        actualEarnedFee = markupAmountDec
    }

    // Calculate rebate
    // rebate = $0.05 - $0.025 = $0.025
    rebateAmount := markupAmountDec.Sub(actualEarnedFee)

    return FeeSettlement{
        ActualFilledValue: actualFilledValue.String(),
        ActualEarnedFee:   actualEarnedFee.String(),
        RebateAmount:      rebateAmount.String(),
    }
}
```

#### Math Verification

**50% Fill with 50 bps fee:**

```
User requested: $10.00
Markup held: $0.05 (0.5%)
Sent to Prime: $9.95
Prime filled: $4.975 (50% fill)

Calculation:
  fee_rate = $0.05 / $10.00 = 0.005
  actual_filled_value = $4.975
  actual_user_cost = $4.975 / (1 - 0.005) = $4.975 / 0.995 = $5.00
  actual_earned_fee = $5.00 × 0.005 = $0.025
  rebate_amount = $0.05 - $0.025 = $0.025

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

## Complete Integration Example

Here's how to build a complete crypto trading app with marked-up fees:

### Step 1: Configuration

Create `config.yaml`:

```yaml
prime:
  access_key: ${PRIME_ACCESS_KEY}
  passphrase: ${PRIME_PASSPHRASE}
  signing_key: ${PRIME_SIGNING_KEY}
  portfolio: ${PRIME_PORTFOLIO}
  service_account_id: ${PRIME_SERVICE_ACCOUNT_ID}

market_data:
  products: [BTC-USD, ETH-USD]
  websocket_url: wss://ws-feed.prime.coinbase.com
  max_levels: 5

fees:
  buy:
    type: percent
    percent: "0.005"  # 50 bps
  sell:
    type: percent
    percent: "0.003"  # 30 bps

database:
  path: orders.db

server:
  log_level: info
  log_json: true
```

### Step 2: Show Live Prices (with Fees)

```go
package main

import (
    "github.com/coinbase-samples/prime-trading-fees-go/config"
    "github.com/coinbase-samples/prime-trading-fees-go/internal/fees"
    "github.com/coinbase-samples/prime-trading-fees-go/internal/marketdata"
)

func main() {
    // Load config
    cfg, _ := config.LoadConfig("config.yaml")
    config.SetupLogger(cfg.Server.LogLevel, cfg.Server.LogJSON)

    // Create fee strategy
    feeStrategy, _ := fees.CreateFeeStrategy(cfg.Fees)
    adjuster := fees.NewPriceAdjuster(feeStrategy)

    // Start market data feed
    store := marketdata.NewOrderBookStore()
    wsClient := marketdata.NewWebSocketClient(wsConfig, store)
    go wsClient.Connect()

    // Display prices every 5 seconds
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        book := store.GetOrderBook("BTC-USD")
        displayPrices(book, adjuster)
    }
}
```

### Step 3: Order Preview

```go
func previewOrder(userRequest OrderRequest) (*OrderPreviewResponse, error) {
    // Create order service
    orderService := order.NewOrderServiceWithPrime(cfg, adjuster, nil)

    // Generate preview
    preview, err := orderService.GeneratePreview(ctx, userRequest)
    if err != nil {
        return nil, err
    }

    // Show user the preview with all fees disclosed
    fmt.Printf("You'll pay: $%s total\n", preview.UserRequestedAmount)
    fmt.Printf("  - Crypto value: $%s\n", preview.RawPrimePreview.FilledValue)
    fmt.Printf("  - Prime fees: $%s\n", preview.RawPrimePreview.TotalFees)
    fmt.Printf("  - Our markup: $%s\n", preview.CustomFeeOverlay.MarkupAmount)
    fmt.Printf("Effective price: $%s\n", preview.FinalBreakdown.EffectivePrice)

    return preview, nil
}
```

### Step 4: Place Order

```go
func placeOrder(userRequest OrderRequest) (*OrderResponse, error) {
    // Create order service with metadata store
    metadataStore := orders.NewMetadataStore()
    orderService := order.NewOrderServiceWithPrime(cfg, adjuster, metadataStore)

    // Place order (automatically deducts fee and stores metadata)
    response, err := orderService.PlaceOrder(ctx, userRequest)
    if err != nil {
        return nil, err
    }

    fmt.Printf("Order placed: %s\n", response.OrderId)
    fmt.Printf("Tracking execution via websocket...\n")

    return response, nil
}
```

### Step 5: Track Execution

```go
func trackOrders() {
    // Open database
    db, _ := database.NewOrdersDb(cfg.Database.Path)
    defer db.Close()

    // Create metadata store (shared with order placement)
    metadataStore := orders.NewMetadataStore()

    // Create order handler with fee settlement
    handler := orders.NewDbOrderHandler(db, adjuster, metadataStore)

    // Start websocket client
    wsClient := orders.NewWebSocketClient(wsConfig, handler)
    wsClient.Connect()

    // Orders are automatically tracked and settled
}
```

### Step 6: Check Rebates

Query the database to see fee settlements:

```sql
SELECT
    order_id,
    status,
    user_requested_amount,
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

### Fee Settlement Examples

The fee settlement logic handles three scenarios:

**Example 1: 100% Fill (Full Execution)**
```
User requested: $10.00
Fee held upfront: $0.05 (50 bps)
Prime filled: $9.95 (100%)

Settlement:
  → Earned fee: $0.05
  → Rebate: $0.00 (no refund needed)
```

**Example 2: 50% Fill (Partial Execution)**
```
User requested: $10.00
Fee held upfront: $0.05 (50 bps)
Prime filled: $4.98 (50%)

Settlement:
  → Earned fee: $0.025 (0.5% of $5.00 actual cost)
  → Rebate: $0.025 (refund unfilled portion)
  → User pays: $5.00 total ($4.975 BTC + $0.025 fee)
```

**Example 3: 0% Fill (Order Cancelled)**
```
User requested: $10.00
Fee held upfront: $0.05 (50 bps)
Prime filled: $0.00 (0%)

Settlement:
  → Earned fee: $0.00
  → Rebate: $0.05 (full refund including fee)
```

### Manual Testing Checklist

**Market Data:**
- [ ] Raw Prime prices stream correctly
- [ ] Buy prices adjusted UP by fee percentage
- [ ] Sell prices adjusted DOWN by fee percentage
- [ ] Spreads widen appropriately

**Order Preview:**
- [ ] Quote-denominated preview deducts fee upfront
- [ ] Base-denominated preview adds fee on top
- [ ] Response shows user_requested_amount correctly
- [ ] Effective price includes all fees

**Order Placement:**
- [ ] Metadata stored in database before order sent
- [ ] Prime receives reduced amount (user_amount - markup)
- [ ] Order ID tracked correctly

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

---

## Summary

You now know how to build a complete fee passthrough layer on Prime:

1. **Market Data** - Show fee-adjusted prices to users
2. **Order Preview** - Deduct fees upfront for quote orders
3. **Order Placement** - Store metadata for settlement
4. **Fee Settlement** - Fairly rebate partial fills
5. **Testing** - Verify all scenarios work correctly

**Key Principles:**

✅ **Transparency** - Users see all fees upfront
✅ **Fairness** - Fees proportional to execution
✅ **Precision** - Use `decimal.Decimal` for all money
✅ **Auditability** - Track everything in database

**Next Steps:**

1. Implement rebate processing (actual refunds to users)
2. Add monitoring/alerting for unsettled fees
3. Build admin dashboard for fee tracking
4. Test with real orders in Prime sandbox
5. Add reconciliation process for daily settlement

---

## Reference

### Configuration

See `config.yaml` for complete configuration options.

### Code Structure

```
internal/
├── fees/         - Fee strategy implementations
├── marketdata/   - Order book + WebSocket for prices
├── order/        - Order preview and placement (REST API)
├── orders/       - Order tracking (WebSocket + database)
└── database/     - SQLite persistence

cmd/
├── stream/           - Market data streaming
├── order/            - Order preview/placement
└── orders-stream/    - Order execution tracking
```

### Database Schema

See `internal/database/orders.go` for complete schema definition.

### API Documentation

- [Prime API Docs](https://docs.cdp.coinbase.com/prime/docs)
- [Prime Go SDK](https://github.com/coinbase-samples/prime-sdk-go)
