package exchange

// Gate.io V4 REST API adapter.
//
// Auth: HMAC-SHA512 підпис у заголовках KEY / SIGN / Timestamp.
// Payload: timestamp + "\n" + METHOD + "\n" + path + "\n" + query + "\n" + sha512(body)
//
// Документація: https://www.gate.io/docs/developers/apiv4

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const gateBaseURL = "https://api.gateio.ws"

type Gate struct{}

func NewGate() *Gate { return &Gate{} }

func (g *Gate) Name() string { return "gate" }

// authHeaders генерує заголовки для авторизованого GET-запиту Gate.io V4.
func (g *Gate) authHeaders(path, query string, creds Credentials) map[string]string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	// bodyHash = sha512("") для GET (тіло порожнє)
	payload := ts + "\nGET\n" + path + "\n" + query + "\n" + sha512Hex("")
	return map[string]string{
		"KEY":       creds.APIKey,
		"SIGN":      hmacSHA512(creds.APISecret, payload),
		"Timestamp": ts,
	}
}

func (g *Gate) signedGet(path string, params map[string]string, creds Credentials, dest interface{}) error {
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	query := vals.Encode()
	headers := g.authHeaders(path, query, creds)
	fullURL := gateBaseURL + path
	if query != "" {
		fullURL += "?" + query
	}
	return getJSON(fullURL, headers, dest)
}

// --- Balances ---

func (g *Gate) GetBalances(creds Credentials) ([]Balance, error) {
	var resp []struct {
		Currency  string `json:"currency"`
		Available string `json:"available"`
		Locked    string `json:"locked"`
	}
	if err := g.signedGet("/api/v4/spot/accounts", nil, creds, &resp); err != nil {
		return nil, fmt.Errorf("gate balances: %w", err)
	}
	var result []Balance
	for _, a := range resp {
		total := parseFloat(a.Available) + parseFloat(a.Locked)
		if total > 1e-8 {
			result = append(result, Balance{Symbol: a.Currency, Quantity: total})
		}
	}
	return result, nil
}

// --- Open Positions ---

func (g *Gate) GetOpenPositions(creds Credentials) ([]Position, error) {
	var resp []struct {
		Contract      string `json:"contract"`
		Size          int64  `json:"size"`
		EntryPrice    string `json:"entry_price"`
		MarkPrice     string `json:"mark_price"`
		UnrealisedPnl string `json:"unrealised_pnl"`
		Leverage      string `json:"leverage"`
	}
	if err := g.signedGet("/api/v4/futures/usdt/positions", nil, creds, &resp); err != nil {
		return nil, nil // немає ф'ючерсного акаунту — не помилка
	}
	var result []Position
	for _, p := range resp {
		if p.Size == 0 {
			continue
		}
		side := "LONG"
		if p.Size < 0 {
			side = "SHORT"
		}
		result = append(result, Position{
			Symbol:     normalizeSymbol(p.Contract),
			Side:       side,
			Quantity:   math.Abs(float64(p.Size)),
			EntryPrice: parseFloat(p.EntryPrice),
			MarkPrice:  parseFloat(p.MarkPrice),
			Leverage:   int(parseFloat(p.Leverage)),
			PnL:        parseFloat(p.UnrealisedPnl),
		})
	}
	return result, nil
}

// --- Closed Trades ---

func (g *Gate) GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error) {
	var resp []struct {
		Contract   string `json:"contract"`
		Side       string `json:"side"`
		CloseSize  string `json:"close_size"`
		OpenPrice  string `json:"open_price"`
		ClosePrice string `json:"close_price"`
		Pnl        string `json:"pnl"`
		Time       int64  `json:"time"` // Unix seconds
	}
	params := map[string]string{
		"from":  strconv.FormatInt(startMs/1000, 10),
		"to":    strconv.FormatInt(endMs/1000, 10),
		"limit": "100",
	}
	if err := g.signedGet("/api/v4/futures/usdt/position_close", params, creds, &resp); err != nil {
		return nil, nil
	}
	var result []ClosedTrade
	for _, item := range resp {
		side := strings.ToUpper(item.Side)
		if side == "" {
			side = "LONG"
		}
		result = append(result, ClosedTrade{
			Symbol:     normalizeSymbol(item.Contract),
			Side:       side,
			Quantity:   math.Abs(parseFloat(item.CloseSize)),
			EntryPrice: parseFloat(item.OpenPrice),
			ClosePrice: parseFloat(item.ClosePrice),
			PnL:        parseFloat(item.Pnl),
			ClosedAt:   item.Time * 1000, // → ms
		})
	}
	return result, nil
}

// --- Prices ---

func (g *Gate) GetPrices(_ []string) (map[string]float64, error) {
	var resp []struct {
		CurrencyPair string `json:"currency_pair"`
		Last         string `json:"last"`
	}
	if err := getPublic(gateBaseURL+"/api/v4/spot/tickers", &resp); err != nil {
		return nil, fmt.Errorf("gate prices: %w", err)
	}
	prices := make(map[string]float64, len(resp))
	for _, t := range resp {
		if !strings.HasSuffix(t.CurrencyPair, "_USDT") {
			continue
		}
		price := parseFloat(t.Last)
		base := normalizeSymbol(strings.ReplaceAll(t.CurrencyPair, "_", ""))
		prices[base+"USDT"] = price
		prices[base] = price
	}
	return prices, nil
}
