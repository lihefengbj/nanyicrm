package crm

import (
	"testing"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
)

func TestValidateIntentResult(t *testing.T) {
	score := 80
	confidence := 0.9
	tests := []struct {
		name    string
		result  *ai.IntentResult
		wantErr bool
	}{
		{
			name: "valid",
			result: &ai.IntentResult{
				IntentLevel: "high",
				IntentScore: &score,
				Confidence:  &confidence,
			},
		},
		{
			name:    "nil result",
			result:  nil,
			wantErr: true,
		},
		{
			name: "invalid level",
			result: &ai.IntentResult{
				IntentLevel: "hot",
				IntentScore: &score,
			},
			wantErr: true,
		},
		{
			name: "score out of range",
			result: &ai.IntentResult{
				IntentLevel: "high",
				IntentScore: func() *int { value := 101; return &value }(),
			},
			wantErr: true,
		},
		{
			name: "unknown may omit score",
			result: &ai.IntentResult{
				IntentLevel: "unknown",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateIntentResult(tt.result); (err != nil) != tt.wantErr {
				t.Fatalf("validateIntentResult() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
