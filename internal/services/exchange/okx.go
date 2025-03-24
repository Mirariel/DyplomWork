package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

const okxBaseURL = "https://www.okx.com"

type OKX struct{}

func NewOKX() *OKX { return &OKX{} }

func (o *OKX) Name() string { return "okx" }

// --- Auth ---

// okxSign генерує підпис і повертає заголовки для авторизованого запиту OKX V5.
func okxSign(method, path, queryString, apiSecret string) (string, string) {
	// OKX timestamp: "2024-01-15T10:30:00.123Z"
	now := time.Now().UTC()
	ms := fmt.Sprintf("%03d", now.UnixMilli()%1000)
	timestamp := now.Format("2006-01-02T15:04:05") + "." + ms + "Z"

	prehash := timestamp + strings.ToUpper(method) + path + queryString
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(prehash))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return timestamp, sign
}

func (o *OKX) authHeaders(method, path, query string, creds Credentials) map[string]string {
	ts, sign := okxSign(method, path, query, creds.APISecret)
	return map[string]string{
		"Content-Type":         "application/json",
		"OK-ACCESS-KEY":        creds.APIKey,
		"OK-ACCESS-SIGN":       sign,
		"OK-ACCESS-TIMESTAMP":  ts,
		"OK-ACCESS-PASSPHRASE": creds.Passphrase,
	}
}

func (o *OKX) get(path string, params map[string]string, creds Credentials, dest interface{}) error {
	query := ""
	if len(params) > 0 {
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, v)
		}
		query = "?" + vals.Encode()
	}
	fullURL := okxBaseURL + path + query
	headers := o.authHeaders("GET", path, query, creds)
	return getJSON(fullURL, headers, dest)
}

// --- Balances ---

