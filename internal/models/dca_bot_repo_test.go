package models_test

import (
	"testing"
	"time"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func newTestDCABot(userID int64, nextBuyAt time.Time) *models.DCABot {
	return &models.DCABot{
		UserID:        userID,
		Exchange:      "okx",
		Symbol:        "BTC",
		Category:      "spot",
		AmountUSD:     100.0,
		IntervalHours: 24,
		NextBuyAt:     nextBuyAt,
	}
}

func TestDCABotRepository_CreateAndGet(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "d1")
	repo := models.NewDCABotRepository(testDB)

	b := newTestDCABot(user.ID, time.Now().Add(time.Hour))
	if err := repo.Create(b); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b.ID == 0 {
		t.Fatal("expected non-zero ID after Create")
	}

	got, err := repo.GetByID(b.ID, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Symbol != "BTC" {
		t.Errorf("Symbol: got %q, want BTC", got.Symbol)
	}
	if got.AmountUSD != 100.0 {
		t.Errorf("AmountUSD: got %v, want 100", got.AmountUSD)
	}
	if got.IntervalHours != 24 {
		t.Errorf("IntervalHours: got %d, want 24", got.IntervalHours)
	}
}

func TestDCABotRepository_GetByID_WrongUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "d2")
	repo := models.NewDCABotRepository(testDB)

	b := newTestDCABot(user.ID, time.Now().Add(time.Hour))
	_ = repo.Create(b)

	if _, err := repo.GetByID(b.ID, user.ID+1); err == nil {
		t.Error("expected error for wrong userID")
	}
}

func TestDCABotRepository_ListByUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "d3")
	repo := models.NewDCABotRepository(testDB)

	for range 3 {
		_ = repo.Create(newTestDCABot(user.ID, time.Now().Add(time.Hour)))
	}

	list, err := repo.ListByUser(user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ListByUser: got %d, want 3", len(list))
	}
}

func TestDCABotRepository_ListDue(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "d4")
	repo := models.NewDCABotRepository(testDB)

	// Due bot: next_buy_at 2 days in the past (safe regardless of server timezone).
	due := newTestDCABot(user.ID, time.Now().Add(-48*time.Hour))
	if err := repo.Create(due); err != nil {
		t.Fatalf("Create due: %v", err)
	}
	if err := repo.SetStatus(due.ID, "running", nil); err != nil {
		t.Fatalf("SetStatus due: %v", err)
	}

	// Not-due bot: next_buy_at 2 days in the future.
	notDue := newTestDCABot(user.ID, time.Now().Add(48*time.Hour))
	if err := repo.Create(notDue); err != nil {
		t.Fatalf("Create notDue: %v", err)
	}
	if err := repo.SetStatus(notDue.ID, "running", nil); err != nil {
		t.Fatalf("SetStatus notDue: %v", err)
	}

	// Stopped bot with past time — should NOT appear (wrong status).
	stopped := newTestDCABot(user.ID, time.Now().Add(-48*time.Hour))
	if err := repo.Create(stopped); err != nil {
		t.Fatalf("Create stopped: %v", err)
	}
	// Default status is 'stopped', but set explicitly for clarity.
	if err := repo.SetStatus(stopped.ID, "stopped", nil); err != nil {
		t.Fatalf("SetStatus stopped: %v", err)
	}

	list, err := repo.ListDue()
	if err != nil {
		t.Fatalf("ListDue: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListDue: got %d, want 1", len(list))
	}
	if list[0].ID != due.ID {
		t.Errorf("ListDue[0].ID: got %d, want %d", list[0].ID, due.ID)
	}
}

func TestDCABotRepository_SetStatus(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "d5")
	repo := models.NewDCABotRepository(testDB)

	b := newTestDCABot(user.ID, time.Now().Add(time.Hour))
	_ = repo.Create(b)

	if err := repo.SetStatus(b.ID, "running", nil); err != nil {
		t.Fatalf("SetStatus running: %v", err)
	}
	got, _ := repo.GetByID(b.ID, user.ID)
	if got.Status != "running" {
		t.Errorf("Status: got %q, want running", got.Status)
	}

	errMsg := "no price available"
	_ = repo.SetStatus(b.ID, "stopped", &errMsg)
	got, _ = repo.GetByID(b.ID, user.ID)
	if got.Status != "stopped" {
		t.Errorf("Status after stop: got %q, want stopped", got.Status)
	}
	if got.ErrorMessage == nil || *got.ErrorMessage != errMsg {
		t.Errorf("ErrorMessage: got %v, want %q", got.ErrorMessage, errMsg)
	}
}

func TestDCABotRepository_RecordBuy(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "d6")
	repo := models.NewDCABotRepository(testDB)

	b := newTestDCABot(user.ID, time.Now().Add(-time.Hour))
	_ = repo.Create(b)
	_ = repo.SetStatus(b.ID, "running", nil)

	next := time.Now().Add(24 * time.Hour)
	if err := repo.RecordBuy(b.ID, 0.0015, 100.0, next); err != nil {
		t.Fatalf("RecordBuy: %v", err)
	}

	got, _ := repo.GetByID(b.ID, user.ID)
	if got.TotalInvested != 100.0 {
		t.Errorf("TotalInvested: got %v, want 100", got.TotalInvested)
	}
	if got.TotalQuantity != 0.0015 {
		t.Errorf("TotalQuantity: got %v, want 0.0015", got.TotalQuantity)
	}
	if got.OrdersPlaced != 1 {
		t.Errorf("OrdersPlaced: got %d, want 1", got.OrdersPlaced)
	}

	// Second buy: values should accumulate.
	_ = repo.RecordBuy(b.ID, 0.0010, 100.0, time.Now().Add(48*time.Hour))
	got, _ = repo.GetByID(b.ID, user.ID)
	if got.TotalInvested != 200.0 {
		t.Errorf("TotalInvested after 2nd buy: got %v, want 200", got.TotalInvested)
	}
	if got.OrdersPlaced != 2 {
		t.Errorf("OrdersPlaced after 2nd buy: got %d, want 2", got.OrdersPlaced)
	}
}
