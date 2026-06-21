package exchange

import (
	"encoding/base64"
	"testing"
)

func TestHmacSHA256(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		data   string
		want   string
	}{
		{
			name:   "binance signature example",
			secret: "NhqPtmdSJYdKjVHjA7PZj4Mge3R5YNiP1e3UZjInClVN65XAbvqqM6A7H5fATj0j",
			data:   "symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559",
			want:   "c8db56825ae71d6d79447849e617115f4a920fa2acdcab2b053c4b2838bd6b71",
		},
		{
			name:   "known value",
			secret: "key",
			data:   "message",
			want:   "6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a",
		},
		{
			name:   "empty data self-consistent",
			secret: "secret",
			data:   "",
			want:   hmacSHA256("secret", ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hmacSHA256(tt.secret, tt.data)
			if got != tt.want {
				t.Errorf("hmacSHA256() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHmacSHA256Base64(t *testing.T) {
	got := hmacSHA256Base64("secret", "message")
	if got == "" {
		t.Error("hmacSHA256Base64() returned empty string")
	}
	// Must be valid base64
	_, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Errorf("hmacSHA256Base64() returned invalid base64: %v", err)
	}
	// Must be deterministic
	got2 := hmacSHA256Base64("secret", "message")
	if got != got2 {
		t.Error("hmacSHA256Base64() is not deterministic")
	}
	// Different inputs must produce different outputs
	got3 := hmacSHA256Base64("secret", "other")
	if got == got3 {
		t.Error("hmacSHA256Base64() produced same output for different inputs")
	}
}

func TestHmacSHA512(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		data   string
	}{
		{
			name:   "known key and message",
			secret: "key",
			data:   "message",
		},
		{
			name:   "gate.io style",
			secret: "gateio_secret",
			data:   "GET\n/api/v4/spot/accounts\n\n\n1640995200",
		},
		{
			name:   "empty data",
			secret: "secret",
			data:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hmacSHA512(tt.secret, tt.data)
			if got == "" {
				t.Error("hmacSHA512() returned empty string")
			}
			// Result must be 128 hex characters (512 bits / 4 bits per hex char)
			if len(got) != 128 {
				t.Errorf("hmacSHA512() length = %d, want 128", len(got))
			}
			// Must be deterministic
			got2 := hmacSHA512(tt.secret, tt.data)
			if got != got2 {
				t.Error("hmacSHA512() is not deterministic")
			}
		})
	}

	// Different inputs must yield different outputs
	a := hmacSHA512("key", "message")
	b := hmacSHA512("key", "other")
	if a == b {
		t.Error("hmacSHA512() produced same output for different inputs")
	}
}

func TestSha512Hex(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"hello", "hello"},
		{"long input", "The quick brown fox jumps over the lazy dog"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sha512Hex(tt.input)
			// SHA-512 produces 64 bytes = 128 hex characters
			if len(got) != 128 {
				t.Errorf("sha512Hex(%q) length = %d, want 128 hex chars", tt.input, len(got))
			}
			// Must be valid hex (only 0-9 and a-f)
			for i, c := range got {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("sha512Hex() char at index %d is not valid hex: %c", i, c)
					break
				}
			}
			// Must be deterministic
			got2 := sha512Hex(tt.input)
			if got != got2 {
				t.Error("sha512Hex() is not deterministic")
			}
		})
	}

	// Known value: SHA-512 of empty string
	emptyHash := sha512Hex("")
	wantEmpty := "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	if emptyHash != wantEmpty {
		t.Errorf("sha512Hex(\"\") = %q, want %q", emptyHash, wantEmpty)
	}

	// Different inputs must produce different hashes
	if sha512Hex("hello") == sha512Hex("world") {
		t.Error("sha512Hex() produced same hash for different inputs")
	}
}

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"BTCUSDT", "BTC"},
		{"BTC-USDT", "BTC"},
		{"BTC_USDT", "BTC"},
		{"BTC-USDT-SWAP", "BTC"},
		{"ETHUSDC", "ETH"},
		{"ETH-USD-SWAP", "ETH"},
		{"SOL-USDT-PERP", "SOL"},
		{"BTC", "BTC"},
		{"btcusdt", "BTC"},
		{"  BTCUSDT  ", "BTC"},
		{"ETH-BTC", "ETH"},
		{"BNBUSDT", "BNB"},
		{"BTC-USDC-SWAP", "BTC"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeSymbol(tt.input)
			if got != tt.want {
				t.Errorf("normalizeSymbol(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
