package exchange

import "strconv"

// parseFloat конвертує рядок у float64, повертає 0 при помилці.
func parseFloat(s string) float64 {
	if s == "" || s == "0" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseInt64 конвертує рядок у int64, повертає 0 при помилці.
func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
