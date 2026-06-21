package models

import (
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64     `db:"id"         json:"id"`
	Username  string    `db:"username"   json:"username"`
	Email     string    `db:"email"      json:"email"`
	Password  string    `db:"password"   json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"-"`
}

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(email string) (*User, error) {
	var u User
	err := r.db.Get(&u, "SELECT * FROM users WHERE email = ? LIMIT 1", email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByEmailOrUsername — один SQL запит для логіну по email або username
func (r *UserRepository) FindByEmailOrUsername(login string) (*User, error) {
	var u User
	err := r.db.Get(&u, "SELECT * FROM users WHERE email = ? OR username = ? LIMIT 1", login, login)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindByID(id int64) (*User, error) {
	var u User
	err := r.db.Get(&u, "SELECT * FROM users WHERE id = ? LIMIT 1", id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) Create(username, email, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	_, err = r.db.Exec(
		"INSERT INTO users (username, email, password) VALUES (?, ?, ?)",
		username, email, string(hash),
	)
	if err != nil {
		return nil, err
	}

	return r.FindByEmail(email)
}

func (r *UserRepository) CheckPassword(user *User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) == nil
}

func (r *UserRepository) UpdateProfile(id int64, username, email string) (*User, error) {
	_, err := r.db.Exec(
		"UPDATE users SET username = ?, email = ?, updated_at = NOW() WHERE id = ?",
		username, email, id,
	)
	if err != nil {
		return nil, err
	}
	return r.FindByID(id)
}

func (r *UserRepository) UpdatePassword(id int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		"UPDATE users SET password = ?, updated_at = NOW() WHERE id = ?",
		string(hash), id,
	)
	return err
}
