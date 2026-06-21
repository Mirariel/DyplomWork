package models_test

import (
	"strings"
	"testing"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func TestSnapshotRepository_UpsertAndList(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "sn1")
	repo := models.NewSnapshotRepository(testDB)

	s1 := &models.PortfolioSnapshot{
		UserID:       user.ID,
		TotalValue:   10000.0,
		SpotValue:    8000.0,
		FuturesPnl:   2000.0,
		SnapshotDate: "2026-06-01",
	}
	if err := repo.Upsert(s1); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	s2 := &models.PortfolioSnapshot{
		UserID:       user.ID,
		TotalValue:   11000.0,
		SpotValue:    9000.0,
		FuturesPnl:   2000.0,
		SnapshotDate: "2026-06-02",
	}
	_ = repo.Upsert(s2)

	list, err := repo.ListByUser(user.ID, 30)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByUser: got %d, want 2", len(list))
	}
	// Ordered by snapshot_date ASC.
	// MySQL DATE with parseTime=true may be scanned as "2026-06-01" or "2026-06-01T00:00:00Z".
	if !strings.HasPrefix(list[0].SnapshotDate, "2026-06-01") {
		t.Errorf("list[0].SnapshotDate: got %q, want prefix 2026-06-01", list[0].SnapshotDate)
	}
	if list[0].TotalValue != 10000.0 {
		t.Errorf("list[0].TotalValue: got %v, want 10000", list[0].TotalValue)
	}
}

func TestSnapshotRepository_Upsert_UpdatesExistingDate(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "sn2")
	repo := models.NewSnapshotRepository(testDB)

	original := &models.PortfolioSnapshot{
		UserID:       user.ID,
		TotalValue:   5000.0,
		SpotValue:    4000.0,
		FuturesPnl:   1000.0,
		SnapshotDate: "2026-06-10",
	}
	_ = repo.Upsert(original)

	// Same date, updated values.
	updated := &models.PortfolioSnapshot{
		UserID:       user.ID,
		TotalValue:   6500.0,
		SpotValue:    5000.0,
		FuturesPnl:   1500.0,
		SnapshotDate: "2026-06-10",
	}
	if err := repo.Upsert(updated); err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}

	list, _ := repo.ListByUser(user.ID, 30)
	if len(list) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(list))
	}
	if list[0].TotalValue != 6500.0 {
		t.Errorf("TotalValue after update: got %v, want 6500", list[0].TotalValue)
	}
	if list[0].SpotValue != 5000.0 {
		t.Errorf("SpotValue after update: got %v, want 5000", list[0].SpotValue)
	}
}

func TestSnapshotRepository_ListByUser_DaysFilter(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "sn3")
	repo := models.NewSnapshotRepository(testDB)

	// Insert one recent and one old snapshot.
	_ = repo.Upsert(&models.PortfolioSnapshot{
		UserID: user.ID, TotalValue: 1000,
		SnapshotDate: "2020-01-01", // very old
	})
	_ = repo.Upsert(&models.PortfolioSnapshot{
		UserID: user.ID, TotalValue: 2000,
		SnapshotDate: "2026-06-20", // recent
	})

	recent, err := repo.ListByUser(user.ID, 7)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	// Only the recent snapshot should be within last 7 days.
	for _, s := range recent {
		if strings.HasPrefix(s.SnapshotDate, "2020-01-01") {
			t.Error("old snapshot should not appear in 7-day window")
		}
	}
}

func TestSnapshotRepository_ListByUser_EmptyForOtherUser(t *testing.T) {
	truncateAll(t)
	user1 := createTestUser(t, "sn4a")
	user2 := createTestUser(t, "sn4b")
	repo := models.NewSnapshotRepository(testDB)

	_ = repo.Upsert(&models.PortfolioSnapshot{
		UserID: user1.ID, TotalValue: 999,
		SnapshotDate: "2026-06-20",
	})

	list, err := repo.ListByUser(user2.ID, 30)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 snapshots for other user, got %d", len(list))
	}
}
