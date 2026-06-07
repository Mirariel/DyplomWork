package exchange

// OKX trading implementation — реалізує інтерфейс Trader.
//
// Ордери: POST /api/v5/trade/order
// Скасування: POST /api/v5/trade/cancel-order
// Статус: GET /api/v5/trade/order
//
// Auth: той самий HMAC-SHA256 base64, але для POST запитів
// JSON body включається в рядок підпису замість query string:
//   prehash = timestamp + "POST" + path + json_body

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Compile-time перевірка що OKX реалізує Trader
var _ Trader = (*OKX)(nil)

func (o *OKX) PlaceOrder(creds Credentials, req PlaceOrderRequest) (PlacedOrder, error) {
	path := "/api/v5/trade/order"

	body := map[string]string{
		"instId":  okxSymbol(req.Symbol, req.Category),
		"tdMode":  okxTradeMode(req.Category),
		"side":    string(req.Side),
		"ordType": okxOrderType(req.Type),
		"sz":      fmt.Sprintf("%.8f", req.Quantity),
	}
	if req.Type == OrderTypeLimit {
		body["px"] = fmt.Sprintf("%.8f", req.Price)
	}

	bodyBytes, _ := json.Marshal(body)
	bodyStr := string(bodyBytes)

	ts, sign := okxSign("POST", path, bodyStr, creds.APISecret)
	headers := map[string]string{
		"OK-ACCESS-KEY":        creds.APIKey,
		"OK-ACCESS-SIGN":       sign,
		"OK-ACCESS-TIMESTAMP":  ts,
		"OK-ACCESS-PASSPHRASE": creds.Passphrase,
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			OrdId   string `json:"ordId"`
			ClOrdId string `json:"clOrdId"`
			SCode   string `json:"sCode"`
			SMsg    string `json:"sMsg"`
		} `json:"data"`
	}
	if err := postJSON(okxBaseURL+path, bodyStr, headers, &resp); err != nil {
		return PlacedOrder{}, fmt.Errorf("okx place order: %w", err)
	}
	if resp.Code != "0" {
		return PlacedOrder{}, fmt.Errorf("okx order API code=%s: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return PlacedOrder{}, fmt.Errorf("okx place order: empty response")
	}
	if resp.Data[0].SCode != "0" {
		return PlacedOrder{}, fmt.Errorf("okx order error: %s", resp.Data[0].SMsg)
	}

	return PlacedOrder{
		OrderID:  resp.Data[0].OrdId,
		Symbol:   req.Symbol,
		Side:     req.Side,
		Type:     req.Type,
		Category: req.Category,
		Quantity: req.Quantity,
		Price:    req.Price,
		Status:   OrderStatusNew,
	}, nil
}

func (o *OKX) CancelOrder(creds Credentials, req CancelOrderRequest) error {
	path := "/api/v5/trade/cancel-order"

	body := map[string]string{
		"instId": okxSymbol(req.Symbol, req.Category),
		"ordId":  req.OrderID,
	}
	bodyBytes, _ := json.Marshal(body)
	bodyStr := string(bodyBytes)

	ts, sign := okxSign("POST", path, bodyStr, creds.APISecret)
	headers := map[string]string{
		"OK-ACCESS-KEY":        creds.APIKey,
		"OK-ACCESS-SIGN":       sign,
		"OK-ACCESS-TIMESTAMP":  ts,
		"OK-ACCESS-PASSPHRASE": creds.Passphrase,
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := postJSON(okxBaseURL+path, bodyStr, headers, &resp); err != nil {
		return fmt.Errorf("okx cancel order: %w", err)
	}
	if resp.Code != "0" {
		return fmt.Errorf("okx cancel API code=%s: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (o *OKX) GetOrderStatus(creds Credentials, req CancelOrderRequest) (PlacedOrder, error) {
	path := "/api/v5/trade/order"
	query := fmt.Sprintf("?instId=%s&ordId=%s", okxSymbol(req.Symbol, req.Category), req.OrderID)

	ts, sign := okxSign("GET", path, query, creds.APISecret)
	headers := map[string]string{
		"OK-ACCESS-KEY":        creds.APIKey,
		"OK-ACCESS-SIGN":       sign,
		"OK-ACCESS-TIMESTAMP":  ts,
		"OK-ACCESS-PASSPHRASE": creds.Passphrase,
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			OrdId   string `json:"ordId"`
			InstId  string `json:"instId"`
			Side    string `json:"side"`
			OrdType string `json:"ordType"`
			Sz      string `json:"sz"`
			Px      string `json:"px"`
			AccFillSz string `json:"accFillSz"`
			AvgPx   string `json:"avgPx"`
			State   string `json:"state"`
		} `json:"data"`
	}
	if err := getJSON(okxBaseURL+path+query, headers, &resp); err != nil {
		return PlacedOrder{}, fmt.Errorf("okx order status: %w", err)
	}
	if resp.Code != "0" || len(resp.Data) == 0 {
		return PlacedOrder{}, fmt.Errorf("okx status API code=%s", resp.Code)
	}

	d := resp.Data[0]
	return PlacedOrder{
		OrderID:   d.OrdId,
		Symbol:    req.Symbol,
		Side:      OrderSide(d.Side),
		Type:      OrderType(d.OrdType),
		Category:  req.Category,
		Quantity:  parseFloat(d.Sz),
		Price:     parseFloat(d.Px),
		FilledQty: parseFloat(d.AccFillSz),
		AvgPrice:  parseFloat(d.AvgPx),
		Status:    okxNormalizeStatus(d.State),
	}, nil
}

// --- helpers ---

// okxSymbol конвертує базовий тікер у формат OKX.
// spot: "BTC" → "BTC-USDT"
// futures (perpetual swap): "BTC" → "BTC-USDT-SWAP"
func okxSymbol(base string, cat OrderCategory) string {
	b := strings.ToUpper(base)
	if cat == OrderCategoryFutures {
		return b + "-USDT-SWAP"
	}
	return b + "-USDT"
}

// okxTradeMode визначає режим торгівлі.
// cash — спот без маржі; cross — cross margin для futures.
func okxTradeMode(cat OrderCategory) string {
	if cat == OrderCategoryFutures {
		return "cross"
	}
	return "cash"
}

// okxOrderType конвертує OrderType у формат OKX.
func okxOrderType(t OrderType) string {
	if t == OrderTypeLimit {
		return "limit"
	}
	return "market"
}

// okxNormalizeStatus конвертує OKX статус у внутрішній.
func okxNormalizeStatus(s string) OrderStatus {
	switch s {
	case "live":
		return OrderStatusNew
	case "partially_filled":
		return OrderStatusPartial
	case "filled":
		return OrderStatusFilled
	case "canceled":
		return OrderStatusCancelled
	}
	return OrderStatusNew
}
