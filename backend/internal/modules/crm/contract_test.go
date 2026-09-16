package crm

import "testing"

func TestValidContractTransition(t *testing.T) {
	tests := []struct {
		from int8
		to   int8
		want bool
	}{
		{1, 1, true},
		{1, 2, true},
		{1, 3, false},
		{1, 4, true},
		{2, 3, true},
		{2, 4, true},
		{3, 2, false},
		{4, 1, false},
	}
	for _, tt := range tests {
		if got := validContractTransition(tt.from, tt.to); got != tt.want {
			t.Fatalf("transition %d -> %d = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
