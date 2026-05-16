package main

import (
	"testing"
)

func TestAbs(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want int64
	}{
		{name: "positive number", n: 42, want: 42},
		{name: "negative number", n: -42, want: 42},
		{name: "zero", n: 0, want: 0},
		{name: "large positive", n: 1099511627776, want: 1099511627776},
		{name: "large negative", n: -1099511627776, want: 1099511627776},
		{name: "negative one", n: -1, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := abs(tt.n)
			if got != tt.want {
				t.Errorf("abs(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}
