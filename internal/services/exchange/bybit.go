package exchange

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	bybitBaseURL   = "https://api.bybit.com"
	bybitRecvWindow = "5000"
)

type Bybit struct{}

func NewBybit() *Bybit { return &Bybit{} }

func (by *Bybit) Name() string { return "bybit" }

// --- Auth ---

// bybitSign генерує підпис Bybit V5: HMAC-SHA256(timestamp+apiKey+recvWindow+params)
func bybitSign(ts, apiKey, apiSecret, params string) string {
	payload := ts + apiKey + bybitRecvWindow + params
	return hmacSHA256(apiSecret, payload)
}

func (by *Bybit) authHeaders(ts, apiKey, apiSecret, params string) map[string]string {
	return map[string]string{
		"X-BAPI-API-KEY":    apiKey,
		"X-BAPI-SIGN":       bybitSign(ts, apiKey, apiSecret, params),
		"X-BAPI-TIMESTAMP":  ts,
		"X-BAPI-RECV-WINDOW": bybitRecvWindow,
	}
}

func (by *Bybit) signedGet(path string, params map[string]string, creds Credentials, dest interface{}) error {
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	query := vals.Encode()
	ts := strconv.FormatInt(nowMs(), 10)
	headers := by.authHeaders(ts, creds.APIKey, creds.APISecret, query)
	fullURL := bybitBaseURL + path + "?" + query
	return getJSON(fullURL, headers, dest)
}

// --- Balances ---

func (by *Bybit) GetBalances(creds Credentials) ([]Balance, error) {
	totals := make(map[string]float64)

	for _, accType := range []string{"UNIFIED", "FUND", "SPOT"} {
		var resp struct {
			RetCode int    `json:"retCode"`
			RetMsg  string `json:"retMsg"`
			Result  struct {
				List []struct {
					Coin []struct {
						Coin          string `json:"coin"`
						WalletBalance string `json:"walletBalance"`
						Equity        string `json:"equity"`
					} `json:"coin"`
				} `json:"list"`
			} `json:"result"`
		}

		err := by.signedGet("/v5/account/wallet-balance",
			map[string]string{"accountType": accType},
			creds, &resp)
		if err != nil || resp.RetCode != 0 {
			continue
		}

		for _, account := range resp.Result.List {
			for _, coin := range account.Coin {
				v := parseFloat(coin.WalletBalance)
				if v == 0 {
					v = parseFloat(coin.Equity)
				}
				if v > 0 {
					totals[coin.Coin] += v
				}
			}
		}
	}

	var result []Balance
	for sym, amt := range totals {
		if amt > 1e-8 {
			result = append(result, Balance{Symbol: sym, Quantity: amt})
		}
	}
	return result, nil
}

// --- Open Positions ---

func (by *Bybit) GetOpenPositions(creds Credentials) ([]Position, error) {
	type category struct {
		cat        string
		settleCoin string
	}
	categories := []category{
		{"linear", "USDT"},
		{"linear", "USDC"},
		{"inverse", ""},
	}

	var result []Position

	for _, cfg := range categories {
		params := map[string]string{"category": cfg.cat}
		if cfg.settleCoin != "" {
			params["settleCoin"] = cfg.settleCoin
		}

		var resp struct {
			RetCode int    `json:"retCode"`
			RetMsg  string `json:"retMsg"`
			Result  struct {
				List []struct {
					Symbol        string `json:"symbol"`
					Side          string `json:"side"`
					Size          string `json:"size"`
					AvgPrice      string `json:"avgPrice"`
					MarkPrice     string `json:"markPrice"`
					UnrealisedPnl string `json:"unrealisedPnl"`
					Leverage      string `json:"leverage"`
					TradeMode     int    `json:"tradeMode"`
					LiqPrice      string `json:"liqPrice"`
				} `json:"list"`
			} `json:"result"`
		}

		if err := by.signedGet("/v5/position/list", params, creds, &resp); err != nil || resp.RetCode != 0 {
			continue
		}

		for _, p := range resp.Result.List {
			qty := parseFloat(p.Size)
			if qty == 0 {
				continue
			}
			side := "SHORT"
			if p.Side == "Buy" {
				side = "LONG"
			}
			marginType := "cross"
			if p.TradeMode == 1 {
				marginType = "isolated"
			}
			result = append(result, Position{
				Symbol:     normalizeSymbol(p.Symbol),
				Side:       side,
				Quantity:   math.Abs(qty),
				EntryPrice: parseFloat(p.AvgPrice),
				MarkPrice:  parseFloat(p.MarkPrice),
				Leverage:   int(parseFloat(p.Leverage)),
				PnL:        parseFloat(p.UnrealisedPnl),
				MarginType: marginType,
			})
		}
	}
	return result, nil
}

// --- Closed Trades ---
// Bybit API вимагає startTime+endTime одночасно, максимум 7 днів на запит.
// Скануємо 7-денними вікнами з пагінацією через nextPageCursor.

