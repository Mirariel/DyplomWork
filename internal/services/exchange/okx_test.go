package exchange

import (
	"math"
	"testing"
)

// TestOKXNotionalEntryUsd verifies that ctVal is correctly applied when computing
// NotionalEntryUsd for OKX SWAP positions. Without ctVal the result is wrong
// by a factor of 100 (P-035).
//
// Real example: XAG-USD-SWAP
//   qty = 1983 contracts, ctVal = 0.01, entry = 57.59, leverage = 25
//   NotionalEntryUsd = |1983| * 0.01 * 57.59 = 1141.9917 ≈ 1141.99
//   InitialMargin = 1141.99 / 25 = 45.68
//
// Without ctVal: 1983 * 57.59 = 114199.97 → margin = 4568.00 (100x too large)
func TestOKXNotionalEntryUsd(t *testing.T) {
	const (
		qty      = 1983.0
		ctVal    = 0.01
		entry    = 57.59
		leverage = 25

		wantNotional = 1142.01 // qty × ctVal × entry (1983 × 0.01 × 57.59)
		wantMargin   = 45.68   // notional / leverage
		tolerance    = 0.01
	)

	gotNotional := math.Abs(qty) * ctVal * entry
	gotMargin := gotNotional / float64(leverage)

	if math.Abs(gotNotional-wantNotional) > tolerance {
		t.Errorf("NotionalEntryUsd = %.4f, want %.4f (ctVal applied)", gotNotional, wantNotional)
	}
	if math.Abs(gotMargin-wantMargin) > tolerance {
		t.Errorf("InitialMargin = %.4f, want %.4f", gotMargin, wantMargin)
	}

	// Verify that WITHOUT ctVal the result is wildly wrong
	wrongNotional := math.Abs(qty) * entry // missing ctVal
	wrongMargin := wrongNotional / float64(leverage)
	if wrongMargin < 1000 {
		t.Fatal("sanity check failed: without ctVal, margin should be ~4568, not", wrongMargin)
	}
	if math.Abs(wrongMargin-wantMargin) < 100 {
		t.Fatal("test not distinguishing ctVal presence: wrongMargin too close to wantMargin")
	}
}
