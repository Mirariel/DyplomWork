// Package notify надсилає push-сповіщення через Telegram Bot API.
//
// Якщо TELEGRAM_BOT_TOKEN або TELEGRAM_CHAT_ID не задані — всі виклики є no-op.
// Це дозволяє запускати сервер без Telegram і увімкнути сповіщення пізніше
// без зміни коду — лише додати змінні в .env.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const telegramAPI = "https://api.telegram.org/bot%s/sendMessage"

// Notifier надсилає повідомлення через Telegram.
type Notifier struct {
	token  string
	chatID string
	client *http.Client
	logger *slog.Logger
}

// New створює Notifier. Якщо token або chatID порожні — всі Send є no-op.
func New(token, chatID string, logger *slog.Logger) *Notifier {
	return &Notifier{
		token:  token,
		chatID: chatID,
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger,
	}
}

// Enabled повертає true якщо Telegram сповіщення налаштовані.
func (n *Notifier) Enabled() bool {
	return n.token != "" && n.chatID != ""
}

// Send надсилає текстове повідомлення (Markdown V2).
// Не блокує — виконується в окремій goroutine.
func (n *Notifier) Send(text string) {
	if !n.Enabled() {
		return
	}
	go n.send(text)
}

func (n *Notifier) send(text string) {
	body, _ := json.Marshal(map[string]string{
		"chat_id":    n.chatID,
		"text":       text,
		"parse_mode": "HTML",
	})

	url := fmt.Sprintf(telegramAPI, n.token)
	resp, err := n.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		n.logger.Warn("telegram: send failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		n.logger.Warn("telegram: unexpected status", "status", resp.StatusCode)
	}
}

// ─── Typed notification helpers ──────────────────────────────────────────────

// SmartOrderTriggered — сповіщення коли smart order спрацював.
func (n *Notifier) SmartOrderTriggered(orderType, symbol, exchange string, price float64) {
	emoji := map[string]string{
		"stop_loss":     "🛑",
		"take_profit":   "✅",
		"trailing_stop": "📉",
	}
	e := emoji[orderType]
	if e == "" {
		e = "🔔"
	}
	n.Send(fmt.Sprintf(
		"%s <b>Smart Order Triggered</b>\n"+
			"Type: <code>%s</code>\n"+
			"Symbol: <b>%s</b> on %s\n"+
			"Trigger price: <b>$%.4f</b>",
		e, orderType, symbol, exchange, price,
	))
}

// DCABought — сповіщення після успішної DCA покупки.
func (n *Notifier) DCABought(symbol, exchange string, qty, amountUSD, price float64) {
	n.Send(fmt.Sprintf(
		"🤖 <b>DCA Buy Executed</b>\n"+
			"Symbol: <b>%s</b> on %s\n"+
			"Bought: <b>%.6f</b> for <b>$%.2f</b>\n"+
			"Price: <b>$%.4f</b>",
		symbol, exchange, qty, amountUSD, price,
	))
}

// GridBotError — сповіщення коли grid bot зупинився через помилку.
func (n *Notifier) GridBotError(botID int64, symbol, exchange, errMsg string) {
	n.Send(fmt.Sprintf(
		"⚠️ <b>Grid Bot Stopped</b>\n"+
			"Bot ID: <code>%d</code>\n"+
			"Symbol: <b>%s</b> on %s\n"+
			"Error: <code>%s</code>",
		botID, symbol, exchange, errMsg,
	))
}

// SmartOrderFailed — сповіщення коли smart order не вдалось виконати.
func (n *Notifier) SmartOrderFailed(orderID int64, symbol, exchange, errMsg string) {
	n.Send(fmt.Sprintf(
		"❌ <b>Smart Order Failed</b>\n"+
			"Order ID: <code>%d</code>\n"+
			"Symbol: <b>%s</b> on %s\n"+
			"Error: <code>%s</code>",
		orderID, symbol, exchange, errMsg,
	))
}
