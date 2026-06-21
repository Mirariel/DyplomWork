package models_test

import (
	"testing"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func newTestBot(userID int64) *models.Bot {
	return &models.Bot{
		UserID:     userID,
		Exchange:   "bybit",
		Symbol:     "ETH",
		Category:   "spot",
		PriceLow:   2000.0,
		PriceHigh:  3000.0,
		Grids:      5,
		QtyPerGrid: 0.1,
	}
}

func TestBotRepository_CreateAndGet(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "b1")
	repo := models.NewBotRepository(testDB)

	b := newTestBot(user.ID)
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
	if got.Symbol != "ETH" {
		t.Errorf("Symbol: got %q, want ETH", got.Symbol)
	}
	if got.PriceLow != 2000.0 {
		t.Errorf("PriceLow: got %v, want 2000", got.PriceLow)
	}
	if got.Grids != 5 {
		t.Errorf("Grids: got %d, want 5", got.Grids)
	}
}

func TestBotRepository_GetByID_WrongUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "b2")
	repo := models.NewBotRepository(testDB)

	b := newTestBot(user.ID)
	_ = repo.Create(b)

	if _, err := repo.GetByID(b.ID, user.ID+1); err == nil {
		t.Error("expected error for wrong userID")
	}
}

func TestBotRepository_ListByUser(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "b3")
	repo := models.NewBotRepository(testDB)

	_ = repo.Create(newTestBot(user.ID))
	_ = repo.Create(newTestBot(user.ID))

	list, err := repo.ListByUser(user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByUser: got %d, want 2", len(list))
	}
}

func TestBotRepository_SetStatusAndListRunning(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "b4")
	repo := models.NewBotRepository(testDB)

	b := newTestBot(user.ID)
	_ = repo.Create(b)

	running, _ := repo.ListRunning()
	if len(running) != 0 {
		t.Errorf("ListRunning before SetStatus: got %d, want 0", len(running))
	}

	if err := repo.SetStatus(b.ID, "running", nil); err != nil {
		t.Fatalf("SetStatus running: %v", err)
	}

	running, err := repo.ListRunning()
	if err != nil {
		t.Fatalf("ListRunning: %v", err)
	}
	if len(running) != 1 {
		t.Errorf("ListRunning after SetStatus: got %d, want 1", len(running))
	}

	errMsg := "test error"
	_ = repo.SetStatus(b.ID, "stopped", &errMsg)
	got, _ := repo.GetByID(b.ID, user.ID)
	if got.Status != "stopped" {
		t.Errorf("Status: got %q, want stopped", got.Status)
	}
	if got.ErrorMessage == nil || *got.ErrorMessage != errMsg {
		t.Errorf("ErrorMessage: got %v, want %q", got.ErrorMessage, errMsg)
	}
}

func TestBotRepository_AddProfit(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "b5")
	repo := models.NewBotRepository(testDB)

	b := newTestBot(user.ID)
	_ = repo.Create(b)

	_ = repo.AddProfit(b.ID, 10.5)
	_ = repo.AddProfit(b.ID, 5.25)

	got, _ := repo.GetByID(b.ID, user.ID)
	if got.TotalProfit != 15.75 {
		t.Errorf("TotalProfit: got %v, want 15.75", got.TotalProfit)
	}
}

func TestBotRepository_Grids(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "b6")
	repo := models.NewBotRepository(testDB)

	b := newTestBot(user.ID)
	_ = repo.Create(b)

	oid1 := "EX-G-001"
	oid2 := "EX-G-002"
	grids := []models.BotGrid{
		{BotID: b.ID, Level: 0, Price: 2000, Side: "buy", ExchangeOrderID: &oid1, Status: "open"},
		{BotID: b.ID, Level: 1, Price: 2200, Side: "sell", ExchangeOrderID: &oid2, Status: "open"},
		{BotID: b.ID, Level: 2, Price: 2400, Side: "sell", Status: "pending"},
	}
	if err := repo.CreateGrids(grids); err != nil {
		t.Fatalf("CreateGrids: %v", err)
	}

	open, err := repo.ListOpenGrids(b.ID)
	if err != nil {
		t.Fatalf("ListOpenGrids: %v", err)
	}
	if len(open) != 2 {
		t.Errorf("ListOpenGrids: got %d, want 2", len(open))
	}

	// Fill the first grid.
	if err := repo.UpdateGrid(grids[0].ID, oid1, "filled"); err != nil {
		t.Fatalf("UpdateGrid: %v", err)
	}

	all, _ := repo.ListAllGrids(b.ID)
	filledCount := 0
	for _, g := range all {
		if g.Status == "filled" {
			filledCount++
			if g.FilledAt == nil {
				t.Error("FilledAt: expected non-nil after filling")
			}
		}
	}
	if filledCount != 1 {
		t.Errorf("filled grids: got %d, want 1", filledCount)
	}

	// Cancel all remaining open/pending grids.
	if err := repo.CancelAllGrids(b.ID); err != nil {
		t.Fatalf("CancelAllGrids: %v", err)
	}

	open, _ = repo.ListOpenGrids(b.ID)
	if len(open) != 0 {
		t.Errorf("ListOpenGrids after CancelAll: got %d, want 0", len(open))
	}
}