func (by *Bybit) GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error) {
	type category struct {
		cat        string
		settleCoin string
	}
	categories := []category{
		{"linear", "USDT"},
		{"linear", "USDC"},
		{"inverse", ""},
	}

	const windowMs = 7 * 24 * 60 * 60 * 1000 // 7 днів у мс

	var result []ClosedTrade

	for _, cfg := range categories {
		for ws := startMs; ws < endMs; ws += windowMs {
			we := ws + windowMs - 1
			if we > endMs {
				we = endMs
			}

			cursor := ""
			for page := 0; page < 10; page++ { // max 10 сторінок на вікно
				params := map[string]string{
					"category":  cfg.cat,
					"limit":     "100",
					"startTime": strconv.FormatInt(ws, 10),
					"endTime":   strconv.FormatInt(we, 10),
				}
				if cfg.settleCoin != "" {
					params["settleCoin"] = cfg.settleCoin
				}
				if cursor != "" {
					params["cursor"] = cursor
				}

				var resp struct {
					RetCode int    `json:"retCode"`
					RetMsg  string `json:"retMsg"`
					Result  struct {
						List []struct {
							Symbol        string `json:"symbol"`
							Side          string `json:"side"`
							Qty           string `json:"qty"`
							AvgEntryPrice string `json:"avgEntryPrice"`
							AvgExitPrice  string `json:"avgExitPrice"`
							ClosedPnl     string `json:"closedPnl"`
							Leverage      string `json:"leverage"`
							CreatedTime   string `json:"createdTime"`
							UpdatedTime   string `json:"updatedTime"`
						} `json:"list"`
						NextPageCursor string `json:"nextPageCursor"`
					} `json:"result"`
				}

				if err := by.signedGet("/v5/position/closed-pnl", params, creds, &resp); err != nil || resp.RetCode != 0 {
					break
				}

				for _, item := range resp.Result.List {
					side := "SHORT"
					if item.Side == "Buy" {
						side = "LONG"
					}
					closedAt, _ := strconv.ParseInt(item.UpdatedTime, 10, 64)
					lever := int(parseFloat(item.Leverage))
					result = append(result, ClosedTrade{
						Symbol:      normalizeSymbol(item.Symbol),
						Side:        side,
						Quantity:    parseFloat(item.Qty),
						EntryPrice:  parseFloat(item.AvgEntryPrice),
						ClosePrice:  parseFloat(item.AvgExitPrice),
						PnL:         parseFloat(item.ClosedPnl),
						Leverage:    lever,
						MarginMode:  "cross", // Bybit closed-pnl doesn't return margin mode
						NotionalUsd: parseFloat(item.Qty) * parseFloat(item.AvgEntryPrice),
						ClosedAt:    closedAt,
					})
				}

				cursor = resp.Result.NextPageCursor
				if cursor == "" || len(resp.Result.List) == 0 {
					break
				}

				// Невелика пауза щоб не вибити rate limit
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	return result, nil
}

// Compile-time перевірка що Bybit реалізує SpotTrader
var _ SpotTrader = (*Bybit)(nil)

// GetRecentTrades повертає спот-угоди Bybit (/v5/execution/list category=spot).
func (by *Bybit) GetRecentTrades(creds Credentials, startMs, endMs int64) ([]SpotTrade, error) {
	var result []SpotTrade

	for _, cat := range []string{"spot", "linear"} {
		cursor := ""
		for page := 0; page < 10; page++ {
			params := map[string]string{
				"category":  cat,
				"limit":     "100",
				"startTime": strconv.FormatInt(startMs, 10),
				"endTime":   strconv.FormatInt(endMs, 10),
			}
			if cursor != "" {
				params["cursor"] = cursor
			}

			var resp struct {
				RetCode int    `json:"retCode"`
				RetMsg  string `json:"retMsg"`
				Result  struct {
					List []struct {
						Symbol      string `json:"symbol"`
						Side        string `json:"side"`
						ExecQty     string `json:"execQty"`
						ExecPrice   string `json:"execPrice"`
						ExecFee     string `json:"execFee"`
						FeeCurrency string `json:"feeCurrency"`
						ExecTime    string `json:"execTime"`
					} `json:"list"`
					NextPageCursor string `json:"nextPageCursor"`
				} `json:"result"`
			}

			if err := by.signedGet("/v5/execution/list", params, creds, &resp); err != nil || resp.RetCode != 0 {
				break
			}

			for _, t := range resp.Result.List {
				fee := parseFloat(t.ExecFee)
				if fee < 0 {
					fee = -fee
				}
				ts, _ := strconv.ParseInt(t.ExecTime, 10, 64)
				side := "sell"
				if t.Side == "Buy" {
					side = "buy"
				}
				result = append(result, SpotTrade{
					Symbol:   normalizeSymbol(t.Symbol),
					Side:     side,
					Quantity: parseFloat(t.ExecQty),
					Price:    parseFloat(t.ExecPrice),
					Fee:      fee,
					FeeAsset: t.FeeCurrency,
					TradedAt: ts,
				})
			}

			cursor = resp.Result.NextPageCursor
			if cursor == "" || len(resp.Result.List) == 0 {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return result, nil
}

// --- Prices ---

func (by *Bybit) GetPrices(symbols []string) (map[string]float64, error) {
	prices := make(map[string]float64)

	for _, cat := range []string{"spot", "linear"} {
		url := fmt.Sprintf("%s/v5/market/tickers?category=%s", bybitBaseURL, cat)

		var resp struct {
			RetCode int    `json:"retCode"`
			RetMsg  string `json:"retMsg"`
			Result  struct {
				List []struct {
					Symbol    string `json:"symbol"`
					LastPrice string `json:"lastPrice"`
				} `json:"list"`
			} `json:"result"`
		}

		if err := getPublic(url, &resp); err != nil || resp.RetCode != 0 {
			continue
		}

		for _, t := range resp.Result.List {
			if !strings.HasSuffix(t.Symbol, "USDT") {
				continue
			}
			price := parseFloat(t.LastPrice)
			prices[t.Symbol] = price
			prices[strings.TrimSuffix(t.Symbol, "USDT")] = price
		}
	}
	return prices, nil
}
