package exchange

import (
	"fmt"
	"math"
	"strings"
)

const (
	binanceSpotURL    = "https://api.binance.com"
	binanceFuturesURL = "https://fapi.binance.com"
)

type Binance struct{}

func NewBinance() *Binance { return &Binance{} }

func (b *Binance) Name() string { return "binance" }

// --- Balances ---

func (b *Binance) GetBalances(creds Credentials) ([]Balance, error) {
	ts := nowMs()
	query := fmt.Sprintf("timestamp=%d", ts)
	sig := hmacSHA256(creds.APISecret, query)
	url := fmt.Sprintf("%s/api/v3/account?%s&signature=%s", binanceSpotURL, query, sig)

	var resp struct {
		Balances []struct {
			Asset  string `json:"asset"`
			Free   string `json:"free"`
			Locked string `json:"locked"`
		} `json:"balances"`
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := getJSON(url, map[string]string{"X-MBX-APIKEY": creds.APIKey}, &resp); err != nil {
		return nil, fmt.Errorf("binance balances: %w", err)
	}
	if resp.Code != 0 && resp.Msg != "" {
		return nil, fmt.Errorf("binance balances API: %s", resp.Msg)
	}

	var result []Balance
	for _, a := range resp.Balances {
		total := parseFloat(a.Free) + parseFloat(a.Locked)
		if total > 1e-8 {
			result = append(result, Balance{Symbol: a.Asset, Quantity: total})
		}
	}
	return result, nil
}

// --- Open Positions ---

func (b *Binance) GetOpenPositions(creds Credentials) ([]Position, error) {
	ts := nowMs()
	query := fmt.Sprintf("timestamp=%d", ts)
	sig := hmacSHA256(creds.APISecret, query)
	url := fmt.Sprintf("%s/fapi/v2/positionRisk?%s&signature=%s", binanceFuturesURL, query, sig)

	var resp []struct {
		Symbol           string `json:"symbol"`
		PositionAmt      string `json:"positionAmt"`
		EntryPrice       string `json:"entryPrice"`
		MarkPrice        string `json:"markPrice"`
		UnRealizedProfit string `json:"unRealizedProfit"`
		Leverage         string `json:"leverage"`
		MarginType       string `json:"marginType"`
		LiquidationPrice string `json:"liquidationPrice"`
		IsolatedMargin   string `json:"isolatedMargin"`
		Notional         string `json:"notional"`
		Code             int    `json:"code"`
		Msg              string `json:"msg"`
	}

	if err := getJSON(url, map[string]string{"X-MBX-APIKEY": creds.APIKey}, &resp); err != nil {
		// Binance повертає порожній масив якщо нема ф'ючерсного акаунту — не помилка
		return nil, nil
	}

	var result []Position
	for _, p := range resp {
		qty := parseFloat(p.PositionAmt)
		if qty == 0 {
			continue
		}
		side := "LONG"
		if qty < 0 {
			side = "SHORT"
		}
		marginType := strings.ToLower(p.MarginType)
		if marginType == "" {
			marginType = "cross"
		}
		margin := parseFloat(p.IsolatedMargin)
		if margin == 0 {
			notional := math.Abs(parseFloat(p.Notional))
			lev := parseFloat(p.Leverage)
			if lev > 0 {
				margin = notional / lev
			}
		}
		entry := parseFloat(p.EntryPrice)
		absQty := math.Abs(qty)
		result = append(result, Position{
			Symbol:           normalizeSymbol(p.Symbol),
			Side:             side,
			Quantity:         absQty,
			EntryPrice:       entry,
			MarkPrice:        parseFloat(p.MarkPrice),
			Leverage:         int(parseFloat(p.Leverage)),
			PnL:              parseFloat(p.UnRealizedProfit),
			MarginType:       marginType,
			Margin:           margin,
			NotionalEntryUsd: absQty * entry,
		})
	}
	return result, nil
}

// --- Closed Trades (через income REALIZED_PNL) ---

func (b *Binance) GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error) {
	ts := nowMs()
	query := fmt.Sprintf("incomeType=REALIZED_PNL&limit=1000&startTime=%d&endTime=%d&timestamp=%d",
		startMs, endMs, ts)
	sig := hmacSHA256(creds.APISecret, query)
	url := fmt.Sprintf("%s/fapi/v1/income?%s&signature=%s", binanceFuturesURL, query, sig)

	var resp []struct {
		Symbol string `json:"symbol"`
		Income string `json:"income"`
		Time   int64  `json:"time"`
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
	}

	if err := getJSON(url, map[string]string{"X-MBX-APIKEY": creds.APIKey}, &resp); err != nil {
		return nil, nil // немає ф'ючерсного акаунту — не помилка
	}

	var result []ClosedTrade
	for _, item := range resp {
		pnl := parseFloat(item.Income)
		side := "LONG"
		if pnl < 0 {
			side = "SHORT"
		}
		result = append(result, ClosedTrade{
			Symbol:     normalizeSymbol(item.Symbol),
			Side:       side,
			PnL:        pnl,
			MarginMode: "cross", // Binance income API doesn't return margin mode; default to cross
			ClosedAt:   item.Time,
		})
	}
	return result, nil
}

