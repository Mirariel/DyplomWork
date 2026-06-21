package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// PortfolioSnapshot — щоденний зріз вартості портфеля користувача.
// Один запис на дату (UNIQUE KEY user_id + snapshot_date).
type PortfolioSnapshot struct {
	ID           int64     `db:"id"            json:"id"`
	UserID       int64     `db:"user_id"       json:"-"`
	TotalValue   float64   `db:"total_value"   json:"total_value"`
	SpotValue    float64   `db:"spot_value"    json:"spot_value"`
	FuturesPnl   float64   `db:"futures_pnl"   json:"futures_pnl"`
	SnapshotDate string    `db:"snapshot_date" json:"snapshot_date"` // "2026-06-21"
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// SnapshotRepository — DB-операції для portfolio_snapshots.
type SnapshotRepository struct {
	db *sqlx.DB
}

func NewSnapshotRepository(db *sqlx.DB) *SnapshotRepository {
	return &SnapshotRepository{db: db}
}

// Upsert зберігає snapshot за поточну дату.
// ON DUPLICATE KEY UPDATE — оновлює якщо запис за цю дату вже є.
func (r *SnapshotRepository) Upsert(s *PortfolioSnapshot) error {
	_, err := r.db.Exec(`
		INSERT INTO portfolio_snapshots (user_id, total_value, spot_value, futures_pnl, snapshot_date)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		    total_value  = VALUES(total_value),
		    spot_value   = VALUES(spot_value),
		    futures_pnl  = VALUES(futures_pnl)`,
		s.UserID, s.TotalValue, s.SpotValue, s.FuturesPnl, s.SnapshotDate,
	)
	return err
}

// ListByUser повертає snapshots користувача за останні N днів.
func (r *SnapshotRepository) ListByUser(userID int64, days int) ([]PortfolioSnapshot, error) {
	snaps := make([]PortfolioSnapshot, 0)
	err := r.db.Select(&snaps, `
		SELECT * FROM portfolio_snapshots
		WHERE user_id = ? AND snapshot_date >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		ORDER BY snapshot_date ASC`,
		userID, days,
	)
	return snaps, err
}

// AllUserIDs повертає ID всіх унікальних користувачів що мають portfolios.
func (r *SnapshotRepository) AllUserIDs() ([]int64, error) {
	var ids []int64
	err := r.db.Select(&ids,
		"SELECT DISTINCT user_id FROM user_portfolios UNION SELECT DISTINCT user_id FROM open_positions")
	return ids, err
}
