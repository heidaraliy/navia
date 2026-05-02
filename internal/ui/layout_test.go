package ui

import "testing"

func TestSplitUsesCompactAndWideRatios(t *testing.T) {
	tests := []struct {
		width     int
		wantLeft  int
		wantRight int
	}{
		{60, 30, 30},
		{100, 45, 55},
		{40, 16, 24},
		{30, 6, 24},
	}
	for _, tt := range tests {
		left, right := Split(tt.width)
		if left != tt.wantLeft || right != tt.wantRight {
			t.Fatalf("Split(%d) = %d,%d want %d,%d", tt.width, left, right, tt.wantLeft, tt.wantRight)
		}
	}
}
