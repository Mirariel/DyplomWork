package models_test

import (
	"testing"

	"github.com/okochadmytro/tradetracker/internal/models"
)

func TestUserRepository_CreateAndFind(t *testing.T) {
	truncateAll(t)
	repo := models.NewUserRepository(testDB)

	created, err := repo.Create("alice", "alice@test.com", "secret")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero ID after Create")
	}

	byEmail, err := repo.FindByEmail("alice@test.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Errorf("FindByEmail ID: got %d, want %d", byEmail.ID, created.ID)
	}
	if byEmail.Username != "alice" {
		t.Errorf("Username: got %q, want %q", byEmail.Username, "alice")
	}

	byID, err := repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if byID.Email != "alice@test.com" {
		t.Errorf("FindByID email: got %q, want %q", byID.Email, "alice@test.com")
	}
}

func TestUserRepository_CheckPassword(t *testing.T) {
	truncateAll(t)
	repo := models.NewUserRepository(testDB)

	user, err := repo.Create("bob", "bob@test.com", "correct")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !repo.CheckPassword(user, "correct") {
		t.Error("CheckPassword: expected true for correct password")
	}
	if repo.CheckPassword(user, "wrong") {
		t.Error("CheckPassword: expected false for wrong password")
	}
}

func TestUserRepository_DuplicateEmail(t *testing.T) {
	truncateAll(t)
	repo := models.NewUserRepository(testDB)

	if _, err := repo.Create("carol", "carol@test.com", "pass"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := repo.Create("carol2", "carol@test.com", "pass"); err == nil {
		t.Error("expected error on duplicate email, got nil")
	}
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	truncateAll(t)
	repo := models.NewUserRepository(testDB)

	if _, err := repo.FindByID(999999); err == nil {
		t.Error("expected error for non-existent ID")
	}
}
