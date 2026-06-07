package exchange

// Bybit trading implementation — реалізує інтерфейс Trader.
//
// Розміщення: POST /v5/order/create
// Скасування: POST /v5/order/cancel
// Статус:     GET  /v5/order/realtime
//
// Auth: той самий HMAC-SHA256 що і для GET,
// але для POST підписується JSON body замість query string:
//   payload = timestamp + apiKey + recvWindow + json_body

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Compile-time перевірка що Bybit реалізує Trader
var _ Trader = (*Bybit)(nil)

func (by *Bybit) PlaceOrder(creds Credentials, req PlaceOrderRequest) (PlacedOrder, error) {
	body := map[string]interface{}{
		"category":  bybitCategory(req.Category),
		"symbol":    bybitSymbol(req.Symbol),
		"side":      bybitSide(req.Side),
		"orderType": bybitOrderType(req.Type),
		"qty":       strconv.FormatFloat(req.Quantity, 'f', 8, 64),
	}
	if req.Type == OrderTypeLimit {
		body["price"] = strconv.FormatFloat(req.Price, 'f', 8, 64)
		body["timeInForce"] = "GTC"
	}

	bodyBytes, _ := json.Marshal(body)
	bodyStr := string(bodyBytes)

	ts := strconv.FormatInt(nowMs(), 10)
	headers := by.authHeaders(ts, creds.APIKey, creds.APISecret, bodyStr)
	headers["Content-Type"] = "application/json"

	var resp struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			OrderId string `json:"orderId"`
		} `json:"result"`
	}
	if err := postJSON(bybitBaseURL+"/v5/order/create", bodyStr, headers, &resp); err != nil {
		return PlacedOrder{}, fmt.Errorf("bybit place order: %w", err)
	}
	if resp.RetCode != 0 {
		return PlacedOrder{}, fmt.Errorf("bybit order API: %s", resp.RetMsg)
	}

	return PlacedOrder{
		OrderID:  resp.Result.OrderId,
		Symbol:   req.Symbol,
		Side:     req.Side,
		Type:     req.Type,
		Category: req.Category,
		Quantity: req.Quantity,
		Price:    req.Price,
		Status:   OrderStatusNew,
	}, nil
}

func (by *Bybit) CancelOrder(creds Credentials, req CancelOrderRequest) error {
	body := map[string]string{
		"category": bybitCategory(req.Category),
		"symbol":   bybitSymbol(req.Symbol),
		"orderId":  req.OrderID,
	}
	bodyBytes, _ := json.Marshal(body)
	bodyStr := string(bodyBytes)

	ts := strconv.FormatInt(nowMs(), 10)
	headers := by.authHeaders(ts, creds.APIKey, creds.APISecret, bodyStr)
	headers["Content-Type"] = "application/json"

	var resp struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := postJSON(bybitBaseURL+"/v5/order/cancel", bodyStr, headers, &resp); err != nil {
		return fmt.Errorf("bybit cancel order: %w", err)
	}
	if resp.RetCode != 0 {
		return fmt.Errorf("bybit cancel API: %s", resp.RetMsg)
	}
	return nil
}

func (by *Bybit) GetOrderStatus(creds Credentials, req CancelOrderRequest) (PlacedOrder, error) {
	params := map[string]string{
		"category": bybitCategory(req.Category),
		"orderId":  req.OrderID,
	}
	var resp struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				OrderId     string `json:"orderId"`
				Symbol      string `json:"symbol"`
				Side        string `json:"side"`
				OrderType   string `json:"orderType"`
				Qty         string `json:"qty"`
				Price       string `json:"price"`
				CumExecQty  string `json:"cumExecQty"`
				AvgPrice    string `json:"avgPrice"`
				OrderStatus string `json:"orderStatus"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := by.signedGet("/v5/order/realtime", params, creds, &resp); err != nil {
		return PlacedOrder{}, fmt.Errorf("bybit order status: %w", err)
	}
	if resp.RetCode != 0 || len(resp.Result.List) == 0 {
		return PlacedOrder{}, fmt.Errorf("bybit status API: %s", resp.RetMsg)
	}

	o := resp.Result.List[0]
	return PlacedOrder{
		OrderID:   o.OrderId,
		Symbol:    req.Symbol,
		Side:      OrderSide(strings.ToLower(o.Side)),
		Type:      OrderType(strings.ToLower(o.OrderType)),
		Category:  req.Category,
		Quantity:  parseFloat(o.Qty),
		Price:     parseFloat(o.Price),
		FilledQty: parseFloat(o.CumExecQty),
		AvgPrice:  parseFloat(o.AvgPrice),
		Status:    bybitNormalizeStatus(o.OrderStatus),
	}, nil
}

// --- helpers ---

// bybitSymbol конвертує базовий тікер у формат Bybit (linear і spot).
// "BTC" → "BTCUSDT"
func bybitSymbol(base string) string {
	return strings.ToUpper(base) + "USDT"
}

// bybitCategory конвертує OrderCategory у рядок Bybit V5 API.
func bybitCategory(cat OrderCategory) string {
	if cat == OrderCategoryFutures {
		return "linear" // USDT perpetual
	}
	return "spot"
}

// bybitSide конвертує OrderSide у формат Bybit ("Buy" / "Sell").
func bybitSide(side OrderSide) string {
	if side == OrderSideBuy {
		return "Buy"
	}
	return "Sell"
}

// bybitOrderType конвертує OrderType у формат Bybit ("Market" / "Limit").
func bybitOrderType(t OrderType) string {
	if t == OrderTypeLimit {
		return "Limit"
	}
	return "Market"
}

// bybitNormalizeStatus конвертує Bybit статус у внутрішній.
func bybitNormalizeStatus(s string) OrderStatus {
	switch s {
	case "Created", "New", "Untriggered":
		return OrderStatusNew
	case "PartiallyFilled":
		return OrderStatusPartial
	case "Filled":
		return OrderStatusFilled
	case "Cancelled", "Deactivated":
		return OrderStatusCancelled
	case "Rejected":
		return OrderStatusRejected
	}
	return OrderStatusNew
}
