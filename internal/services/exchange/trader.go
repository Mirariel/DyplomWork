package exchange

// Trader — необов'язковий інтерфейс для бірж що підтримують торгівлю.
//
// Не всі біржі реалізують Trader. Перевіряти через type assertion:
//
//	if t, ok := ex.(exchange.Trader); ok {
//	    order, err := t.PlaceOrder(creds, req)
//	}
//
// Зараз реалізовано: Binance, OKX, Bybit.
// Gate.io / Kraken / KuCoin — в майбутніх фазах.

// OrderSide — напрям ордеру
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// OrderType — тип ордеру
type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

// OrderCategory — ринок де розміщується ордер
type OrderCategory string

const (
	OrderCategorySpot    OrderCategory = "spot"
	OrderCategoryFutures OrderCategory = "futures"
)

// OrderStatus — нормалізований статус ордеру (незалежно від біржі)
type OrderStatus string

const (
	OrderStatusNew       OrderStatus = "new"
	OrderStatusPartial   OrderStatus = "partial"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRejected  OrderStatus = "rejected"
)

// PlaceOrderRequest — параметри ордеру у внутрішньому форматі.
// Symbol — базовий тікер без quote ("BTC", "ETH") — адаптер конвертує сам.
type PlaceOrderRequest struct {
	Symbol   string        // "BTC", "ETH", "SOL" ...
	Side     OrderSide     // buy | sell
	Type     OrderType     // market | limit
	Category OrderCategory // spot | futures
	Quantity float64       // кількість базового активу
	Price    float64       // ціна для limit ордеру; 0 для market
	Leverage int           // плече для futures; 0 = не змінювати
}

// CancelOrderRequest — параметри для скасування ордеру.
type CancelOrderRequest struct {
	OrderID  string        // ID ордеру на біржі
	Symbol   string        // базовий символ ("BTC")
	Category OrderCategory // spot | futures
}

// PlacedOrder — результат розміщення або стан ордеру.
type PlacedOrder struct {
	OrderID   string      // ID ордеру на біржі
	Symbol    string      // базовий символ
	Side      OrderSide
	Type      OrderType
	Category  OrderCategory
	Quantity  float64
	Price     float64     // ціна з ордеру (для market — 0 або avg fill price)
	FilledQty float64     // вже виконана кількість
	AvgPrice  float64     // середня ціна виконання
	Status    OrderStatus
}

// Trader — інтерфейс торгівлі.
// Реалізується поверх Exchange адаптерів для бірж що підтримують API-торгівлю.
type Trader interface {
	// PlaceOrder розміщує новий ордер. Повертає PlacedOrder з OrderID.
	PlaceOrder(creds Credentials, req PlaceOrderRequest) (PlacedOrder, error)

	// CancelOrder скасовує ордер за Exchange Order ID.
	CancelOrder(creds Credentials, req CancelOrderRequest) error

	// GetOrderStatus повертає поточний стан ордеру.
	GetOrderStatus(creds Credentials, req CancelOrderRequest) (PlacedOrder, error)
}
