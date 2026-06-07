package exchange

// KuCoin V1/V2 REST API adapter.
//
// Auth: HMAC-SHA256 з base64-виводом (KC-API-KEY v2).
// Sign: Base64(HMAC-SHA256(timestamp + method + path + body, secret))
// Passphrase: Base64(HMAC-SHA256(passphrase, secret))  ← обов'язково для v2 ключів
//
// Документація: https://docs.kucoin.com/

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const kucoinBaseURL = "https://api.kucoin.com"

type KuCoin struct{}

func NewKuCoin() *KuCoin { return &KuCoin{} }

func (ku *KuCoin) Name() string { return "kucoin" }

// authHeaders генерує заголовки для авторизованого GET-запиту KuCoin V2.
func (ku *KuCoin) authHeaders(method, path, body string, creds Credentials) map[string]string {
	ts := strconv.FormatInt(nowMs(), 10)
	// strToSign = timestamp + method + path + body
	strToSign := ts + strings.ToUpper(method) + path + body
	sign := hmacSHA256Base64(creds.APISecret, strToSign)
	// Passphrase для v2 ключів теж підписується
	passphrase := hmacSHA256Base64(creds.APISecret, creds.Passphrase)
	return map[string]string{
		"KC-API-KEY":         creds.APIKey,
		"KC-API-SIGN":        sign,
		"KC-API-TIMESTAMP":   ts,
		"KC-API-PASSPHRASE":  passphrase,
		"KC-API-KEY-VERSION": "2",
		"Content-Type":       "application/json",
	}
}

func (ku *KuCoin) signedGet(path string, params map[string]string, creds Credentials, dest interface{}) error {
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	query := vals.Encode()
	fullPath := path
	if query != "" {
		fullPath = path + "?" + query
	}
	headers := ku.authHeaders("GET", fullPath, "", creds)
	return getJSON(kucoinBaseURL+fullPath, headers, dest)
}

// --- Balances ---

func (ku *KuCoin) GetBalances(creds Credentials) ([]Balance, error) {
	var resp struct {
		Code string `json:"code"`
		Data []struct {
			Currency  string `json:"currency"`
			Type      string `json:"type"` // "main" | "trade" | "margin"
			Balance   string `json:"balance"`
			Available string `json:"available"`
			Holds     string `json:"holds"`
		} `json:"data"`
	}
	if err := ku.signedGet("/api/v1/accounts", nil, creds, &resp); err != nil {
		return nil, fmt.Errorf("kucoin balances: %w", err)
	}
	if resp.Code != "200000" {
		return nil, fmt.Errorf("kucoin balances code: %s", resp.Code)
	}

	// Сумуємо однакові валюти по всіх типах акаунтів
	totals := make(map[string]float64)
	for _, a := range resp.Data {
		v := parseFloat(a.Balance)
		if v > 0 {
			totals[a.Currency] += v
		}
	}

	var result []Balance
	for sym, qty := range totals {
		if qty > 1e-8 {
			result = append(result, Balance{Symbol: sym, Quantity: qty})
		}
	}
	return result, nil
}

// --- Open Positions ---
// KuCoin Futures має окремий домен (api-futures.kucoin.com).
// Повертаємо порожній список — futures будуть додані в Фазі 3.

func (ku *KuCoin) GetOpenPositions(_ Credentials) ([]Position, error) {
	return nil, nil
}

// --- Closed Trades (spot fills) ---

func (ku *KuCoin) GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error) {
	params := map[string]string{
		"tradeType":   "TRADE",
		"startAt":     strconv.FormatInt(startMs, 10),
		"endAt":       strconv.FormatInt(endMs, 10),
		"pageSize":    "50",
		"currentPage": "1",
	}
	var resp struct {
		Code string `json:"code"`
		Data struct {
			Items []struct {
				Symbol    string `json:"symbol"`
				Side      string `json:"side"` // "buy" | "sell"
				Price     string `json:"price"`
				Size      string `json:"size"`
				Funds     string `json:"funds"`
				Fee       string `json:"fee"`
				CreatedAt int64  `json:"createdAt"` // ms
			} `json:"items"`
		} `json:"data"`
	}
	if err := ku.signedGet("/api/v1/fills", params, creds, &resp); err != nil {
		return nil, fmt.Errorf("kucoin fills: %w", err)
	}
	if resp.Code != "200000" {
		return nil, nil
	}
	var result []ClosedTrade
	for _, t := range resp.Data.Items {
		side := "LONG"
		if t.Side == "sell" {
			side = "SHORT"
		}
		price := parseFloat(t.Price)
		qty := parseFloat(t.Size)
		result = append(result, ClosedTrade{
			Symbol:     normalizeSymbol(strings.ReplaceAll(t.Symbol, "-", "")),
			Side:       side,
			Quantity:   qty,
			EntryPrice: price,
			ClosePrice: price,
			PnL:        0, // spot fills не мають PnL — обрахунок потребує позиції
			ClosedAt:   t.CreatedAt,
		})
	}
	return result, nil
}

// --- Prices ---

func (ku *KuCoin) GetPrices(_ []string) (map[string]float64, error) {
	var resp struct {
		Code string `json:"code"`
		Data struct {
			Ticker []struct {
				Symbol string `json:"symbol"`
				Last   string `json:"last"`
			} `json:"ticker"`
		} `json:"data"`
	}
	if err := getPublic(kucoinBaseURL+"/api/v1/market/allTickers", &resp); err != nil {
		return nil, fmt.Errorf("kucoin prices: %w", err)
	}
	prices := make(map[string]float64)
	for _, t := range resp.Data.Ticker {
		if !strings.HasSuffix(t.Symbol, "-USDT") {
			continue
		}
		price := parseFloat(t.Last)
		base := normalizeSymbol(strings.ReplaceAll(t.Symbol, "-", ""))
		prices[base+"USDT"] = price
		prices[base] = price
	}
	return prices, nil
}
