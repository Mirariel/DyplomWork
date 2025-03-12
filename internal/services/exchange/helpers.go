package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// hmacSHA256 обчислює HMAC-SHA256 підпис і повертає hex-рядок
func hmacSHA256(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// nowMs повертає поточний час у мілісекундах (Unix)
func nowMs() int64 {
	return time.Now().UnixMilli()
}

// normalizeSymbol перетворює символ будь-якої біржі до базового тікера.
// "BTC-USDT-SWAP" → "BTC", "BTCUSDT" → "BTC", "BTC-USDT" → "BTC"
func normalizeSymbol(symbol string) string {
	suffixes := []string{
		"-USDT-SWAP", "-USDC-SWAP", "-USD-SWAP",
		"-USDT-PERP", "-USDC-PERP",
		"-USDT", "-USDC", "-USD", "-BTC", "-ETH", "-BNB",
		"USDT", "USDC", "BUSD", "FDUSD",
	}
	s := strings.ToUpper(strings.TrimSpace(symbol))
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSuffix(s, suf)
			break
		}
	}
	return s
}

// msToTime конвертує Unix ms у time.Time
func msToTime(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}