func (o *OKX) GetBalances(creds Credentials) ([]Balance, error) {
	totals := make(map[string]float64)

	// 1. Trading account
	var tradingResp struct {
		Code string `json:"code"`
		Data []struct {
			Details []struct {
				Ccy string `json:"ccy"`
				Eq  string `json:"eq"`
				Bal string `json:"bal"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := o.get("/api/v5/account/balance", nil, creds, &tradingResp); err == nil && tradingResp.Code == "0" {
		for _, d := range tradingResp.Data {
			for _, a := range d.Details {
				v := parseFloat(a.Eq)
				if v == 0 {
					v = parseFloat(a.Bal)
				}
				if v > 0 {
					totals[a.Ccy] += v
				}
			}
		}
	}

	// 2. Funding account
	var fundingResp struct {
		Code string `json:"code"`
		Data []struct {
			Ccy string `json:"ccy"`
			Bal string `json:"bal"`
		} `json:"data"`
	}
	if err := o.get("/api/v5/asset/balances", nil, creds, &fundingResp); err == nil && fundingResp.Code == "0" {
		for _, a := range fundingResp.Data {
			if v := parseFloat(a.Bal); v > 0 {
				totals[a.Ccy] += v
			}
		}
	}

	// 3. Earn (savings, staking, ETH2)
	earnPaths := []string{
		"/api/v5/finance/savings/balance",
		"/api/v5/finance/fixed-income/staking-data",
		"/api/v5/finance/ethstaking/balance",
	}
	for _, path := range earnPaths {
		var earnResp struct {
			Code string `json:"code"`
			Data []struct {
				Ccy      string `json:"ccy"`
				CcyId    string `json:"ccyId"`
				Amt      string `json:"amt"`
				Bal      string `json:"bal"`
				TotalAmt string `json:"totalAmt"`
			} `json:"data"`
		}
		if err := o.get(path, nil, creds, &earnResp); err == nil && earnResp.Code == "0" {
			for _, a := range earnResp.Data {
				ccy := a.Ccy
				if ccy == "" {
					ccy = a.CcyId
				}
				if ccy == "" {
					ccy = "ETH"
				}
				v := parseFloat(a.Amt)
				if v == 0 {
					v = parseFloat(a.Bal)
				}
				if v == 0 {
					v = parseFloat(a.TotalAmt)
				}
				if ccy != "" && v > 0 {
					totals[ccy] += v
				}
			}
		}
	}

	var result []Balance
	for ccy, amt := range totals {
		if amt > 1e-8 {
			result = append(result, Balance{Symbol: ccy, Quantity: amt})
		}
	}
	return result, nil
}

// --- Open Positions ---

func (o *OKX) GetOpenPositions(creds Credentials) ([]Position, error) {
	var resp struct {
		Code string `json:"code"`
		Data []struct {
			InstId    string `json:"instId"`
			PosSide   string `json:"posSide"`
			Pos       string `json:"pos"`
			AvgPx     string `json:"avgPx"`
			MarkPx    string `json:"markPx"`
			Upl       string `json:"upl"`
			Lever     string `json:"lever"`
			MgnMode   string `json:"mgnMode"`
			LiqPx     string `json:"liqPx"`
			Margin    string `json:"margin"`
			Imr       string `json:"imr"`
			NotionalUsd string `json:"notionalUsd"`
		} `json:"data"`
	}

	if err := o.get("/api/v5/account/positions", nil, creds, &resp); err != nil {
		return nil, fmt.Errorf("okx positions: %w", err)
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx positions API code %s", resp.Code)
	}

	var result []Position
	for _, p := range resp.Data {
		qty := parseFloat(p.Pos)
		if qty == 0 {
			continue
		}
		side := strings.ToUpper(p.PosSide)
		if side == "NET" {
			if qty > 0 {
				side = "LONG"
			} else {
				side = "SHORT"
			}
		}
		result = append(result, Position{
			Symbol:     normalizeSymbol(p.InstId),
			Side:       side,
			Quantity:   math.Abs(qty),
			EntryPrice: parseFloat(p.AvgPx),
			MarkPrice:  parseFloat(p.MarkPx),
			Leverage:   int(parseFloat(p.Lever)),
			PnL:        parseFloat(p.Upl),
		})
	}
	return result, nil
}

// --- Closed Trades ---

func (o *OKX) GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error) {
	var resp struct {
		Code string `json:"code"`
		Data []struct {
			InstId        string `json:"instId"`
			PosSide       string `json:"posSide"`
			CloseTotalPos string `json:"closeTotalPos"`
			OpenAvgPx     string `json:"openAvgPx"`
			CloseAvgPx    string `json:"closeAvgPx"`
			RealizedPnl   string `json:"realizedPnl"`
			CTime         string `json:"cTime"`
			UTime         string `json:"uTime"`
			Lever         string `json:"lever"`
		} `json:"data"`
	}

	if err := o.get("/api/v5/account/positions-history", nil, creds, &resp); err != nil {
		return nil, fmt.Errorf("okx history: %w", err)
	}
	if resp.Code != "0" {
		return nil, nil
	}

	var result []ClosedTrade
	for _, item := range resp.Data {
		side := strings.ToUpper(item.PosSide)
		qty := parseFloat(item.CloseTotalPos)
		if side == "NET" {
			if qty > 0 {
				side = "LONG"
			} else {
				side = "SHORT"
			}
		}
		closedMs := parseInt64(item.UTime)
		if closedMs < startMs || closedMs > endMs {
			continue
		}
		result = append(result, ClosedTrade{
			Symbol:     normalizeSymbol(item.InstId),
			Side:       side,
			Quantity:   math.Abs(qty),
			EntryPrice: parseFloat(item.OpenAvgPx),
			ClosePrice: parseFloat(item.CloseAvgPx),
			PnL:        parseFloat(item.RealizedPnl),
			ClosedAt:   closedMs,
		})
	}
	return result, nil
}

// --- Prices ---

func (o *OKX) GetPrices(symbols []string) (map[string]float64, error) {
	url := okxBaseURL + "/api/v5/market/tickers?instType=SPOT"

	var resp struct {
		Code string `json:"code"`
		Data []struct {
			InstId string `json:"instId"`
			Last   string `json:"last"`
		} `json:"data"`
	}

	if err := getPublic(url, &resp); err != nil {
		return nil, fmt.Errorf("okx prices: %w", err)
	}

	prices := make(map[string]float64)
	for _, t := range resp.Data {
		if strings.HasSuffix(t.InstId, "-USDT") {
			base := normalizeSymbol(t.InstId)
			price := parseFloat(t.Last)
			prices[base+"USDT"] = price
			prices[base] = price
		}
	}
	return prices, nil
}
