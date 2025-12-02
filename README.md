# Prime Trading Fees Manager

A reference implementation for adding custom fee markup on top of Coinbase Prime trading operations. This is built on top of the [Prime Go SDK](https://github.com/coinbase-samples/prime-sdk-go). 

> **IMPORTANT**: This is a sample application for demonstration purposes only. Test thoroughly.

## How Fee Passthrough Works

This app demonstrates the complete flow for marking up Prime trades:

```
1. User Input          →  User requests: "Buy $100 of BTC"
2. Your Fee Deduction  →  Deduct your fee: $100 - $0.50 (example at 50 bps) = $99.50
3. Place Prime Order   →  Create a Prime Order for $99.50
4. Prime Execution     →  Prime fills $99.50, minus commission (example at $0.10) = $99.40
5. Return to User      →  User gets $99.40 worth of BTC (paid $100 total)
6. Settlement Logic    →  If partial fill or cancellation, handle partial fee refund
```

**Result:** You earn $0.50, Prime earns $0.10, user gets $99.40 of BTC for their $100.

## What It Does

Shows how to build this fee passthrough layer:
- **Track execution** via WebSocket with automatic fee settlement
- **Handle partial fills** fairly with proportional refunds
- **Stream market data** with your fees included in displayed prices (optional)
- **Preview orders** to show users total cost before trading (optional)

## Quick Start

### 1. Configure Credentials

Copy the example environment file and add your credentials:

```bash
cp .env.example .env
```

Then edit `.env` with your Prime API credentials:

```bash
# Prime API Credentials
PRIME_ACCESS_KEY=your_access_key
PRIME_PASSPHRASE=your_passphrase
PRIME_SIGNING_KEY=your_signing_key
PRIME_PORTFOLIO=your_portfolio_id
PRIME_SERVICE_ACCOUNT_ID=your_service_account_id

# Fee Strategy
FEE_PERCENT=0.005    # 50 bps (0.5%) for all orders
```

### 2. Install the CLI

```bash
go install ./cmd/prime
```

This installs the `prime` CLI tool to your `$GOPATH/bin` (make sure it's in your `$PATH`).

### 3. Run Examples

**1. Stream market data with fee-adjusted prices (optional):**
```bash
prime stream --symbols=BTC-USD,ETH-USD
```

This displays Prime's live order book with **your fees already included** in the prices. Updates refresh every 5 seconds (configurable in `.env` via `MARKET_DATA_DISPLAY_UPDATE_RATE`). The displayed prices are calculated in real-time by adding your markup to Prime's WebSocket data feed.

**2. Preview an order (simulates execution):**
```bash
# Quote-denominated (default for buys): "buy $100 worth of BTC"
prime order --symbol=BTC-USD --side=buy --unit=quote --qty=100 --mode=preview

# Base-denominated (default for sells): "sell 0.5 BTC"
prime order --symbol=BTC-USD --side=sell --unit=base --qty=0.5 --mode=preview
```

Preview mode calls Prime's [Create Order Preview](https://docs.cdp.coinbase.com/api-reference/prime-api/rest-api/orders/get-order-preview) API to **simulate** what would happen if you placed this order right now, showing estimated execution price, Coinbase fees, and total cost based on current market conditions. No actual order is placed.

**3. Track order execution (start this BEFORE placing real orders):**
```bash
prime orders-stream --symbols=BTC-USD,ETH-USD
```

This WebSocket client listens for order updates in real-time, specific to the products you subscribe to. **You must start this before placing orders** so you don't miss any execution updates. It automatically calculates fee settlements for partial fills. Leave this running in a separate terminal.

**4. Place an actual order:**
```bash
prime order --symbol=BTC-USD --side=buy --unit=quote --qty=100 --mode=execute
```

This places a real order with Prime. **Prerequisite:** The orders WebSocket (#3) must already be running to capture execution updates and handle fee settlement. You'll see real-time updates in the WebSocket terminal as the order executes.

**5. Request For Quote (RFQ) - Get guaranteed price before executing (optional):**
```bash
# Preview quote only
prime rfq --symbol=BTC-USD --side=buy --unit=quote --qty=1000 --price=88000

# Get quote and auto-accept
prime rfq --symbol=BTC-USD --side=buy --unit=quote --qty=1000 --price=88000 --auto-accept
```

RFQ provides a guaranteed price quote with expiration time. Unlike market orders that execute immediately, RFQ lets you see the exact execution price before deciding. **Note:** Marketable limit prices are required for all RFQ requests.

### CLI Help

Get help for any command:
```bash
prime --help
prime order --help
prime stream --help
prime orders-stream --help
prime rfq --help
```

## Sample Output

### Market Data Stream

```
═══════════════════════════════════════════════════
  BTC-USD Order Book @ 14:32:15
═══════════════════════════════════════════════════

  ASK SIZE    PRIME PRICE    YOUR PRICE (incl. 0.5% fee)
  --------    -----------    ----------------------------
  0.5000      43250.00       43466.25
  1.2000      43248.50       43464.54

  BID SIZE    PRIME PRICE    YOUR PRICE (incl. 0.5% fee)
  --------    -----------    ----------------------------
  0.7500      43245.00       43028.78
  1.5000      43243.50       43027.28
```

### Order Preview

```json
{
  "product": "BTC-USD",
  "side": "BUY",
  "type": "MARKET",
  "order_unit": "quote",
  "user_requested_amount": "100.00",
  "raw_prime_preview": {
    "quantity": "0.0023",
    "average_filled_price": "43250.00",
    "total_value": "99.50",
    "commission": "0.10"
  },
  "custom_fee_overlay": {
    "fee_amount": "0.50",
    "fee_percent": "0.50",
    "effective_price": "43478.26"
  },
  "timestamp": "2025-01-15T14:32:15Z"
}
```

## Documentation

**📖 [COMPLETE INTEGRATION GUIDE →](GUIDE.md)**

The comprehensive guide covers:
- Fee passthrough architecture
- Fee calculation strategies (percent)
- Implementing all four endpoints (market data, preview, placement, RFQ)
- Fee settlement for partial fills

## Key Features

### 1. Fee-Adjusted Market Data

Show users prices that include your markup:
- **Buy prices** (asks) adjusted UP
- **Sell prices** (bids) adjusted DOWN

### 2. Smart Order Handling

**Quote-denominated orders** ("buy $100 worth"):
- Deduct fee upfront
- Send reduced amount to Prime
- Store metadata for settlement

**Base-denominated orders** ("buy 0.5 BTC"):
- Add fee on top
- Simple pass-through

### 3. Fair Partial Fill Settlement

Automatically handles fair rebates when orders partially fill:

```
User wants: $10 worth of BTC (50 bps fee = $0.05)
Sent to Prime: $9.95
Prime fills: 50% ($4.975)

Fair settlement:
  User should pay: $5.00 ($4.975 + $0.025 fee)
  Rebate owed: $5.00 ($4.975 unfilled + $0.025 fee)
```

## Configuration

### Fee Strategies

**Percent Fee:**
```yaml
fees:
  type: percent
  percent: "0.005"  # 50 bps (0.5%)
```

## License

Licensed under the Apache License, Version 2.0.

## Resources

- **[Integration Guide](GUIDE.md)** - Complete implementation guide
- [Prime API Documentation](https://docs.cdp.coinbase.com/prime/docs)
- [Prime Go SDK](https://github.com/coinbase-samples/prime-sdk-go)
