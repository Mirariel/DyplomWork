package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/golang-jwt/jwt/v5"
	appMiddleware "github.com/okochadmytro/tradetracker/internal/middleware"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second // < pongWait
	maxMessageSize = 512
)

// Client — одне WebSocket-з'єднання.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	userID    int64  // 0 до авторизації
	jwtSecret string
	logger    *log.Logger
}

func NewClient(hub *Hub, conn *websocket.Conn, jwtSecret string, logger *log.Logger) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 64),
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// incomingMsg — повідомлення від клієнта
type incomingMsg struct {
	Type  string `json:"type"`
	Token string `json:"token"` // JWT токен для auth
}

// ReadPump читає повідомлення від клієнта (одна goroutine на клієнт).
// При отриманні auth-повідомлення перевіряє JWT і зберігає userID.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Printf("[ws] read error: %v", err)
			}
			break
		}

		var msg incomingMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		if msg.Type == "auth" && msg.Token != "" {
			c.handleAuth(msg.Token)
		}
	}
}

// WritePump відправляє повідомлення клієнту (одна goroutine на клієнт).
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleAuth(tokenStr string) {
	token, err := jwt.ParseWithClaims(tokenStr, &appMiddleware.Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(c.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		c.sendJSON(map[string]string{"type": "auth_error", "message": "invalid token"})
		return
	}

	claims, ok := token.Claims.(*appMiddleware.Claims)
	if !ok || claims.UserID == 0 {
		c.sendJSON(map[string]string{"type": "auth_error", "message": "invalid claims"})
		return
	}

	c.userID = claims.UserID
	c.logger.Printf("[ws] user %d authenticated", c.userID)
	c.sendJSON(map[string]interface{}{"type": "auth_success", "user_id": c.userID})
}

func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}
