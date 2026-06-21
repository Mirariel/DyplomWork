package services

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// AnalyticsService — генерує portfolio snapshots, статистику торгівлі та arbitrage сканер.
//
// TakeAllSnapshots — викликати щогодини через scheduler.
// Snapshot розраховується з live-даних БД (user_portfolios + open_positions).
type AnalyticsService struct {
	db        *sqlx.DB
	snapshots *models.SnapshotRepository
	exchanges map[string]exchange.Exchange
	logger    *slog.Logger
}

func NewAnalyticsService(db *sqlx.DB, snapshots *models.SnapshotRepository, logger *slog.Logger) *AnalyticsService {
	return &AnalyticsService{
		db:        db,
		snapshots: snapshots,
		exchanges: exchange.Registry(),
		logger:    logger,
	}
}

// --- Snapshots ---

// TakeAllSnapshots записує snapshot для кожного активного користувача.
// Використовується scheduler'ом кожну годину.
func (s *AnalyticsService) TakeAllSnapshots(ctx context.Context) {
	userIDs, err := s.snapshots.AllUserIDs()
	if err != nil || len(userIDs) == 0 {
		return
	}
	today := time.Now().Format("2006-01-02")
	for _, uid := range userIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := s.takeSnapshot(uid, today); err != nil {
			s.logger.Error("analytics: snapshot failed", "user_id", uid, "error", err)
		}
	}
}

// TakeSnapshotForUser записує snapshot для конкретного користувача прямо зараз.
// Викликається вручну через ендпоінт (для тесту без чекання scheduler).
func (s *AnalyticsService) TakeSnapshotForUser(userID int64) error {
	today := time.Now().Format("2006-01-02")
	return s.takeSnapshot(userID, today)
}

func (s *AnalyticsService) takeSnapshot(userID int64, date string) error {
	// Сума спот-активів: кількість * поточна ціна
	var spotValue float64
	if err := s.db.Get(&spotValue, `
		SELECT COALESCE(SUM(up.quantity * a.current_price), 0)
		FROM user_portfolios up
		JOIN assets a ON a.id = up.asset_id
		WHERE up.user_id = ? AND a.current_price > 0`, userID,
	); err != nil {
		return err
	}

	// Нереалізований PnL відкритих позицій
	var futuresPnl float64
	if err := s.db.Get(&futuresPnl, `
		SELECT COALESCE(SUM(pnl), 0) FROM open_positions WHERE user_id = ?`, userID,
	); err != nil {
		return err
	}

	snap := &models.PortfolioSnapshot{
		UserID:       userID,
		SpotValue:    spotValue,
		FuturesPnl:   futuresPnl,
		TotalValue:   spotValue + futuresPnl,
		SnapshotDate: date,
	}
	return s.snapshots.Upsert(snap)
}

// --- Trading Analytics ---

// TradeSummary — загальна статистика закритих угод.
type TradeSummary struct {
	TotalTrades      int     `json:"total_trades"`
	WinningTrades    int     `json:"winning_trades"`
	LosingTrades     int     `json:"losing_trades"`
	Winrate          float64 `json:"winrate"`           // відсоток
	TotalRealizedPnl float64 `json:"total_realized_pnl"`
	AvgPnl           float64 `json:"avg_pnl"`
	BestTrade        float64 `json:"best_trade"`
	WorstTrade       float64 `json:"worst_trade"`
	AvgWin           float64 `json:"avg_win"`
	AvgLoss          float64 `json:"avg_loss"`
	ProfitFactor     float64 `json:"profit_factor"` // gross_profit / abs(gross_loss)
}

// GetTradeSummary повертає статистику по всіх закритих угодах.
func (s *AnalyticsService) GetTradeSummary(userID int64) (TradeSummary, error) {
	type row struct {
		TotalTrades      int     `db:"total_trades"`
		WinningTrades    int     `db:"winning_trades"`
		TotalRealizedPnl float64 `db:"total_realized_pnl"`
		AvgPnl           float64 `db:"avg_pnl"`
		BestTrade        float64 `db:"best_trade"`
		WorstTrade       float64 `db:"worst_trade"`
		AvgWin           float64 `db:"avg_win"`
		AvgLoss          float64 `db:"avg_loss"`
		GrossProfit      float64 `db:"gross_profit"`
		GrossLoss        float64 `db:"gross_loss"`
	}
	var r row
	err := s.db.Get(&r, `
		SELECT
		    COUNT(*)                                          AS total_trades,
		    SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END) AS winning_trades,
		    COALESCE(SUM(realized_pnl), 0)                   AS total_realized_pnl,
		    COALESCE(AVG(realized_pnl), 0)                   AS avg_pnl,
		    COALESCE(MAX(realized_pnl), 0)                   AS best_trade,
		    COALESCE(MIN(realized_pnl), 0)                   AS worst_trade,
		    COALESCE(AVG(CASE WHEN realized_pnl > 0 THEN realized_pnl END), 0) AS avg_win,
		    COALESCE(AVG(CASE WHEN realized_pnl < 0 THEN realized_pnl END), 0) AS avg_loss,
		    COALESCE(SUM(CASE WHEN realized_pnl > 0 THEN realized_pnl ELSE 0 END), 0) AS gross_profit,
		    COALESCE(SUM(CASE WHEN realized_pnl < 0 THEN realized_pnl ELSE 0 END), 0) AS gross_loss
		FROM position_history
		WHERE user_id = ?`, userID,
	)
	if err != nil {
		return TradeSummary{}, err
	}

	summary := TradeSummary{
		TotalTrades:      r.TotalTrades,
		WinningTrades:    r.WinningTrades,
		LosingTrades:     r.TotalTrades - r.WinningTrades,
		TotalRealizedPnl: r.TotalRealizedPnl,
		AvgPnl:           r.AvgPnl,
		BestTrade:        r.BestTrade,
		WorstTrade:       r.WorstTrade,
		AvgWin:           r.AvgWin,
		AvgLoss:          r.AvgLoss,
	}
	if r.TotalTrades > 0 {
		summary.Winrate = float64(r.WinningTrades) / float64(r.TotalTrades) * 100
	}
	if r.GrossLoss < 0 {
		summary.ProfitFactor = r.GrossProfit / (-r.GrossLoss)
	}
	return summary, nil
}

