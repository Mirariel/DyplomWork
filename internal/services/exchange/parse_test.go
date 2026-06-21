package exchange

import "testing"

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"", 0},
		{"0", 0},
		{"1.5", 1.5},
		{"67420.50000000", 67420.5},
		{"-95.123", -95.123},
		{"not_a_number", 0},
		{"1e5", 100000},
		{"0.00000001", 0.00000001},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFloat(tt.input)
			if got != tt.want {
				t.Errorf("parseFloat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"", 0},
		{"0", 0},
		{"42", 42},
		{"-1", -1},
		{"9223372036854775807", 9223372036854775807}, // math.MaxInt64
		{"not_a_number", 0},
		{"1.5", 0}, // strconv.ParseInt cannot parse floats
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInt64(tt.input)
			if got != tt.want {
				t.Errorf("parseInt64(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
