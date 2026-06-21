package ws

import (
	"math"
	"testing"
)

func TestRoundFloat(t *testing.T) {
	tests := []struct {
		input    float64
		decimals int
		want     float64
	}{
		{1.5555, 2, 1.56},
		{1.5545, 2, 1.55},
		{-5.555, 2, -5.56},
		{-5.545, 2, -5.55},
		{0.0, 2, 0.0},
		{100.0, 0, 100.0},
		{1.23456789, 4, 1.2346},
		{-1.23456789, 4, -1.2346},
		{0.005, 2, 0.01},
		{-0.005, 2, -0.01},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := roundFloat(tt.input, tt.decimals)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("roundFloat(%v, %d) = %v, want %v", tt.input, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestFormatLeverage(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "1x"},
		{-1, "1x"},
		{1, "1x"},
		{10, "10x"},
		{125, "125x"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := formatLeverage(tt.input)
			if got != tt.want {
				t.Errorf("formatLeverage(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
