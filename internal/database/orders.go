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

package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// OrdersDb handles order state persistence
type OrdersDb struct {
	db *sql.DB
}

// OrderRecord represents the current state of an order
type OrderRecord struct {
	// Prime order fields
	OrderId       string
	ClientOrderId string
	ProductId     string
	Side          string
	OrderType     string
	Status        string
	CumQty        string // Cumulative filled quantity (BTC received by user)
	LeavesQty     string // Remaining quantity
	AvgPx         string // Average execution price
	NetAvgPx      string // Net average price (after Prime fees)
	Fees          string // Prime fees in quote currency
	Commission    string // Prime commission
	VenueFee      string // Venue fees
	CesCommission string // CES commission

	// User's original request (for quote-denominated orders)
	UserRequestedAmount string // What the user wanted to spend (e.g., $10)
	MarkupAmount        string // What we kept upfront as fee hold (e.g., $0.05)
	PrimeOrderAmount    string // What we sent to Prime (e.g., $9.95)

	// Fee settlement (calculated at terminal state)
	ActualFilledValue string // Actual notional value filled: cum_qty * avg_px
	ActualEarnedFee   string // Fee we actually earned based on filled amount
	RebateAmount      string // Amount to rebate: markup_amount - actual_earned_fee
	FeeSettled        bool   // Whether fee has been settled

	// Metadata
	FirstSeenAt   time.Time
	LastUpdatedAt time.Time
}

// OrderEvent represents a single websocket update event
type OrderEvent struct {
	Id            int64
	OrderId       string
	SequenceNum   int64
	EventType     string // "snapshot" or "update"
	Status        string
	CumQty        string
	LeavesQty     string
	AvgPx         string
	NetAvgPx      string
	Fees          string
	Commission    string
	VenueFee      string
	CesCommission string
	RawJson       string // Full raw JSON for debugging
	ReceivedAt    time.Time
}

// NewOrdersDb creates a new orders database
func NewOrdersDb(dbPath string) (*OrdersDb, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent write performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout to handle concurrent writes
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	ordersDb := &OrdersDb{db: db}

	if err := ordersDb.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return ordersDb, nil
}

