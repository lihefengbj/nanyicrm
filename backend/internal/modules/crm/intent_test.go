package crm

import (
	"testing"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
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

func TestSanitizeIntentText(t *testing.T) {
	input := "联系人 13812345678，邮箱 sales@example.com，证件 11010519900101123X"
	got := sanitizeIntentText(input, 200)
	want := "联系人 [手机号已脱敏]，邮箱 [邮箱已脱敏]，证件 [证件号已脱敏]"
	if got != want {
		t.Fatalf("sanitizeIntentText() = %q, want %q", got, want)
	}
}

func TestSanitizeIntentTextTruncatesByRune(t *testing.T) {
	if got := sanitizeIntentText("客户需要尽快安排产品演示", 4); got != "客户需要…" {
		t.Fatalf("sanitizeIntentText() = %q", got)
	}
}

func TestEffectiveIntentLevel(t *testing.T) {
	intent := &model.CrmCustomerIntent{
		IntentLevel:       "medium",
		ManualOverride:    true,
		ManualIntentLevel: "high",
	}
	if got := effectiveIntentLevel(intent); got != "high" {
		t.Fatalf("effectiveIntentLevel() = %q, want high", got)
	}
	intent.ManualIntentLevel = ""
	if got := effectiveIntentLevel(intent); got != "medium" {
		t.Fatalf("effectiveIntentLevel() = %q, want medium", got)
	}
}

func TestIsCurrentIntentAnalysis(t *testing.T) {
	analyzedAt := time.Now()
	intent := &model.CrmCustomerIntent{AnalysisID: 12, AnalyzedAt: analyzedAt}
	if !isCurrentIntentAnalysis(intent, &model.CrmCustomerIntentAnalysis{
		Base:       model.Base{ID: 12},
		Status:     intentStatusSuccess,
		AnalyzedAt: &analyzedAt,
	}) {
		t.Fatal("analysis linked by ID should be current")
	}
	if isCurrentIntentAnalysis(intent, &model.CrmCustomerIntentAnalysis{
		Base:       model.Base{ID: 11},
		Status:     intentStatusSuccess,
		AnalyzedAt: &analyzedAt,
	}) {
		t.Fatal("older analysis should not be current")
	}
}
