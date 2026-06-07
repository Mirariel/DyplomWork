package exchange

// Compile-time перевірка що всі адаптери реалізують інтерфейс Exchange.
var (
	_ Exchange = (*Binance)(nil)
	_ Exchange = (*OKX)(nil)
	_ Exchange = (*Bybit)(nil)
	_ Exchange = (*Gate)(nil)
	_ Exchange = (*Kraken)(nil)
	_ Exchange = (*KuCoin)(nil)
)

// Registry повертає всі зареєстровані адаптери бірж.
// Ключ — lowercase назва біржі (те саме що зберігається в external_api_credentials.exchange).
// Щоб додати нову біржу: реалізувати Exchange interface і додати тут один рядок.
func Registry() map[string]Exchange {
	return map[string]Exchange{
		"binance": NewBinance(),
		"okx":     NewOKX(),
		"bybit":   NewBybit(),
		"gate":    NewGate(),
		"kraken":  NewKraken(),
		"kucoin":  NewKuCoin(),
	}
}
