package exchange

// Binance trading implementation — реалізує інтерфейс Trader.
//
// Spot:    POST https://api.binance.com/api/v3/order
// Futures: POST https://fapi.binance.com/fapi/v1/order
//
// Auth: ті ж самі HMAC-SHA256 заголовки що і для читання,
// але параметри (включно з підписом) передаються у form-body POST-запиту.

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Compile-time перевірка що Binance реалізує Trader
var _ Trader = (*Binance)(nil)

func (b *Binance) PlaceOrder(creds Credentials, req PlaceOrderRequest) (PlacedOrder, error) {
	ts := strconv.FormatInt(nowMs(), 10)

	params := url.Values{}
	params.Set("symbol", binanceSymbol(req.Symbol, req.Category))
	params.Set("side", strings.ToUpper(string(req.Side)))
	params.Set("type", strings.ToUpper(string(req.Type)))
	params.Set("quantity", strconv.FormatFloat(req.Quantity, 'f', 8, 64))
	params.Set("timestamp", ts)

	if req.Type == OrderTypeLimit {
		params.Set("price", strconv.FormatFloat(req.Price, 'f', 8, 64))
		params.Set("timeInForce", "GTC")
	}

	// Підписуємо закодований рядок параметрів (без signature)
	signed := params.Encode()
	params.Set("signature", hmacSHA256(creds.APISecret, signed))

	var resp struct {
		OrderId      int64   `json:"orderId"`
		Symbol       string  `json:"symbol"`
		Side         string  `json:"side"`
		Type         string  `json:"type"`
		Status       string  `json:"status"`
		OrigQty      string  `json:"origQty"`
		Price        string  `json:"price"`
		ExecutedQty  string  `json:"executedQty"`
		Code         int     `json:"code"`
		Msg          string  `json:"msg"`
	}

	endpoint := binanceOrderURL(req.Category)
	headers := map[string]string{"X-MBX-APIKEY": creds.APIKey}

	if err := postForm(endpoint, params, headers, &resp); err != nil {
		return PlacedOrder{}, fmt.Errorf("binance place order: %w", err)
	}
	if resp.Code != 0 && resp.Msg != "" {
		return PlacedOrder{}, fmt.Errorf("binance order API: %s", resp.Msg)
	}

	return PlacedOrder{
		OrderID:   strconv.FormatInt(resp.OrderId, 10),
		Symbol:    req.Symbol,
		Side:      req.Side,
		Type:      req.Type,
		Category:  req.Category,
		Quantity:  req.Quantity,
		Price:     parseFloat(resp.Price),
		FilledQty: parseFloat(resp.ExecutedQty),
		Status:    binanceNormalizeStatus(resp.Status),
	}, nil
}

func (b *Binance) CancelOrder(creds Credentials, req CancelOrderRequest) error {
	ts := strconv.FormatInt(nowMs(), 10)

	params := url.Values{}
	params.Set("symbol", binanceSymbol(req.Symbol, req.Category))
	params.Set("orderId", req.OrderID)
	params.Set("timestamp", ts)

	signed := params.Encode()
	params.Set("signature", hmacSHA256(creds.APISecret, signed))

	endpoint := binanceCancelURL(req.Category) + "?" + params.Encode()
	headers := map[string]string{"X-MBX-APIKEY": creds.APIKey}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := deleteJSON(endpoint, headers, &resp); err != nil {
		return fmt.Errorf("binance cancel order: %w", err)
	}
	if resp.Code != 0 && resp.Msg != "" {
		return fmt.Errorf("binance cancel API: %s", resp.Msg)
	}
	return nil
}

func (b *Binance) GetOrderStatus(creds Credentials, req CancelOrderRequest) (PlacedOrder, error) {
	ts := strconv.FormatInt(nowMs(), 10)

	query := fmt.Sprintf("symbol=%s&orderId=%s&timestamp=%s",
		binanceSymbol(req.Symbol, req.Category), req.OrderID, ts)
	sig := hmacSHA256(creds.APISecret, query)

	endpoint := binanceStatusURL(req.Category) + "?" + query + "&signature=" + sig
	headers := map[string]string{"X-MBX-APIKEY": creds.APIKey}

	var resp struct {
		OrderId     int64  `json:"orderId"`
		Symbol      string `json:"symbol"`
		Side        string `json:"side"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		OrigQty     string `json:"origQty"`
		Price       string `json:"price"`
		ExecutedQty string `json:"executedQty"`
		Code        int    `json:"code"`
		Msg         string `json:"msg"`
	}
	if err := getJSON(endpoint, headers, &resp); err != nil {
		return PlacedOrder{}, fmt.Errorf("binance order status: %w", err)
	}
	if resp.Code != 0 && resp.Msg != "" {
		return PlacedOrder{}, fmt.Errorf("binance status API: %s", resp.Msg)
	}

	return PlacedOrder{
		OrderID:   strconv.FormatInt(resp.OrderId, 10),
		Symbol:    req.Symbol,
		Side:      OrderSide(strings.ToLower(resp.Side)),
		Type:      OrderType(strings.ToLower(resp.Type)),
		Category:  req.Category,
		Quantity:  parseFloat(resp.OrigQty),
		Price:     parseFloat(resp.Price),
		FilledQty: parseFloat(resp.ExecutedQty),
		Status:    binanceNormalizeStatus(resp.Status),
	}, nil
}

// --- helpers ---

// binanceSymbol конвертує базовий тікер у формат Binance.
// "BTC" → "BTCUSDT" (для spot та futures)
func binanceSymbol(base string, _ OrderCategory) string {
	return strings.ToUpper(base) + "USDT"
}

func binanceOrderURL(cat OrderCategory) string {
	if cat == OrderCategoryFutures {
		return binanceFuturesURL + "/fapi/v1/order"
	}
	return binanceSpotURL + "/api/v3/order"
}

func binanceCancelURL(cat OrderCategory) string {
	if cat == OrderCategoryFutures {
		return binanceFuturesURL + "/fapi/v1/order"
	}
	return binanceSpotURL + "/api/v3/order"
}

func binanceStatusURL(cat OrderCategory) string {
	if cat == OrderCategoryFutures {
		return binanceFuturesURL + "/fapi/v1/order"
	}
	return binanceSpotURL + "/api/v3/order"
}

// binanceNormalizeStatus конвертує Binance статус у внутрішній.
func binanceNormalizeStatus(s string) OrderStatus {
	switch strings.ToUpper(s) {
	case "NEW", "PENDING_NEW":
		return OrderStatusNew
	case "PARTIALLY_FILLED":
		return OrderStatusPartial
	case "FILLED":
		return OrderStatusFilled
	case "CANCELED", "CANCELLED", "PENDING_CANCEL", "EXPIRED":
		return OrderStatusCancelled
	case "REJECTED":
		return OrderStatusRejected
	}
	return OrderStatusNew
}
