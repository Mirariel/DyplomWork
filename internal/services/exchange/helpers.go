package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

// hmacSHA256 обчислює HMAC-SHA256 і повертає hex-рядок (Binance, Bybit, KuCoin sign)
func hmacSHA256(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// hmacSHA256Base64 обчислює HMAC-SHA256 і повертає base64-рядок (KuCoin)
func hmacSHA256Base64(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// hmacSHA512 обчислює HMAC-SHA512 і повертає hex-рядок (Gate.io)
func hmacSHA512(secret, data string) string {
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// sha512Hex обчислює SHA512 hash рядка і повертає hex-рядок (Gate.io body hash)
func sha512Hex(data string) string {
	h := sha512.Sum512([]byte(data))
	return hex.EncodeToString(h[:])
}

// nowMs повертає поточний час у мілісекундах (Unix)
func nowMs() int64 {
	return time.Now().UnixMilli()
}

// normalizeSymbol перетворює символ будь-якої біржі до базового тікера.
// "BTC-USDT-SWAP" → "BTC", "BTCUSDT" → "BTC", "BTC_USDT" → "BTC"
func normalizeSymbol(symbol string) string {
	suffixes := []string{
		"-USDT-SWAP", "-USDC-SWAP", "-USD-SWAP",
		"-USDT-PERP", "-USDC-PERP",
		"-USDT", "-USDC", "-USD", "-BTC", "-ETH", "-BNB",
		"_USDT", "_USDC", "_USD", "_BTC", "_ETH",
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
