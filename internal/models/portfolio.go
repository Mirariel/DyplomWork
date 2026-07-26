package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// Asset — визначення активу (монети)
type Asset struct {
	ID              int64      `db:"id"               json:"id"`
	Symbol          string     `db:"symbol"           json:"symbol"`
	Name            string     `db:"name"             json:"name"`
	Type            string     `db:"type"             json:"type"`
	CurrentPrice    float64    `db:"current_price"    json:"current_price"`
	PriceUpdatedAt  *time.Time `db:"price_updated_at" json:"price_updated_at"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
}

// UserPortfolio — спот-баланс користувача на конкретній біржі
type UserPortfolio struct {
	ID          int64     `db:"id"            json:"id"`
	UserID      int64     `db:"user_id"       json:"user_id"`
	AssetID     int64     `db:"asset_id"      json:"asset_id"`
	Exchange    string    `db:"exchange"      json:"exchange"`
	Quantity    float64   `db:"quantity"      json:"quantity"`
	AvgBuyPrice float64   `db:"avg_buy_price" json:"avg_buy_price"`
	ManuallySet bool      `db:"manually_set"  json:"manually_set"`
	UpdatedAt   time.Time `db:"updated_at"    json:"updated_at"`

	// Joined з assets
	Symbol       string  `db:"symbol"        json:"symbol"`
	CurrentPrice float64 `db:"current_price" json:"current_price"`
}

// OpenPosition — відкрита ф'ючерсна/маржинальна позиція
type OpenPosition struct {
	ID         int64     `db:"id"          json:"id"`
	UserID     int64     `db:"user_id"     json:"user_id"`
	AssetID    int64     `db:"asset_id"    json:"asset_id"`
	Symbol     string    `db:"symbol"      json:"symbol"`
	Exchange   string    `db:"exchange"    json:"exchange"`
	Side       string    `db:"side"        json:"side"`
	MarginMode string    `db:"margin_mode" json:"margin_mode"`
	EntryPrice float64   `db:"entry_price" json:"entry_price"`
	MarkPrice  float64   `db:"mark_price"  json:"mark_price"`
	Quantity   float64   `db:"quantity"    json:"quantity"`
	PnL        float64   `db:"pnl"         json:"pnl"`
	Leverage   string    `db:"leverage"    json:"leverage"`  // "10x"
	LiqPrice   float64   `db:"liq_price"   json:"liq_price"`
	Margin     float64   `db:"margin"      json:"margin"`
	TradeSize  float64   `db:"trade_size"  json:"trade_size"`
	Comment    *string   `db:"comment"     json:"comment"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

// PositionHistory — закрита угода (архів)
type PositionHistory struct {
	ID          int64    `db:"id"           json:"id"`
	UserID      int64    `db:"user_id"      json:"user_id"`
	Symbol      string   `db:"symbol"       json:"symbol"`
	Exchange    string   `db:"exchange"     json:"exchange"`
	Side        string   `db:"side"         json:"side"`
	MarginMode  string   `db:"margin_mode"  json:"margin_mode"`
	Leverage    string   `db:"leverage"     json:"leverage"`
	EntryPrice  float64  `db:"entry_price"  json:"entry_price"`
	ExitPrice   float64  `db:"exit_price"   json:"exit_price"`
	Quantity    float64  `db:"quantity"     json:"quantity"`
	RealizedPnl float64  `db:"realized_pnl" json:"realized_pnl"`
	Fee         float64  `db:"fee"          json:"fee"`
	MaxSize     float64  `db:"max_size"     json:"max_size"`
	Margin      float64  `db:"margin"       json:"margin"`
	OpenedAt    *string  `db:"opened_at"    json:"opened_at"`
	ClosedAt    *string  `db:"closed_at"    json:"closed_at"`
	Comment     *string  `db:"comment"      json:"comment"`
	CreatedAt   string   `db:"created_at"   json:"created_at"`
}

// ExternalApiCredential — зашифровані API-ключі біржі
type ExternalApiCredential struct {
	ID                  int64      `db:"id"                    json:"id"`
	UserID              int64      `db:"user_id"               json:"user_id"`
	Exchange            string     `db:"exchange"              json:"exchange"`
	Label               string     `db:"label"                 json:"label"`
	ApiKeyHint          string     `db:"api_key_hint"          json:"api_key_hint"`
	ApiKeyEncrypted     string     `db:"api_key_encrypted"     json:"-"`
	ApiSecretEncrypted  string     `db:"api_secret_encrypted"  json:"-"`
	PassphraseEncrypted *string    `db:"passphrase_encrypted"  json:"-"`
	HasPassphrase       bool       `db:"has_passphrase"        json:"has_passphrase"`
	IsActive            bool       `db:"is_active"             json:"is_active"`
	LastSyncAt          *time.Time `db:"last_sync_at"          json:"last_sync_at"`
	LastSyncError       *string    `db:"last_sync_error"       json:"last_sync_error"`
	CreatedAt           time.Time  `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"            json:"updated_at"`
}

// --- Repositories ---

type PortfolioRepository struct {
	db *sqlx.DB
}

func NewPortfolioRepository(db *sqlx.DB) *PortfolioRepository {
	return &PortfolioRepository{db: db}
}

func (r *PortfolioRepository) GetUserAssets(userID int64) ([]UserPortfolio, error) {
	items := make([]UserPortfolio, 0)
	err := r.db.Select(&items, `
		SELECT
			up.id, up.user_id, up.asset_id, up.exchange,
			up.quantity, up.avg_buy_price, up.manually_set, up.updated_at,
			a.symbol, a.current_price
		FROM user_portfolios up
		JOIN assets a ON a.id = up.asset_id
		WHERE up.user_id = ? AND up.quantity > 0
		ORDER BY (up.quantity * a.current_price) DESC
	`, userID)
	return items, err
}

// UpdateAssetPrice оновлює середню ціну входу для активу.
// ManuallySet = true забороняє перезапис при автосинку.
func (r *PortfolioRepository) UpdateAssetPrice(id int64, userID int64, avgBuyPrice float64, manuallySet bool) error {
	_, err := r.db.Exec(
		`UPDATE user_portfolios SET avg_buy_price = ?, manually_set = ? WHERE id = ? AND user_id = ?`,
		avgBuyPrice, manuallySet, id, userID,
	)
	return err
}

func (r *PortfolioRepository) GetOpenPositions(userID int64) ([]OpenPosition, error) {
	items := make([]OpenPosition, 0)
	err := r.db.Select(&items,
		`SELECT * FROM open_positions WHERE user_id = ? ORDER BY updated_at DESC`,
		userID)
	return items, err
}

func (r *PortfolioRepository) GetHistory(userID int64, limit, offset int) ([]PositionHistory, error) {
	items := make([]PositionHistory, 0)
	err := r.db.Select(&items,
		`SELECT * FROM position_history WHERE user_id = ? ORDER BY closed_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset)
	return items, err
}

func (r *PortfolioRepository) GetHistoryCount(userID int64) (int, error) {
	var count int
	err := r.db.Get(&count, `SELECT COUNT(*) FROM position_history WHERE user_id = ?`, userID)
	return count, err
}

// GetHistoryFiltered — з опціональним фільтром дат (from/to у форматі YYYY-MM-DD або порожній).
func (r *PortfolioRepository) GetHistoryFiltered(userID int64, limit, offset int, from, to string) ([]PositionHistory, error) {
	query := `SELECT * FROM position_history WHERE user_id = ?`
	args := []interface{}{userID}
	if from != "" {
		query += ` AND closed_at >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND closed_at < DATE_ADD(?, INTERVAL 1 DAY)`
		args = append(args, to)
	}
	query += ` ORDER BY closed_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	items := make([]PositionHistory, 0)
	err := r.db.Select(&items, query, args...)
	return items, err
}

func (r *PortfolioRepository) GetHistoryFilteredCount(userID int64, from, to string) (int, error) {
	query := `SELECT COUNT(*) FROM position_history WHERE user_id = ?`
	args := []interface{}{userID}
	if from != "" {
		query += ` AND closed_at >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND closed_at < DATE_ADD(?, INTERVAL 1 DAY)`
		args = append(args, to)
	}
	var count int
	err := r.db.Get(&count, query, args...)
	return count, err
}

func (r *PortfolioRepository) GetCredentials(userID int64) ([]ExternalApiCredential, error) {
	items := make([]ExternalApiCredential, 0)
	err := r.db.Select(&items,
		`SELECT * FROM external_api_credentials WHERE user_id = ? AND is_active = 1`,
		userID)
	return items, err
}

// GetCredentialByID повертає конкретний API-ключ за ID що належить user.
func (r *PortfolioRepository) GetCredentialByID(id, userID int64) (*ExternalApiCredential, error) {
	var c ExternalApiCredential
	err := r.db.Get(&c,
		`SELECT * FROM external_api_credentials WHERE id = ? AND user_id = ? AND is_active = 1`,
		id, userID)
	return &c, err
}

// GetCredentialsByIDs повертає кілька API-ключів за переліком ID.
func (r *PortfolioRepository) GetCredentialsByIDs(ids []int64, userID int64) ([]ExternalApiCredential, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(
		`SELECT * FROM external_api_credentials WHERE id IN (?) AND user_id = ? AND is_active = 1`,
		ids, userID)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)
	items := make([]ExternalApiCredential, 0, len(ids))
	err = r.db.Select(&items, query, args...)
	return items, err
}

func (r *PortfolioRepository) UpsertCredential(cred *ExternalApiCredential) error {
	res, err := r.db.NamedExec(`
		INSERT INTO external_api_credentials
			(user_id, exchange, label, api_key_hint, api_key_encrypted, api_secret_encrypted, passphrase_encrypted, is_active)
		VALUES
			(:user_id, :exchange, :label, :api_key_hint, :api_key_encrypted, :api_secret_encrypted, :passphrase_encrypted, :is_active)
	`, cred)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if id > 0 {
		cred.ID = id
	}
	return nil
}

func (r *PortfolioRepository) DeleteCredential(id, userID int64) error {
	_, err := r.db.Exec(
		"DELETE FROM external_api_credentials WHERE id = ? AND user_id = ?",
		id, userID,
	)
	return err
}

func (r *PortfolioRepository) UpdatePositionComment(id, userID int64, comment *string) error {
	_, err := r.db.Exec(
		"UPDATE open_positions SET comment = ? WHERE id = ? AND user_id = ?",
		comment, id, userID,
	)
	return err
}

func (r *PortfolioRepository) UpdateHistoryComment(id, userID int64, comment *string) error {
	_, err := r.db.Exec(
		"UPDATE position_history SET comment = ? WHERE id = ? AND user_id = ?",
		comment, id, userID,
	)
	return err
}