// createTables creates the database schema
func (db *OrdersDb) createTables() error {
	// Main orders table - one row per order, always current state
	ordersTable := `
	CREATE TABLE IF NOT EXISTS orders (
		order_id TEXT PRIMARY KEY,
		client_order_id TEXT NOT NULL,
		product_id TEXT NOT NULL,
		side TEXT NOT NULL,
		order_type TEXT NOT NULL,
		status TEXT NOT NULL,

		-- Execution details (raw Prime data stored as TEXT for exact decimal precision)
		cum_qty TEXT NOT NULL DEFAULT '0',
		leaves_qty TEXT NOT NULL DEFAULT '0',
		avg_px TEXT NOT NULL DEFAULT '0',
		net_avg_px TEXT NOT NULL DEFAULT '0',
		fees TEXT NOT NULL DEFAULT '0',
		commission TEXT,
		venue_fee TEXT,
		ces_commission TEXT,

		-- User's original request (for quote-denominated orders)
		user_requested_amount TEXT DEFAULT '0',
		markup_amount TEXT DEFAULT '0',
		prime_order_amount TEXT DEFAULT '0',

		-- Fee settlement (calculated at terminal state)
		actual_filled_value TEXT DEFAULT '0',
		actual_earned_fee TEXT DEFAULT '0',
		rebate_amount TEXT DEFAULT '0',
		fee_settled BOOLEAN DEFAULT FALSE,

		-- Metadata
		first_seen_at TIMESTAMP NOT NULL,
		last_updated_at TIMESTAMP NOT NULL
	);`

	// Order events table - append-only audit log of all websocket updates
	eventsTable := `
	CREATE TABLE IF NOT EXISTS order_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT NOT NULL,
		sequence_num INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		status TEXT NOT NULL,

		-- Execution snapshot at this event
		cum_qty TEXT NOT NULL,
		leaves_qty TEXT NOT NULL,
		avg_px TEXT NOT NULL,
		net_avg_px TEXT NOT NULL,
		fees TEXT NOT NULL,
		commission TEXT,
		venue_fee TEXT,
		ces_commission TEXT,

		-- Raw data for debugging
		raw_json TEXT NOT NULL,
		received_at TIMESTAMP NOT NULL
	);`

	// Create indexes separately
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_product ON orders(product_id);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_updated ON orders(last_updated_at);`,
		`CREATE INDEX IF NOT EXISTS idx_events_order ON order_events(order_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_seq ON order_events(order_id, sequence_num);`,
		`CREATE INDEX IF NOT EXISTS idx_events_received ON order_events(received_at);`,
	}

	if _, err := db.db.Exec(ordersTable); err != nil {
		return fmt.Errorf("failed to create orders table: %w", err)
	}

	if _, err := db.db.Exec(eventsTable); err != nil {
		return fmt.Errorf("failed to create order_events table: %w", err)
	}

	for _, idx := range indexes {
		if _, err := db.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// UpsertOrder updates or inserts an order record
func (db *OrdersDb) UpsertOrder(order *OrderRecord) error {
	query := `
	INSERT INTO orders (
		order_id, client_order_id, product_id, side, order_type, status,
		cum_qty, leaves_qty, avg_px, net_avg_px, fees,
		commission, venue_fee, ces_commission,
		user_requested_amount, markup_amount, prime_order_amount,
		actual_filled_value, actual_earned_fee, rebate_amount, fee_settled,
		first_seen_at, last_updated_at
	) VALUES (
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?,
		?, ?
	)
	ON CONFLICT(order_id) DO UPDATE SET
		status = excluded.status,
		cum_qty = excluded.cum_qty,
		leaves_qty = excluded.leaves_qty,
		avg_px = excluded.avg_px,
		net_avg_px = excluded.net_avg_px,
		fees = excluded.fees,
		commission = excluded.commission,
		venue_fee = excluded.venue_fee,
		ces_commission = excluded.ces_commission,
		user_requested_amount = COALESCE(NULLIF(orders.user_requested_amount, '0'), excluded.user_requested_amount),
		markup_amount = COALESCE(NULLIF(orders.markup_amount, '0'), excluded.markup_amount),
		prime_order_amount = COALESCE(NULLIF(orders.prime_order_amount, '0'), excluded.prime_order_amount),
		actual_filled_value = excluded.actual_filled_value,
		actual_earned_fee = excluded.actual_earned_fee,
		rebate_amount = excluded.rebate_amount,
		fee_settled = excluded.fee_settled,
		last_updated_at = excluded.last_updated_at
	`

	_, err := db.db.Exec(query,
		order.OrderId, order.ClientOrderId, order.ProductId, order.Side, order.OrderType, order.Status,
		order.CumQty, order.LeavesQty, order.AvgPx, order.NetAvgPx, order.Fees,
		order.Commission, order.VenueFee, order.CesCommission,
		order.UserRequestedAmount, order.MarkupAmount, order.PrimeOrderAmount,
		order.ActualFilledValue, order.ActualEarnedFee, order.RebateAmount, order.FeeSettled,
		order.FirstSeenAt, order.LastUpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert order: %w", err)
	}

	return nil
}

// InsertOrderEvent inserts a new order event
func (db *OrdersDb) InsertOrderEvent(event *OrderEvent) error {
	query := `
	INSERT INTO order_events (
		order_id, sequence_num, event_type, status,
		cum_qty, leaves_qty, avg_px, net_avg_px, fees,
		commission, venue_fee, ces_commission,
		raw_json, received_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.db.Exec(query,
		event.OrderId, event.SequenceNum, event.EventType, event.Status,
		event.CumQty, event.LeavesQty, event.AvgPx, event.NetAvgPx, event.Fees,
		event.Commission, event.VenueFee, event.CesCommission,
		event.RawJson, event.ReceivedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert order event: %w", err)
	}

	return nil
}

// GetOrder retrieves the current state of an order
func (db *OrdersDb) GetOrder(orderId string) (*OrderRecord, error) {
	query := `
	SELECT
		order_id, client_order_id, product_id, side, order_type, status,
		cum_qty, leaves_qty, avg_px, net_avg_px, fees,
		commission, venue_fee, ces_commission,
		user_requested_amount, markup_amount, prime_order_amount,
		actual_filled_value, actual_earned_fee, rebate_amount, fee_settled,
		first_seen_at, last_updated_at
	FROM orders
	WHERE order_id = ?
	`

	var order OrderRecord
	err := db.db.QueryRow(query, orderId).Scan(
		&order.OrderId, &order.ClientOrderId, &order.ProductId, &order.Side, &order.OrderType, &order.Status,
		&order.CumQty, &order.LeavesQty, &order.AvgPx, &order.NetAvgPx, &order.Fees,
		&order.Commission, &order.VenueFee, &order.CesCommission,
		&order.UserRequestedAmount, &order.MarkupAmount, &order.PrimeOrderAmount,
		&order.ActualFilledValue, &order.ActualEarnedFee, &order.RebateAmount, &order.FeeSettled,
		&order.FirstSeenAt, &order.LastUpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// ComputeCustomFees calculates custom fees for an order
func ComputeCustomFees(side string, cumQty, avgPx decimal.Decimal, feePercent decimal.Decimal) (customFee, adjustedPrice, totalCost decimal.Decimal) {
	if cumQty.IsZero() || avgPx.IsZero() {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}

	notional := cumQty.Mul(avgPx)
	customFee = notional.Mul(feePercent)

	// Adjust price based on side
	if side == "BUY" {
		// For buys, add fee to price (paying more)
		adjustedPrice = avgPx.Add(customFee.Div(cumQty))
		totalCost = notional.Add(customFee)
	} else {
		// For sells, subtract fee from price (receiving less)
		adjustedPrice = avgPx.Sub(customFee.Div(cumQty))
		totalCost = notional.Sub(customFee)
	}

	return customFee, adjustedPrice, totalCost
}

// Close closes the database connection
func (db *OrdersDb) Close() error {
	if db.db != nil {
		zap.L().Info("Closing orders database")
		return db.db.Close()
	}
	return nil
}
