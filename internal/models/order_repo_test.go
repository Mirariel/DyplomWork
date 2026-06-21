package models_test

import (
	"testing"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func newTestOrder(userID int64) *models.Order {
	return &models.Order{
		UserID:   userID,
		Exchange: "binance",
		Symbol:   "BTC",
		Side:     "buy",
		Type:     "market",
		Category: "spot",
		Quantity: 0.01,
		Status:   "new",
	}
}

func TestOrderRepository_CreateAndGet(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "o1")
	repo := models.NewOrderRepository(testDB)

	o := newTestOrder(user.ID)
	if err := repo.Create(o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("expected non-zero ID after Create")
	}

	got, err := repo.GetByID(o.ID, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Symbol != "BTC" {
		t.Errorf("Symbol: got %q, want BTC", got.Symbol)
	}
	if got.Status != "new" {
		t.Errorf("Status: got %q, want new", got.Status)
	}
}

func TestOrderRepository_GetByID_WrongUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "o2")
	repo := models.NewOrderRepository(testDB)

	o := newTestOrder(user.ID)
	_ = repo.Create(o)

	if _, err := repo.GetByID(o.ID, user.ID+1); err == nil {
		t.Error("expected error when accessing order with wrong userID")
	}
}

func TestOrderRepository_UpdateStatus(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "o3")
	repo := models.NewOrderRepository(testDB)

	o := newTestOrder(user.ID)
	_ = repo.Create(o)

	if err := repo.UpdateStatus(o.ID, "EX-001", "filled", 0.01, 65000.0); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, _ := repo.GetByID(o.ID, user.ID)
	if got.Status != "filled" {
		t.Errorf("Status: got %q, want filled", got.Status)
	}
	if got.ExchangeOrderID != "EX-001" {
		t.Errorf("ExchangeOrderID: got %q, want EX-001", got.ExchangeOrderID)
	}
	if got.FilledQty != 0.01 {
		t.Errorf("FilledQty: got %v, want 0.01", got.FilledQty)
	}
	if got.AvgPrice != 65000.0 {
		t.Errorf("AvgPrice: got %v, want 65000", got.AvgPrice)
	}
}

func TestOrderRepository_MarkFailed(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "o4")
	repo := models.NewOrderRepository(testDB)

	o := newTestOrder(user.ID)
	_ = repo.Create(o)

	if err := repo.MarkFailed(o.ID, "insufficient funds"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	got, _ := repo.GetByID(o.ID, user.ID)
	if got.Status != "rejected" {
		t.Errorf("Status: got %q, want rejected", got.Status)
	}
	if got.ErrorMessage == nil || *got.ErrorMessage != "insufficient funds" {
		t.Errorf("ErrorMessage: got %v, want 'insufficient funds'", got.ErrorMessage)
	}
}

func TestOrderRepository_ListByUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "o5")
	repo := models.NewOrderRepository(testDB)

	o1 := newTestOrder(user.ID)
	o2 := newTestOrder(user.ID)
	o2.Status = "filled"
	_ = repo.Create(o1)
	_ = repo.Create(o2)
	// Give o2 a filled status via UpdateStatus so it's genuinely filled in DB.
	_ = repo.UpdateStatus(o2.ID, "EX-002", "filled", 0.01, 60000)

	all, err := repo.ListByUser(user.ID, "")
	if err != nil {
		t.Fatalf("ListByUser all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListByUser all: got %d, want 2", len(all))
	}

	filled, err := repo.ListByUser(user.ID, "filled")
	if err != nil {
		t.Fatalf("ListByUser filled: %v", err)
	}
	if len(filled) != 1 {
		t.Errorf("ListByUser filled: got %d, want 1", len(filled))
	}
}
