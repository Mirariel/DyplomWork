package models_test

import (
	"testing"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func newTestSmartOrder(userID int64, soType string) *models.SmartOrder {
	o := &models.SmartOrder{
		UserID:   userID,
		Exchange: "binance",
		Symbol:   "BTC",
		Side:     "sell",
		Category: "spot",
		Quantity: 0.05,
		Type:     soType,
	}
	switch soType {
	case "stop_loss", "take_profit":
		o.TriggerPrice = 60000.0
	case "trailing_stop":
		o.CallbackRate = 2.0
		o.PeakPrice = 65000.0
	}
	return o
}

func TestSmartOrderRepository_CreateAndGet(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so1")
	repo := models.NewSmartOrderRepository(testDB)

	o := newTestSmartOrder(user.ID, "stop_loss")
	if err := repo.Create(o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := repo.GetByID(o.ID, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("Status: got %q, want active", got.Status)
	}
	if got.Type != "stop_loss" {
		t.Errorf("Type: got %q, want stop_loss", got.Type)
	}
	if got.TriggerPrice != 60000.0 {
		t.Errorf("TriggerPrice: got %v, want 60000", got.TriggerPrice)
	}
}

func TestSmartOrderRepository_ListActive(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so2")
	repo := models.NewSmartOrderRepository(testDB)

	o1 := newTestSmartOrder(user.ID, "stop_loss")
	o2 := newTestSmartOrder(user.ID, "take_profit")
	_ = repo.Create(o1)
	_ = repo.Create(o2)
	_ = repo.Cancel(o2.ID, user.ID)

	active, err := repo.ListActive()
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ListActive count: got %d, want 1", len(active))
	}
	if active[0].ID != o1.ID {
		t.Errorf("ListActive[0].ID: got %d, want %d", active[0].ID, o1.ID)
	}
}

func TestSmartOrderRepository_ListByUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so3")
	repo := models.NewSmartOrderRepository(testDB)

	for range 3 {
		o := newTestSmartOrder(user.ID, "stop_loss")
		_ = repo.Create(o)
	}

	list, err := repo.ListByUser(user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ListByUser: got %d, want 3", len(list))
	}
}

func TestSmartOrderRepository_Cancel(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so4")
	repo := models.NewSmartOrderRepository(testDB)

	o := newTestSmartOrder(user.ID, "take_profit")
	_ = repo.Create(o)

	if err := repo.Cancel(o.ID, user.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, _ := repo.GetByID(o.ID, user.ID)
	if got.Status != "cancelled" {
		t.Errorf("Status after Cancel: got %q, want cancelled", got.Status)
	}

	// Cancelling again should be a no-op (already not active).
	if err := repo.Cancel(o.ID, user.ID); err != nil {
		t.Errorf("second Cancel: unexpected error: %v", err)
	}
}

func TestSmartOrderRepository_MarkTriggered(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so5")
	soRepo := models.NewSmartOrderRepository(testDB)
	oRepo := models.NewOrderRepository(testDB)

	// Create the market order that will be linked.
	order := &models.Order{
		UserID: user.ID, Exchange: "binance", Symbol: "BTC",
		Side: "sell", Type: "market", Category: "spot",
		Quantity: 0.05, Status: "filled",
	}
	_ = oRepo.Create(order)

	so := newTestSmartOrder(user.ID, "stop_loss")
	_ = soRepo.Create(so)

	if err := soRepo.MarkTriggered(so.ID, order.ID); err != nil {
		t.Fatalf("MarkTriggered: %v", err)
	}

	got, _ := soRepo.GetByID(so.ID, user.ID)
	if got.Status != "triggered" {
		t.Errorf("Status: got %q, want triggered", got.Status)
	}
	if got.TriggeredOrderID == nil || *got.TriggeredOrderID != order.ID {
		t.Errorf("TriggeredOrderID: got %v, want %d", got.TriggeredOrderID, order.ID)
	}
}

func TestSmartOrderRepository_MarkFailed(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so6")
	repo := models.NewSmartOrderRepository(testDB)

	o := newTestSmartOrder(user.ID, "stop_loss")
	_ = repo.Create(o)

	if err := repo.MarkFailed(o.ID, "exchange error"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	got, _ := repo.GetByID(o.ID, user.ID)
	if got.Status != "failed" {
		t.Errorf("Status: got %q, want failed", got.Status)
	}
	if got.ErrorMessage == nil || *got.ErrorMessage != "exchange error" {
		t.Errorf("ErrorMessage: got %v, want 'exchange error'", got.ErrorMessage)
	}
}

func TestSmartOrderRepository_UpdatePeak(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "so7")
	repo := models.NewSmartOrderRepository(testDB)

	o := newTestSmartOrder(user.ID, "trailing_stop")
	_ = repo.Create(o)

	if err := repo.UpdatePeak(o.ID, 70000.0); err != nil {
		t.Fatalf("UpdatePeak: %v", err)
	}

	got, _ := repo.GetByID(o.ID, user.ID)
	if got.PeakPrice != 70000.0 {
		t.Errorf("PeakPrice: got %v, want 70000", got.PeakPrice)
	}
}
