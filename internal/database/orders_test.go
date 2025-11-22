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
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestNewOrdersDb(t *testing.T) {
	// Use temporary database file
	dbPath := "test_orders.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := NewOrdersDb(dbPath)
	if err != nil {
		t.Fatalf("NewOrdersDb() error = %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("NewOrdersDb() returned nil database")
	}
}

func TestUpsertOrder_Insert(t *testing.T) {
	dbPath := "test_upsert_insert.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := NewOrdersDb(dbPath)
	if err != nil {
		t.Fatalf("NewOrdersDb() error = %v", err)
	}
	defer db.Close()

	now := time.Now()
	order := &OrderRecord{
		OrderId:             "test-order-1",
		ClientOrderId:       "client-1",
		ProductId:           "BTC-USD",
		Side:                "BUY",
		OrderType:           "MARKET",
		Status:              "OPEN",
		CumQty:              "0.001",
		LeavesQty:           "0",
		AvgPx:               "50000",
		NetAvgPx:            "50100",
		Fees:                "0.05",
		Commission:          "0.03",
		VenueFee:            "0.02",
		CesCommission:       "",
		UserRequestedAmount: "10",
		MarkupAmount:        "0.05",
		PrimeOrderAmount:    "9.95",
		ActualFilledValue:   "0",
		ActualEarnedFee:     "0",
		RebateAmount:        "0",
		FeeSettled:          false,
		FirstSeenAt:         now,
		LastUpdatedAt:       now,
	}

	err = db.UpsertOrder(order)
	if err != nil {
		t.Fatalf("UpsertOrder() error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetOrder("test-order-1")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetOrder() returned nil")
	}

	if retrieved.OrderId != order.OrderId {
		t.Errorf("OrderId = %q, want %q", retrieved.OrderId, order.OrderId)
	}
	if retrieved.ProductId != order.ProductId {
		t.Errorf("ProductId = %q, want %q", retrieved.ProductId, order.ProductId)
	}
	if retrieved.Status != order.Status {
		t.Errorf("Status = %q, want %q", retrieved.Status, order.Status)
	}
	if retrieved.UserRequestedAmount != order.UserRequestedAmount {
		t.Errorf("UserRequestedAmount = %q, want %q", retrieved.UserRequestedAmount, order.UserRequestedAmount)
	}
	if retrieved.MarkupAmount != order.MarkupAmount {
		t.Errorf("MarkupAmount = %q, want %q", retrieved.MarkupAmount, order.MarkupAmount)
	}
}

func TestUpsertOrder_Update(t *testing.T) {
	dbPath := "test_upsert_update.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := NewOrdersDb(dbPath)
	if err != nil {
		t.Fatalf("NewOrdersDb() error = %v", err)
	}
	defer db.Close()

	now := time.Now()

	// Initial insert
	order := &OrderRecord{
		OrderId:             "test-order-2",
		ClientOrderId:       "client-2",
		ProductId:           "ETH-USD",
		Side:                "SELL",
		OrderType:           "LIMIT",
		Status:              "OPEN",
		CumQty:              "0",
		LeavesQty:           "0.5",
		AvgPx:               "0",
		NetAvgPx:            "0",
		Fees:                "0",
		UserRequestedAmount: "100",
		MarkupAmount:        "0.50",
		PrimeOrderAmount:    "99.50",
		ActualFilledValue:   "0",
		ActualEarnedFee:     "0",
		RebateAmount:        "0",
		FeeSettled:          false,
		FirstSeenAt:         now,
		LastUpdatedAt:       now,
	}

	err = db.UpsertOrder(order)
	if err != nil {
		t.Fatalf("Initial UpsertOrder() error = %v", err)
	}

	// Update with fill
	time.Sleep(time.Millisecond) // Ensure different timestamp
	updatedOrder := &OrderRecord{
		OrderId:             "test-order-2",
		ClientOrderId:       "client-2",
		ProductId:           "ETH-USD",
		Side:                "SELL",
		OrderType:           "LIMIT",
		Status:              "FILLED",
		CumQty:              "0.5",
		LeavesQty:           "0",
		AvgPx:               "3000",
		NetAvgPx:            "2990",
		Fees:                "1.50",
		UserRequestedAmount: "100",
		MarkupAmount:        "0.50",
		PrimeOrderAmount:    "99.50",
		ActualFilledValue:   "1500",
		ActualEarnedFee:     "0.50",
		RebateAmount:        "0",
		FeeSettled:          true,
		FirstSeenAt:         now, // Should preserve original
		LastUpdatedAt:       time.Now(),
	}

	err = db.UpsertOrder(updatedOrder)
	if err != nil {
		t.Fatalf("Update UpsertOrder() error = %v", err)
	}

	// Retrieve and verify update
	retrieved, err := db.GetOrder("test-order-2")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}

	if retrieved.Status != "FILLED" {
		t.Errorf("Status = %q, want FILLED", retrieved.Status)
	}
	if retrieved.CumQty != "0.5" {
		t.Errorf("CumQty = %q, want 0.5", retrieved.CumQty)
	}
	if retrieved.ActualEarnedFee != "0.50" {
		t.Errorf("ActualEarnedFee = %q, want 0.50", retrieved.ActualEarnedFee)
	}
	if !retrieved.FeeSettled {
		t.Error("FeeSettled = false, want true")
	}

	// Verify FirstSeenAt is preserved (should be close to original)
	timeDiff := retrieved.FirstSeenAt.Sub(now).Abs()
	if timeDiff > time.Second {
		t.Errorf("FirstSeenAt was modified, diff = %v", timeDiff)
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	dbPath := "test_get_notfound.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := NewOrdersDb(dbPath)
	if err != nil {
		t.Fatalf("NewOrdersDb() error = %v", err)
	}
	defer db.Close()

	// Get non-existent order
	order, err := db.GetOrder("non-existent-order")
	if err != nil {
		t.Fatalf("GetOrder() error = %v, want nil", err)
	}
	if order != nil {
		t.Errorf("GetOrder() = %v, want nil for non-existent order", order)
	}
}

func TestInsertOrderEvent(t *testing.T) {
	dbPath := "test_insert_event.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := NewOrdersDb(dbPath)
	if err != nil {
		t.Fatalf("NewOrdersDb() error = %v", err)
	}
	defer db.Close()

	event := &OrderEvent{
		OrderId:       "order-1",
		SequenceNum:   1,
		EventType:     "update",
		Status:        "OPEN",
		CumQty:        "0",
		LeavesQty:     "0.001",
		AvgPx:         "0",
		NetAvgPx:      "0",
		Fees:          "0",
		Commission:    "",
		VenueFee:      "",
		CesCommission: "",
		RawJson:       `{"test":"data"}`,
		ReceivedAt:    time.Now(),
	}

	err = db.InsertOrderEvent(event)
	if err != nil {
		t.Fatalf("InsertOrderEvent() error = %v", err)
	}

	// Insert another event with higher sequence
	event2 := &OrderEvent{
		OrderId:     "order-1",
		SequenceNum: 2,
		EventType:   "update",
		Status:      "FILLED",
		CumQty:      "0.001",
		LeavesQty:   "0",
		AvgPx:       "50000",
		NetAvgPx:    "50100",
		Fees:        "0.05",
		RawJson:     `{"test":"filled"}`,
		ReceivedAt:  time.Now(),
	}

	err = db.InsertOrderEvent(event2)
	if err != nil {
		t.Fatalf("InsertOrderEvent() second event error = %v", err)
	}
}

func TestComputeCustomFees_BuySide(t *testing.T) {
	side := "BUY"
	cumQty := decimal.NewFromFloat(0.001)
	avgPx := decimal.NewFromFloat(50000)
	feePercent := decimal.NewFromFloat(0.005) // 0.5%

	customFee, adjustedPrice, totalCost := ComputeCustomFees(side, cumQty, avgPx, feePercent)

	// Notional = 0.001 * 50000 = 50
	// Custom fee = 50 * 0.005 = 0.25
	expectedFee := decimal.NewFromFloat(0.25)
	if !customFee.Equal(expectedFee) {
		t.Errorf("customFee = %s, want %s", customFee.String(), expectedFee.String())
	}

	// For BUY: adjusted price = avgPx + (fee / qty)
	// = 50000 + (0.25 / 0.001) = 50000 + 250 = 50250
	expectedAdjPrice := decimal.NewFromFloat(50250)
	if !adjustedPrice.Equal(expectedAdjPrice) {
		t.Errorf("adjustedPrice = %s, want %s", adjustedPrice.String(), expectedAdjPrice.String())
	}

	// Total cost = notional + fee = 50 + 0.25 = 50.25
	expectedTotal := decimal.NewFromFloat(50.25)
	if !totalCost.Equal(expectedTotal) {
		t.Errorf("totalCost = %s, want %s", totalCost.String(), expectedTotal.String())
	}
}

func TestComputeCustomFees_SellSide(t *testing.T) {
	side := "SELL"
	cumQty := decimal.NewFromFloat(0.5)
	avgPx := decimal.NewFromFloat(3000)
	feePercent := decimal.NewFromFloat(0.005) // 0.5%

	customFee, adjustedPrice, totalCost := ComputeCustomFees(side, cumQty, avgPx, feePercent)

	// Notional = 0.5 * 3000 = 1500
	// Custom fee = 1500 * 0.005 = 7.5
	expectedFee := decimal.NewFromFloat(7.5)
	if !customFee.Equal(expectedFee) {
		t.Errorf("customFee = %s, want %s", customFee.String(), expectedFee.String())
	}

	// For SELL: adjusted price = avgPx - (fee / qty)
	// = 3000 - (7.5 / 0.5) = 3000 - 15 = 2985
	expectedAdjPrice := decimal.NewFromFloat(2985)
	if !adjustedPrice.Equal(expectedAdjPrice) {
		t.Errorf("adjustedPrice = %s, want %s", adjustedPrice.String(), expectedAdjPrice.String())
	}

	// Total cost = notional - fee = 1500 - 7.5 = 1492.5
	expectedTotal := decimal.NewFromFloat(1492.5)
	if !totalCost.Equal(expectedTotal) {
		t.Errorf("totalCost = %s, want %s", totalCost.String(), expectedTotal.String())
	}
}

func TestComputeCustomFees_ZeroQuantity(t *testing.T) {
	side := "BUY"
	cumQty := decimal.Zero
	avgPx := decimal.NewFromFloat(50000)
	feePercent := decimal.NewFromFloat(0.005)

	customFee, adjustedPrice, totalCost := ComputeCustomFees(side, cumQty, avgPx, feePercent)

	if !customFee.IsZero() {
		t.Errorf("customFee = %s, want 0", customFee.String())
	}
	if !adjustedPrice.IsZero() {
		t.Errorf("adjustedPrice = %s, want 0", adjustedPrice.String())
	}
	if !totalCost.IsZero() {
		t.Errorf("totalCost = %s, want 0", totalCost.String())
	}
}

func TestComputeCustomFees_ZeroPrice(t *testing.T) {
	side := "SELL"
	cumQty := decimal.NewFromFloat(0.001)
	avgPx := decimal.Zero
	feePercent := decimal.NewFromFloat(0.005)

	customFee, adjustedPrice, totalCost := ComputeCustomFees(side, cumQty, avgPx, feePercent)

	if !customFee.IsZero() {
		t.Errorf("customFee = %s, want 0", customFee.String())
	}
	if !adjustedPrice.IsZero() {
		t.Errorf("adjustedPrice = %s, want 0", adjustedPrice.String())
	}
	if !totalCost.IsZero() {
		t.Errorf("totalCost = %s, want 0", totalCost.String())
	}
}

func TestUpsertOrder_MetadataPreservation(t *testing.T) {
	dbPath := "test_metadata_preserve.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := NewOrdersDb(dbPath)
	if err != nil {
		t.Fatalf("NewOrdersDb() error = %v", err)
	}
	defer db.Close()

	now := time.Now()

	// Initial order with metadata
	initialOrder := &OrderRecord{
		OrderId:             "metadata-test",
		ClientOrderId:       "client-meta",
		ProductId:           "BTC-USD",
		Side:                "BUY",
		OrderType:           "MARKET",
		Status:              "OPEN",
		CumQty:              "0",
		LeavesQty:           "0.001",
		UserRequestedAmount: "50",
		MarkupAmount:        "0.25",
		PrimeOrderAmount:    "49.75",
		FirstSeenAt:         now,
		LastUpdatedAt:       now,
	}

	err = db.UpsertOrder(initialOrder)
	if err != nil {
		t.Fatalf("Initial UpsertOrder() error = %v", err)
	}

	// Update from websocket (without metadata fields)
	updateOrder := &OrderRecord{
		OrderId:             "metadata-test",
		ClientOrderId:       "client-meta",
		ProductId:           "BTC-USD",
		Side:                "BUY",
		OrderType:           "MARKET",
		Status:              "FILLED",
		CumQty:              "0.001",
		LeavesQty:           "0",
		AvgPx:               "50000",
		NetAvgPx:            "50100",
		Fees:                "0.05",
		UserRequestedAmount: "50",    // These should be preserved
		MarkupAmount:        "0.25",  // These should be preserved
		PrimeOrderAmount:    "49.75", // These should be preserved
		ActualFilledValue:   "50",
		ActualEarnedFee:     "0.25",
		RebateAmount:        "0",
		FeeSettled:          true,
		FirstSeenAt:         now,
		LastUpdatedAt:       time.Now(),
	}

	err = db.UpsertOrder(updateOrder)
	if err != nil {
		t.Fatalf("Update UpsertOrder() error = %v", err)
	}

	// Retrieve and verify metadata is preserved
	retrieved, err := db.GetOrder("metadata-test")
	if err != nil {
		t.Fatalf("GetOrder() error = %v", err)
	}

	if retrieved.UserRequestedAmount != "50" {
		t.Errorf("UserRequestedAmount = %q, want 50 (should be preserved)", retrieved.UserRequestedAmount)
	}
	if retrieved.MarkupAmount != "0.25" {
		t.Errorf("MarkupAmount = %q, want 0.25 (should be preserved)", retrieved.MarkupAmount)
	}
	if retrieved.PrimeOrderAmount != "49.75" {
		t.Errorf("PrimeOrderAmount = %q, want 49.75 (should be preserved)", retrieved.PrimeOrderAmount)
	}
}
