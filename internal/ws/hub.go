package ws

import (
	"sync"
)

// Hub — центральний реєстр WebSocket-з'єднань.
// Всі операції з клієнтами проходять через канали (thread-safe).
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool // всі підключені клієнти
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// authenticatedUsers повертає map[userID][]Client — лише авторизованих.
func (h *Hub) authenticatedUsers() map[int64][]*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	grouped := make(map[int64][]*Client)
	for c := range h.clients {
		if c.userID > 0 {
			grouped[c.userID] = append(grouped[c.userID], c)
		}
	}
	return grouped
}

// sendToUser надсилає повідомлення всім з'єднанням конкретного користувача.
func (h *Hub) sendToUser(userID int64, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.userID == userID {
			select {
			case c.send <- data:
			default:
				// Буфер повний — клієнт занадто повільний, пропускаємо
			}
		}
	}
}
