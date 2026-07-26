package exchange

// InitialMargin calculates the initial margin requirement:
// notionalEntry / leverage. Returns 0 if inputs are invalid.
func InitialMargin(notionalEntry float64, leverage int) float64 {
	if leverage <= 0 || notionalEntry <= 0 {
		return 0
	}
	return notionalEntry / float64(leverage)
}
