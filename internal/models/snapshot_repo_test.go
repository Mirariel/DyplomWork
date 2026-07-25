package models_test

import (
	"testing"
	"time"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func TestSnapshotRepository_UpsertAndList(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "sn1")
	repo := models.NewSnapshotRepository(testDB)

	base := time.Now().UTC().Truncate(time.Second)

	s1 := &models.PortfolioSnapshot{
		UserID:     user.ID,
		TotalValue: 10000.0,
		SpotValue:  8000.0,
		FuturesPnl: 2000.0,
		SnapshotAt: base.Add(-2 * time.Hour),
	}
	if err := repo.Upsert(s1); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	s2 := &models.PortfolioSnapshot{
		UserID:     user.ID,
		TotalValue: 11000.0,
		SpotValue:  9000.0,
		FuturesPnl: 2000.0,
		SnapshotAt: base.Add(-1 * time.Hour),
	}
	_ = repo.Upsert(s2)

	list, err := repo.ListByUser(user.ID, 30)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByUser: got %d, want 2", len(list))
	}
	// Ordered by snapshot_at ASC — першим має йти старіший snapshot (s1).
	if list[0].TotalValue != 10000.0 {
		t.Errorf("list[0].TotalValue: got %v, want 10000", list[0].TotalValue)
	}
	if !list[0].SnapshotAt.Before(list[1].SnapshotAt) {
		t.Errorf("expected list ordered by snapshot_at ASC: %v >= %v",
			list[0].SnapshotAt, list[1].SnapshotAt)
	}
}

func TestSnapshotRepository_Upsert_AllowsSameTimestamp(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "sn2")
	repo := models.NewSnapshotRepository(testDB)

	at := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)

	first := &models.PortfolioSnapshot{
		UserID:     user.ID,
		TotalValue: 5000.0,
		SpotValue:  4000.0,
		FuturesPnl: 1000.0,
		SnapshotAt: at,
	}
	_ = repo.Upsert(first)

	// Той самий момент часу — після міграції 011 unique-ключ знято,
	// тому Upsert виконує звичайний INSERT і другий рядок додається.
	second := &models.PortfolioSnapshot{
		UserID:     user.ID,
		TotalValue: 6500.0,
		SpotValue:  5000.0,
		FuturesPnl: 1500.0,
		SnapshotAt: at,
	}
	if err := repo.Upsert(second); err != nil {
		t.Fatalf("Upsert (second insert): %v", err)
	}

	list, _ := repo.ListByUser(user.ID, 30)
	if len(list) != 2 {
		t.Fatalf("expected 2 snapshots (plain insert), got %d", len(list))
	}
}

func TestSnapshotRepository_ListByUser_DaysFilter(t *testing.T) {
	truncateAll(t)
	user := createTestUser(t, "sn3")
	repo := models.NewSnapshotRepository(testDB)

	now := time.Now().UTC().Truncate(time.Second)

	// Один давній і один свіжий snapshot.
	_ = repo.Upsert(&models.PortfolioSnapshot{
		UserID: user.ID, TotalValue: 1000,
		SnapshotAt: now.AddDate(0, 0, -60), // 60 днів тому
	})
	_ = repo.Upsert(&models.PortfolioSnapshot{
		UserID: user.ID, TotalValue: 2000,
		SnapshotAt: now.Add(-time.Hour), // година тому
	})

	recent, err := repo.ListByUser(user.ID, 7)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 snapshot within 7-day window, got %d", len(recent))
	}
	if recent[0].TotalValue != 2000 {
		t.Errorf("old snapshot should not appear in 7-day window (got TotalValue=%v)",
			recent[0].TotalValue)
	}
}

func TestSnapshotRepository_ListByUser_EmptyForOtherUser(t *testing.T) {
	truncateAll(t)
	user1 := createTestUser(t, "sn4a")
	user2 := createTestUser(t, "sn4b")
	repo := models.NewSnapshotRepository(testDB)

	_ = repo.Upsert(&models.PortfolioSnapshot{
		UserID: user1.ID, TotalValue: 999,
		SnapshotAt: time.Now().UTC().Add(-time.Hour),
	})

	list, err := repo.ListByUser(user2.ID, 30)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 snapshots for other user, got %d", len(list))
	}
}
