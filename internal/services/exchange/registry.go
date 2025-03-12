package exchange

// Compile-time перевірка що всі адаптери реалізують інтерфейс Exchange.
var (
	_ Exchange = (*Binance)(nil)
	_ Exchange = (*OKX)(nil)
	_ Exchange = (*Bybit)(nil)
)

// Registry повертає адаптер за назвою біржі (lowercase).
func Registry() map[string]Exchange {
	return map[string]Exchange{
		"binance": NewBinance(),
		"okx":     NewOKX(),
		"bybit":   NewBybit(),
	}
}