// --- Prices ---

func (b *Binance) GetPrices(symbols []string) (map[string]float64, error) {
	url := binanceSpotURL + "/api/v3/ticker/price"

	var resp []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	if err := getPublic(url, &resp); err != nil {
		return nil, fmt.Errorf("binance prices: %w", err)
	}

	prices := make(map[string]float64, len(resp))
	for _, t := range resp {
		if strings.HasSuffix(t.Symbol, "USDT") {
			price := parseFloat(t.Price)
			prices[t.Symbol] = price
			prices[strings.TrimSuffix(t.Symbol, "USDT")] = price
		}
	}
	return prices, nil
}

// Compile-time перевірка що Binance реалізує SpotTrader
var _ SpotTrader = (*Binance)(nil)

// GetRecentTrades повертає спот-угоди Binance за діапазон часу.
// Binance потребує symbol — тому запитуємо по ТОП монетах.
func (b *Binance) GetRecentTrades(creds Credentials, startMs, endMs int64) ([]SpotTrade, error) {
	// Binance /api/v3/myTrades потребує конкретний symbol.
	// Отримуємо всі відомі символи через account balances і запитуємо для кожного.
	balances, err := b.GetBalances(creds)
	if err != nil {
		return nil, err
	}

	var result []SpotTrade
	for _, bal := range balances {
		sym := strings.ToUpper(bal.Symbol) + "USDT"
		ts := nowMs()
		query := fmt.Sprintf("symbol=%s&startTime=%d&endTime=%d&limit=200&timestamp=%d",
			sym, startMs, endMs, ts)
		sig := hmacSHA256(creds.APISecret, query)
		url := fmt.Sprintf("%s/api/v3/myTrades?%s&signature=%s", binanceSpotURL, query, sig)

		var resp []struct {
			Symbol          string `json:"symbol"`
			Id              int64  `json:"id"`
			IsBuyer         bool   `json:"isBuyer"`
			Qty             string `json:"qty"`
			Price           string `json:"price"`
			Commission      string `json:"commission"`
			CommissionAsset string `json:"commissionAsset"`
			Time            int64  `json:"time"`
			Code            int    `json:"code"`
			Msg             string `json:"msg"`
		}
		if err := getJSON(url, map[string]string{"X-MBX-APIKEY": creds.APIKey}, &resp); err != nil {
			continue
		}
		for _, t := range resp {
			if t.Code != 0 {
				continue
			}
			side := "sell"
			if t.IsBuyer {
				side = "buy"
			}
			result = append(result, SpotTrade{
				Symbol:   normalizeSymbol(t.Symbol),
				Side:     side,
				Quantity: parseFloat(t.Qty),
				Price:    parseFloat(t.Price),
				Fee:      parseFloat(t.Commission),
				FeeAsset: t.CommissionAsset,
				TradedAt: t.Time,
			})
		}
	}
	return result, nil
}
