package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// PortfolioSnapshot — зріз вартості спот-активів одного користувача на одній біржі.
// Кожен sync завжди додає новий рядок (INSERT, no unique constraint) — реальна часова серія.
// "All Exchanges" агрегується на рівні запиту: SUM(value) + bucket по 15 хвилин.
type PortfolioSnapshot struct {
	ID         int64     `db:"id"          json:"id"`
	UserID     int64     `db:"user_id"     json:"-"`
	Exchange   string    `db:"exchange"    json:"exchange"`
	TotalValue float64   `db:"total_value" json:"total_value"`
	SpotValue  float64   `db:"spot_value"  json:"spot_value"`
	FuturesPnl float64   `db:"futures_pnl" json:"futures_pnl"`
	SnapshotAt time.Time `db:"snapshot_at" json:"snapshot_at"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

// ChartPoint — одна точка для графіку Portfolio Value.
type ChartPoint struct {
	RecordedAt time.Time `db:"recorded_at" json:"recorded_at"`
	Value      float64   `db:"value"       json:"value"`
}

// SnapshotRepository — DB-операції для portfolio_snapshots.
type SnapshotRepository struct {
	db *sqlx.DB
}

func NewSnapshotRepository(db *sqlx.DB) *SnapshotRepository {
	return &SnapshotRepository{db: db}
}

// Insert завжди вставляє новий рядок (не upsert).
func (r *SnapshotRepository) Upsert(s *PortfolioSnapshot) error {
	_, err := r.db.Exec(`
		INSERT INTO portfolio_snapshots (user_id, exchange, total_value, spot_value, futures_pnl, snapshot_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		s.UserID, s.Exchange, s.TotalValue, s.SpotValue, s.FuturesPnl, s.SnapshotAt,
	)
	return err
}

// chartInterval повертає кількість секунд в інтервалі підтримуваних діапазонів.
func chartFrom(rangeKey string) time.Time {
	now := time.Now().UTC()
	switch rangeKey {
	case "1d":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "30d":
		return now.Add(-30 * 24 * time.Hour)
	case "90d":
		return now.Add(-90 * 24 * time.Hour)
	default: // "all"
		return now.Add(-365 * 24 * time.Hour)
	}
}

// ChartAll повертає агреговані (по всіх біржах) точки з bucket-ингом по 1 хвилині.
// GROUP BY по аліасу recorded_at — сумісно з ONLY_FULL_GROUP_BY.
func (r *SnapshotRepository) ChartAll(userID int64, rangeKey string) ([]ChartPoint, error) {
	from := chartFrom(rangeKey)
	points := make([]ChartPoint, 0)
	err := r.db.Select(&points, `
		SELECT
		    FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(snapshot_at) / 60) * 60) AS recorded_at,
		    SUM(total_value) AS value
		FROM portfolio_snapshots
		WHERE user_id = ? AND snapshot_at >= ?
		GROUP BY recorded_at
		ORDER BY recorded_at ASC`,
		userID, from,
	)
	return points, err
}

// ChartByExchange повертає точки для конкретної біржі без агрегації.
func (r *SnapshotRepository) ChartByExchange(userID int64, exchange, rangeKey string) ([]ChartPoint, error) {
	from := chartFrom(rangeKey)
	points := make([]ChartPoint, 0)
	err := r.db.Select(&points, `
		SELECT snapshot_at AS recorded_at, total_value AS value
		FROM portfolio_snapshots
		WHERE user_id = ? AND exchange = ? AND snapshot_at >= ?
		ORDER BY snapshot_at ASC`,
		userID, exchange, from,
	)
	return points, err
}

// ChartAllRange — як ChartAll, але для довільного діапазону.
func (r *SnapshotRepository) ChartAllRange(userID int64, from, to time.Time) ([]ChartPoint, error) {
	points := make([]ChartPoint, 0)
	err := r.db.Select(&points, `
		SELECT
		    FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(snapshot_at) / 60) * 60) AS recorded_at,
		    SUM(total_value) AS value
		FROM portfolio_snapshots
		WHERE user_id = ? AND snapshot_at >= ? AND snapshot_at <= ?
		GROUP BY recorded_at
		ORDER BY recorded_at ASC`,
		userID, from, to,
	)
	return points, err
}

// ChartByExchangeRange — як ChartByExchange, але для довільного діапазону.
func (r *SnapshotRepository) ChartByExchangeRange(userID int64, exchange string, from, to time.Time) ([]ChartPoint, error) {
	points := make([]ChartPoint, 0)
	err := r.db.Select(&points, `
		SELECT snapshot_at AS recorded_at, total_value AS value
		FROM portfolio_snapshots
		WHERE user_id = ? AND exchange = ? AND snapshot_at >= ? AND snapshot_at <= ?
		ORDER BY snapshot_at ASC`,
		userID, exchange, from, to,
	)
	return points, err
}

// ListByUser повертає сирі snapshots за останні N днів (для legacy / тестів).
func (r *SnapshotRepository) ListByUser(userID int64, days int) ([]PortfolioSnapshot, error) {
	snaps := make([]PortfolioSnapshot, 0)
	err := r.db.Select(&snaps, `
		SELECT * FROM portfolio_snapshots
		WHERE user_id = ? AND snapshot_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		ORDER BY snapshot_at ASC`,
		userID, days,
	)
	return snaps, err
}

// ListByUserHours повертає snapshots за останні N годин.
func (r *SnapshotRepository) ListByUserHours(userID int64, hours int) ([]PortfolioSnapshot, error) {
	snaps := make([]PortfolioSnapshot, 0)
	err := r.db.Select(&snaps, `
		SELECT * FROM portfolio_snapshots
		WHERE user_id = ? AND snapshot_at >= DATE_SUB(NOW(), INTERVAL ? HOUR)
		ORDER BY snapshot_at ASC`,
		userID, hours,
	)
	return snaps, err
}

// ListByUserRange повертає snapshots у довільному діапазоні.
func (r *SnapshotRepository) ListByUserRange(userID int64, from, to time.Time) ([]PortfolioSnapshot, error) {
	snaps := make([]PortfolioSnapshot, 0)
	err := r.db.Select(&snaps, `
		SELECT * FROM portfolio_snapshots
		WHERE user_id = ? AND snapshot_at >= ? AND snapshot_at <= ?
		ORDER BY snapshot_at ASC`,
		userID, from, to,
	)
	return snaps, err
}

// AllUserIDs повертає ID всіх унікальних користувачів з активами або позиціями.
func (r *SnapshotRepository) AllUserIDs() ([]int64, error) {
	var ids []int64
	err := r.db.Select(&ids,
		"SELECT DISTINCT user_id FROM user_portfolios UNION SELECT DISTINCT user_id FROM open_positions")
	return ids, err
}
