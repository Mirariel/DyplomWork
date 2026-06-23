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
// Snapshot розраховується з live-цін кешу (PriceService), fallback → DB current_price.
type AnalyticsService struct {
	db        *sqlx.DB
	snapshots *models.SnapshotRepository
	prices    *PriceService // nil → fallback to DB current_price
	exchanges map[string]exchange.Exchange
	logger    *slog.Logger
}

func NewAnalyticsService(db *sqlx.DB, snapshots *models.SnapshotRepository, prices *PriceService, logger *slog.Logger) *AnalyticsService {
	return &AnalyticsService{
		db:        db,
		snapshots: snapshots,
		prices:    prices,
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
	at := time.Now().UTC()
	for _, uid := range userIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := s.takeSnapshot(uid, at); err != nil {
			s.logger.Error("analytics: snapshot failed", "user_id", uid, "error", err)
		}
	}
}

// TakeSnapshotForUser записує snapshot для конкретного користувача прямо зараз (точний timestamp).
func (s *AnalyticsService) TakeSnapshotForUser(userID int64) error {
	at := time.Now().UTC()
	return s.takeSnapshot(userID, at)
}

func (s *AnalyticsService) takeSnapshot(userID int64, at time.Time) error {
	spotByExchange, err := s.calcSpotValueByExchange(userID)
	if err != nil {
		return err
	}

	if len(spotByExchange) == 0 {
		// Немає активів — зберігаємо нульовий snapshot щоб уникнути прогалин
		return s.snapshots.Upsert(&models.PortfolioSnapshot{
			UserID: userID, Exchange: "all", SnapshotAt: at,
		})
	}

	for exName, spotValue := range spotByExchange {
		var futuresPnl float64
		if err := s.db.Get(&futuresPnl, `
			SELECT COALESCE(SUM(pnl), 0) FROM open_positions
			WHERE user_id = ? AND exchange = ?`, userID, exName,
		); err != nil {
			return err
		}
		snap := &models.PortfolioSnapshot{
			UserID:     userID,
			Exchange:   exName,
			SpotValue:  spotValue,
			FuturesPnl: futuresPnl,
			TotalValue: spotValue, // тільки спот, futures PnL окремо
			SnapshotAt: at,
		}
		if err := s.snapshots.Upsert(snap); err != nil {
			return err
		}
	}
	return nil
}

// calcSpotValueByExchange повертає карту exchange → spot value для всіх активів користувача.
// Пріоритет: live ціни з Redis → DB current_price.
func (s *AnalyticsService) calcSpotValueByExchange(userID int64) (map[string]float64, error) {
	type assetRow struct {
		Exchange     string  `db:"exchange"`
		Symbol       string  `db:"symbol"`
		Quantity     float64 `db:"quantity"`
		CurrentPrice float64 `db:"current_price"`
	}
	var assets []assetRow
	if err := s.db.Select(&assets, `
		SELECT up.exchange, a.symbol, up.quantity, a.current_price
		FROM user_portfolios up
		JOIN assets a ON a.id = up.asset_id
		WHERE up.user_id = ?`, userID,
	); err != nil {
		return nil, err
	}

	var livePrices map[string]float64
	if s.prices != nil {
		livePrices = s.prices.GetLivePrices(nil)
	}

	result := make(map[string]float64)
	for _, a := range assets {
		price := a.CurrentPrice
		if live, ok := livePrices[a.Symbol]; ok && live > 0 {
			price = live
		}
		result[a.Exchange] += a.Quantity * price
	}
	return result, nil
}

// --- Trading Analytics ---

// TradeSummary — загальна статистика закритих угод.
type TradeSummary struct {
	TotalTrades      int     `json:"total_trades"`
	WinningTrades    int     `json:"winning_trades"`
	LosingTrades     int     `json:"losing_trades"`
	Winrate          float64 `json:"winrate"`            // відсоток
	TotalRealizedPnl float64 `json:"total_realized_pnl"`
	AvgPnl           float64 `json:"avg_pnl"`
	BestTrade        float64 `json:"best_trade"`
	WorstTrade       float64 `json:"worst_trade"`
	AvgWin           float64 `json:"avg_win"`
	AvgLoss          float64 `json:"avg_loss"`
	ProfitFactor     float64 `json:"profit_factor"` // gross_profit / abs(gross_loss)
	TotalFees        float64 `json:"total_fees"`    // сумарна комісія за угодами
}

// GetTradeSummary повертає статистику по закритих угодах.
// days=0 — без ліміту по часу; days>0 — останні N днів.
// from/to — конкретний діапазон дат (YYYY-MM-DD); мають пріоритет над days.
// exchange="" — всі біржі.
func (s *AnalyticsService) GetTradeSummary(userID int64, days int, exchange, from, to string) (TradeSummary, error) {
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
		TotalFees        float64 `db:"total_fees"`
	}
	var r row
	filters := ""
	args := []interface{}{userID}

	// date range has priority over days
	if from != "" {
		filters += " AND closed_at >= ?"
		args = append(args, from+" 00:00:00")
	}
	if to != "" {
		filters += " AND closed_at <= ?"
		args = append(args, to+" 23:59:59")
	}
	if from == "" && to == "" && days > 0 {
		filters += " AND closed_at >= DATE_SUB(NOW(), INTERVAL ? DAY)"
		args = append(args, days)
	}
	if exchange != "" {
		filters += " AND exchange = ?"
		args = append(args, exchange)
	}
	err := s.db.Get(&r, `
		SELECT
		    COUNT(*)                                                            AS total_trades,
		    COALESCE(SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END), 0)     AS winning_trades,
		    COALESCE(SUM(realized_pnl), 0)                                     AS total_realized_pnl,
		    COALESCE(AVG(realized_pnl), 0)                                     AS avg_pnl,
		    COALESCE(MAX(realized_pnl), 0)                                     AS best_trade,
		    COALESCE(MIN(realized_pnl), 0)                                     AS worst_trade,
		    COALESCE(AVG(CASE WHEN realized_pnl > 0 THEN realized_pnl END), 0) AS avg_win,
		    COALESCE(AVG(CASE WHEN realized_pnl < 0 THEN realized_pnl END), 0) AS avg_loss,
		    COALESCE(SUM(CASE WHEN realized_pnl > 0 THEN realized_pnl ELSE 0 END), 0) AS gross_profit,
		    COALESCE(SUM(CASE WHEN realized_pnl < 0 THEN realized_pnl ELSE 0 END), 0) AS gross_loss,
		    COALESCE(SUM(fee), 0)                                               AS total_fees
		FROM position_history
		WHERE user_id = ?`+filters, args...,
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
		TotalFees:        r.TotalFees,
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
	rows := make([]CoinPerformance, 0)
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
	opps := make([]ArbitrageOpportunity, 0)
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