// CoinPerformance — статистика по одній монеті.
type CoinPerformance struct {
	Symbol        string  `db:"symbol"         json:"symbol"`
	Exchange      string  `db:"exchange"       json:"exchange"`
	TotalTrades   int     `db:"total_trades"   json:"total_trades"`
	WinningTrades int     `db:"winning_trades" json:"winning_trades"`
	Winrate       float64 `json:"winrate"`
	TotalPnl      float64 `db:"total_pnl"      json:"total_pnl"`
	AvgPnl        float64 `db:"avg_pnl"        json:"avg_pnl"`
	BestTrade     float64 `db:"best_trade"     json:"best_trade"`
	WorstTrade    float64 `db:"worst_trade"    json:"worst_trade"`
}

// GetCoinPerformance повертає статистику по кожній монеті/біржі.
func (s *AnalyticsService) GetCoinPerformance(userID int64) ([]CoinPerformance, error) {
	var rows []CoinPerformance
	err := s.db.Select(&rows, `
		SELECT
		    symbol,
		    exchange,
		    COUNT(*)                                            AS total_trades,
		    SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END)  AS winning_trades,
		    COALESCE(SUM(realized_pnl), 0)                      AS total_pnl,
		    COALESCE(AVG(realized_pnl), 0)                      AS avg_pnl,
		    COALESCE(MAX(realized_pnl), 0)                      AS best_trade,
		    COALESCE(MIN(realized_pnl), 0)                      AS worst_trade
		FROM position_history
		WHERE user_id = ?
		GROUP BY symbol, exchange
		ORDER BY total_pnl DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].TotalTrades > 0 {
			rows[i].Winrate = float64(rows[i].WinningTrades) / float64(rows[i].TotalTrades) * 100
		}
	}
	return rows, nil
}

// --- Arbitrage Scanner ---

// ArbitrageOpportunity — різниця ціни одного символу між двома біржами.
type ArbitrageOpportunity struct {
	Symbol       string  `json:"symbol"`
	BuyExchange  string  `json:"buy_exchange"`
	BuyPrice     float64 `json:"buy_price"`
	SellExchange string  `json:"sell_exchange"`
	SellPrice    float64 `json:"sell_price"`
	SpreadUSD    float64 `json:"spread_usd"`
	SpreadPct    float64 `json:"spread_pct"`
}

// GetArbitrage сканує всі біржі і повертає можливості де різниця цін >= minSpreadPct%.
// Ціни беруться з кешу (не робить нових HTTP запитів якщо кеш свіжий).
func (s *AnalyticsService) GetArbitrage(minSpreadPct float64) []ArbitrageOpportunity {
	// Збираємо ціни з усіх бірж паралельно
	type exPrice struct {
		name   string
		prices map[string]float64
	}
	ch := make(chan exPrice, len(s.exchanges))

	for name, ex := range s.exchanges {
		go func(name string, ex exchange.Exchange) {
			prices, err := ex.GetPrices(nil)
			if err != nil {
				ch <- exPrice{name: name, prices: map[string]float64{}}
				return
			}
			ch <- exPrice{name: name, prices: prices}
		}(name, ex)
	}

	byExchange := make(map[string]map[string]float64, len(s.exchanges))
	for range s.exchanges {
		r := <-ch
		byExchange[r.name] = r.prices
	}

	// Нормалізуємо всі символи до базового тікера → {symbol: {exchange: price}}
	normalized := make(map[string]map[string]float64)
	for exName, prices := range byExchange {
		for rawSym, price := range prices {
			if price <= 0 {
				continue
			}
			// Відкидаємо пари не в USDT
			if !strings.HasSuffix(rawSym, "USDT") {
				continue
			}
			base := strings.TrimSuffix(rawSym, "USDT")
			if base == "" {
				continue
			}
			if normalized[base] == nil {
				normalized[base] = make(map[string]float64)
			}
			normalized[base][exName] = price
		}
	}

	// Шукаємо арбітражні можливості
	var opps []ArbitrageOpportunity
	for symbol, prices := range normalized {
		if len(prices) < 2 {
			continue
		}
		// Знаходимо мінімальну і максимальну ціну
		minPrice, maxPrice := math.MaxFloat64, 0.0
		minEx, maxEx := "", ""
		for exName, price := range prices {
			if price < minPrice {
				minPrice, minEx = price, exName
			}
			if price > maxPrice {
				maxPrice, maxEx = price, exName
			}
		}
		if minEx == maxEx || minPrice <= 0 {
			continue
		}
		spreadPct := (maxPrice - minPrice) / minPrice * 100
		if spreadPct < minSpreadPct {
			continue
		}
		opps = append(opps, ArbitrageOpportunity{
			Symbol:       symbol,
			BuyExchange:  minEx,
			BuyPrice:     minPrice,
			SellExchange: maxEx,
			SellPrice:    maxPrice,
			SpreadUSD:    maxPrice - minPrice,
			SpreadPct:    spreadPct,
		})
	}

	// Сортуємо за спредом (найбільший першим)
	sort.Slice(opps, func(i, j int) bool {
		return opps[i].SpreadPct > opps[j].SpreadPct
	})
	return opps
}
