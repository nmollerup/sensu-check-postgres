package main

import (
	"testing"
)

func TestEvaluateExpression(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		value   float64
		want    bool
		wantErr bool
	}{
		// Greater than
		{name: "gt true", expr: "value > 5", value: 10, want: true},
		{name: "gt false", expr: "value > 5", value: 3, want: false},
		{name: "gt boundary false", expr: "value > 5", value: 5, want: false},

		// Less than
		{name: "lt true", expr: "value < 10", value: 5, want: true},
		{name: "lt false", expr: "value < 10", value: 15, want: false},
		{name: "lt boundary false", expr: "value < 10", value: 10, want: false},

		// Greater than or equal
		{name: "gte true above", expr: "value >= 5", value: 10, want: true},
		{name: "gte true equal", expr: "value >= 5", value: 5, want: true},
		{name: "gte false", expr: "value >= 5", value: 3, want: false},

		// Less than or equal
		{name: "lte true below", expr: "value <= 10", value: 5, want: true},
		{name: "lte true equal", expr: "value <= 10", value: 10, want: true},
		{name: "lte false", expr: "value <= 10", value: 15, want: false},

		// Equal
		{name: "eq true", expr: "value == 5", value: 5, want: true},
		{name: "eq false", expr: "value == 5", value: 6, want: false},

		// Not equal
		{name: "ne true", expr: "value != 5", value: 6, want: true},
		{name: "ne false", expr: "value != 5", value: 5, want: false},

		// Floating point thresholds
		{name: "float threshold", expr: "value > 3.14", value: 4.0, want: true},
		{name: "float threshold false", expr: "value > 3.14", value: 2.0, want: false},

		// Negative thresholds
		{name: "negative threshold gt", expr: "value > -5", value: 0, want: true},
		{name: "negative threshold lt", expr: "value < -5", value: -10, want: true},

		// Zero value
		{name: "zero value gt", expr: "value > 0", value: 0, want: false},
		{name: "zero value eq", expr: "value == 0", value: 0, want: true},

		// Whitespace handling
		{name: "extra whitespace", expr: "  value  >  5  ", value: 10, want: true},

		// Error cases
		{name: "invalid expression no operator", expr: "value 5", value: 10, wantErr: true},
		{name: "invalid threshold not a number", expr: "value > abc", value: 10, wantErr: true},
		{name: "missing value keyword", expr: "x > 5", value: 10, wantErr: true},
		{name: "empty expression", expr: "", value: 10, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateExpression(tt.expr, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("evaluateExpression(%q, %f) expected error, got nil", tt.expr, tt.value)
				}
				return
			}
			if err != nil {
				t.Errorf("evaluateExpression(%q, %f) unexpected error: %v", tt.expr, tt.value, err)
				return
			}
			if got != tt.want {
				t.Errorf("evaluateExpression(%q, %f) = %v, want %v", tt.expr, tt.value, got, tt.want)
			}
		})
	}
}
