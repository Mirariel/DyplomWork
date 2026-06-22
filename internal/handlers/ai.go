package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/middleware"
)

type AIHandler struct {
	db           *sqlx.DB
	anthropicKey string
}

func NewAIHandler(db *sqlx.DB, anthropicKey string) *AIHandler {
	return &AIHandler{db: db, anthropicKey: anthropicKey}
}

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// POST /api/ai/ask
func (h *AIHandler) Ask(c *fiber.Ctx) error {
	if h.anthropicKey == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "AI advisor not configured (ANTHROPIC_API_KEY missing)",
		})
	}

	userID := middleware.GetUserID(c)

	var body struct {
		Message string      `json:"message"`
		History []aiMessage `json:"history"`
	}
	if err := c.BodyParser(&body); err != nil || body.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "message required"})
	}

	ctx := h.buildContext(userID)
	systemPrompt := h.buildSystemPrompt(ctx)

	// Додаємо нове повідомлення до history
	messages := append(body.History, aiMessage{Role: "user", Content: body.Message})

	// Обрізаємо history до останніх 10 повідомлень (економія токенів)
	if len(messages) > 10 {
		messages = messages[len(messages)-10:]
	}

	resp, err := h.callClaude(systemPrompt, messages)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"reply": resp})
}

type tradeContext struct {
	TotalTrades   int
	WinningTrades int
	WinRate       float64
	TotalPnL      float64
	BestTrade     float64
	WorstTrade    float64
	TopSymbols    []string
	RecentTrades  []string
}

func (h *AIHandler) buildContext(userID int64) tradeContext {
	var ctx tradeContext

	_ = h.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(realized_pnl), 0),
		       COALESCE(MAX(realized_pnl), 0),
		       COALESCE(MIN(realized_pnl), 0)
		FROM position_history WHERE user_id = ?`, userID).
		Scan(&ctx.TotalTrades, &ctx.WinningTrades, &ctx.TotalPnL, &ctx.BestTrade, &ctx.WorstTrade)

	if ctx.TotalTrades > 0 {
		ctx.WinRate = float64(ctx.WinningTrades) / float64(ctx.TotalTrades) * 100
	}

	rows, _ := h.db.Query(`
		SELECT symbol, COALESCE(SUM(realized_pnl), 0) AS pnl
		FROM position_history WHERE user_id = ?
		GROUP BY symbol ORDER BY pnl DESC LIMIT 5`, userID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var sym string
			var pnl float64
			if rows.Scan(&sym, &pnl) == nil {
				ctx.TopSymbols = append(ctx.TopSymbols, fmt.Sprintf("%s ($%.2f)", sym, pnl))
			}
		}
	}

	recent, _ := h.db.Query(`
		SELECT symbol, side, realized_pnl, closed_at
		FROM position_history WHERE user_id = ?
		ORDER BY closed_at DESC LIMIT 5`, userID)
	if recent != nil {
		defer recent.Close()
		for recent.Next() {
			var sym, side string
			var pnl float64
			var closedAt *string
			if recent.Scan(&sym, &side, &pnl, &closedAt) == nil {
				ctx.RecentTrades = append(ctx.RecentTrades, fmt.Sprintf("%s %s $%.2f", sym, side, pnl))
			}
		}
	}

	return ctx
}

func (h *AIHandler) buildSystemPrompt(ctx tradeContext) string {
	return fmt.Sprintf(`You are an expert crypto trading advisor for TradeTracker platform.
Analyze the user's trading statistics and provide concise, actionable advice in English or Ukrainian (match the user's language).

User Trading Statistics:
- Total trades: %d
- Win rate: %.1f%% (%d winning trades)
- Total realized PnL: $%.2f
- Best trade: $%.2f | Worst trade: $%.2f
- Top symbols by PnL: %v
- Recent trades: %v

Be direct and practical. Keep responses under 200 words. Focus on what the data tells you.`,
		ctx.TotalTrades, ctx.WinRate, ctx.WinningTrades,
		ctx.TotalPnL, ctx.BestTrade, ctx.WorstTrade,
		ctx.TopSymbols, ctx.RecentTrades)
}

func (h *AIHandler) callClaude(system string, messages []aiMessage) (string, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      "claude-opus-4-6",
		"max_tokens": 1024,
		"system":     system,
		"messages":   messages,
	})

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", h.anthropicKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("claude API: %s", result.Error.Message)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from claude")
	}
	return result.Content[0].Text, nil
}
