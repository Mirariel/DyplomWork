package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"github.com/okochadmytro/tradetracker/internal/config"
)

func Connect(cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}
