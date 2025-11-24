# Prime Trading Fees Manager

A reference implementation for adding custom fee markup on top of Coinbase Prime trading operations.

> **IMPORTANT**: This is a sample application for demonstration purposes only. Test thoroughly before any production use.

## How Fee Passthrough Works

This app demonstrates the complete flow for marking up Prime trades:

```
1. User Input          →  User requests: "Buy $100 of BTC"
2. Your Fee Deduction  →  Deduct your fee: $100 - $0.50 (0.5%) = $99.50
3. Place Prime Order   →  Send $99.50 to Prime
4. Prime Execution     →  Prime fills $99.50, takes their fee ($0.10)
5. Return to User      →  User gets $99.40 worth of BTC (paid $100 total)
6. Settlement Logic    →  If partial fill, calculate fair refund
```

**Result:** You earn $0.50, Prime earns $0.10, user gets $99.40 of BTC for their $100.

## What It Does

Shows how to build this fee passthrough layer:
- **Stream market data** with your fees included in displayed prices
- **Preview orders** to show users total cost before trading
- **Track execution** via WebSocket with automatic fee settlement
- **Handle partial fills** fairly with proportional refunds

Supports flexible fee strategies: flat or percentage-based.

## Quick Start

### 1. Setup

```bash
git clone <repository-url>
cd prime-trading-fees-go
go mod download
```

### 2. Configure Credentials

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
FEE_TYPE=percent
FEE_PERCENT=0.005    # 50 bps (0.5%) for all orders
```

### 3. Run Examples

**1. Stream market data with fee-adjusted prices:**
```bash
go run cmd/stream/main.go
```

This displays Prime's live order book with **your fees already included** in the prices. Updates refresh every 5 seconds (configurable in `config.yaml` via `display_update_rate`). The displayed prices are calculated in real-time by adding your markup to Prime's WebSocket data feed.

**2. Preview an order (simulates execution):**
```bash
# Quote-denominated (default for buys): "buy $100 worth of BTC"
go run cmd/order/main.go --symbol=BTC-USD --side=buy --qty=100 --mode=preview

# Base-denominated (default for sells): "sell 0.5 BTC"
go run cmd/order/main.go --symbol=BTC-USD --side=sell --qty=0.5 --mode=preview
```

Preview mode calls Prime's API to **simulate** what would happen if you placed this order right now, showing estimated execution price, fees, and total cost based on current market conditions. No actual order is placed.

**3. Track order execution (start this BEFORE placing real orders):**
```bash
go run cmd/orders-stream/main.go --symbols=BTC-USD,ETH-USD
```

This WebSocket client listens for order updates in real-time. **You must start this before placing orders** so you don't miss any execution updates. It automatically calculates fee settlements for partial fills. Leave this running in a separate terminal.

**4. Place an actual order:**
```bash
go run cmd/order/main.go --symbol=BTC-USD --side=buy --qty=100 --mode=execute
```

This places a real order with Prime. **Prerequisite:** The orders WebSocket (#3) must already be running to capture execution updates and handle fee settlement. You'll see real-time updates in the WebSocket terminal as the order executes.

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
- Fee passthrough architecture and business model
- Fee calculation strategies (percent, flat)
- Implementing all three endpoints (market data, preview, placement)
- Fee settlement for partial fills
- Complete code examples
- Testing and verification

## Architecture

```
cmd/
├── stream/           Market data streaming with fee-adjusted prices
├── order/            Order preview and placement
└── orders-stream/    Order execution tracking via websocket

internal/
├── fees/             Fee strategy implementations
├── marketdata/       Order book management + websocket
├── order/            Order preview/placement (REST API)
├── orders/           Order tracking (websocket + database)
└── database/         SQLite persistence

config/               Configuration management
```

## Testing

```bash
go test ./...
```

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

Automatically calculates fair rebates when orders partially fill:

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

**Flat Fee:**
```yaml
fees:
  type: flat
  amount: "2.99"  # Fixed $2.99 per trade
```

## Building

```bash
go build -o bin/stream ./cmd/stream
go build -o bin/order ./cmd/order
go build -o bin/orders-stream ./cmd/orders-stream

./bin/stream
./bin/order --symbol=BTC-USD --side=buy --qty=100 --mode=preview
```

## License

Licensed under the Apache License, Version 2.0.

## Resources

- **[Integration Guide](GUIDE.md)** - Complete implementation guide
- [Prime API Documentation](https://docs.cdp.coinbase.com/prime/docs)
- [Prime Go SDK](https://github.com/coinbase-samples/prime-sdk-go)
