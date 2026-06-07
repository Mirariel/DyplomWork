package exchange

// Kraken V0 REST API adapter.
//
// Auth: HMAC-SHA512 з base64-декодованим секретом (унікальна схема Kraken).
// Підпис: Base64(HMAC-SHA512(path + SHA256(nonce + postdata), Base64Decode(secret)))
//
// Документація: https://docs.kraken.com/rest/

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const krakenBaseURL = "https://api.kraken.com"

type Kraken struct{}

func NewKraken() *Kraken { return &Kraken{} }

func (k *Kraken) Name() string { return "kraken" }

// krakenSign генерує підпис для приватного ендпоінту Kraken.
func krakenSign(path, nonce, postData, apiSecret string) (string, error) {
	// SHA256(nonce + postData)
	sha256sum := sha256.Sum256([]byte(nonce + postData))

	// message = path_bytes + sha256sum_bytes
	message := append([]byte(path), sha256sum[:]...)

	// HMAC-SHA512 з base64-декодованим секретом
	secretBytes, err := base64.StdEncoding.DecodeString(apiSecret)
	if err != nil {
		return "", fmt.Errorf("kraken: decode secret: %w", err)
	}
	mac := hmac.New(sha512.New, secretBytes)
	mac.Write(message)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (k *Kraken) privatePost(path string, params url.Values, creds Credentials, dest interface{}) error {
	nonce := strconv.FormatInt(time.Now().UnixMicro(), 10)
	params.Set("nonce", nonce)
	postData := params.Encode()

	sign, err := krakenSign(path, nonce, postData, creds.APISecret)
	if err != nil {
		return err
	}
	headers := map[string]string{
		"API-Key":  creds.APIKey,
		"API-Sign": sign,
	}
	return postForm(krakenBaseURL+path, params, headers, dest)
}

// --- Balances ---

func (k *Kraken) GetBalances(creds Credentials) ([]Balance, error) {
	var resp struct {
		Error  []string          `json:"error"`
		Result map[string]string `json:"result"`
	}
	if err := k.privatePost("/0/private/Balance", url.Values{}, creds, &resp); err != nil {
		return nil, fmt.Errorf("kraken balances: %w", err)
	}
	if len(resp.Error) > 0 {
		return nil, fmt.Errorf("kraken balances API: %s", strings.Join(resp.Error, ", "))
	}
	var result []Balance
	for asset, balStr := range resp.Result {
		qty := parseFloat(balStr)
		if qty > 1e-8 {
			result = append(result, Balance{
				Symbol:   krakenNormalizeAsset(asset),
				Quantity: qty,
			})
		}
	}
	return result, nil
}

// --- Open Positions (margin) ---

func (k *Kraken) GetOpenPositions(creds Credentials) ([]Position, error) {
	var resp struct {
		Error  []string `json:"error"`
		Result map[string]struct {
			Pair     string `json:"pair"`
			Type     string `json:"type"` // "buy" | "sell"
			Vol      string `json:"vol"`
			Cost     string `json:"cost"`
			Value    string `json:"value"`
			Net      string `json:"net"`
			Margin   string `json:"margin"`
		} `json:"result"`
	}
	if err := k.privatePost("/0/private/OpenPositions", url.Values{}, creds, &resp); err != nil {
		return nil, nil // немає маржинального акаунту
	}
	if len(resp.Error) > 0 {
		return nil, nil
	}
	var result []Position
	for _, p := range resp.Result {
		qty := parseFloat(p.Vol)
		if qty == 0 {
			continue
		}
		side := "LONG"
		if p.Type == "sell" {
			side = "SHORT"
		}
		entryPrice := 0.0
		if qty > 0 {
			entryPrice = parseFloat(p.Cost) / qty
		}
		result = append(result, Position{
			Symbol:     krakenNormalizeSymbol(p.Pair),
			Side:       side,
			Quantity:   qty,
			EntryPrice: entryPrice,
			MarkPrice:  parseFloat(p.Value) / qty,
			Leverage:   1,
			PnL:        parseFloat(p.Net),
		})
	}
	return result, nil
}

// --- Closed Trades ---

func (k *Kraken) GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error) {
	params := url.Values{
		"start": {strconv.FormatInt(startMs/1000, 10)},
		"end":   {strconv.FormatInt(endMs/1000, 10)},
	}
	var resp struct {
		Error  []string `json:"error"`
		Result struct {
			Trades map[string]struct {
				Pair      string  `json:"pair"`
				Time      float64 `json:"time"`
				Type      string  `json:"type"`
				Price     string  `json:"price"`
				Vol       string  `json:"vol"`
				Cost      string  `json:"cost"`
				Fee       string  `json:"fee"`
				Net       string  `json:"net"`
			} `json:"trades"`
		} `json:"result"`
	}
	if err := k.privatePost("/0/private/TradesHistory", params, creds, &resp); err != nil {
		return nil, fmt.Errorf("kraken history: %w", err)
	}
	if len(resp.Error) > 0 {
		return nil, nil
	}
	var result []ClosedTrade
	for _, t := range resp.Result.Trades {
		side := "LONG"
		if t.Type == "sell" {
			side = "SHORT"
		}
		result = append(result, ClosedTrade{
			Symbol:     krakenNormalizeSymbol(t.Pair),
			Side:       side,
			Quantity:   parseFloat(t.Vol),
			EntryPrice: parseFloat(t.Price),
			ClosePrice: parseFloat(t.Price),
			PnL:        parseFloat(t.Net) - parseFloat(t.Fee),
			ClosedAt:   int64(t.Time * 1000),
		})
	}
	return result, nil
}

// --- Prices ---
// Kraken потребує явного переліку пар — повертаємо порожній map.
// Binance/OKX/Bybit/Gate покривають всі потрібні ціни.

func (k *Kraken) GetPrices(_ []string) (map[string]float64, error) {
	return make(map[string]float64), nil
}

// --- Helpers ---

// krakenNormalizeAsset нормалізує Kraken asset name до стандартного тікера.
// "XXBT" → "BTC", "XETH" → "ETH", "ZUSD" → "USD", "USDT" → "USDT"
func krakenNormalizeAsset(asset string) string {
	fixed := map[string]string{
		"XXBT": "BTC", "XBT": "BTC",
		"XETH": "ETH", "XLTC": "LTC", "XXRP": "XRP",
		"XXLM": "XLM", "XZEC": "ZEC", "XXMR": "XMR",
		"ZUSD": "USD", "ZEUR": "EUR", "ZGBP": "GBP", "ZCAD": "CAD",
	}
	s := strings.ToUpper(strings.TrimSpace(asset))
	if r, ok := fixed[s]; ok {
		return r
	}
	// Загальне правило: X/Z-prefix для довжини > 3
	if len(s) > 3 && (s[0] == 'X' || s[0] == 'Z') {
		return s[1:]
	}
	return s
}

// krakenNormalizeSymbol витягує базовий символ з пари (напр. "XXBTZUSD" → "BTC")
func krakenNormalizeSymbol(pair string) string {
	quotes := []string{"ZUSD", "ZEUR", "USDT", "USDC", "USD", "EUR", "GBP", "XXBT", "XBT", "XETH", "ETH"}
	p := strings.ToUpper(pair)
	for _, q := range quotes {
		if strings.HasSuffix(p, q) {
			return krakenNormalizeAsset(p[:len(p)-len(q)])
		}
	}
	return krakenNormalizeAsset(p)
}
