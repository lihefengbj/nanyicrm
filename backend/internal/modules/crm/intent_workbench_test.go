package crm

import (
	"testing"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

func TestEstimateIntentCost(t *testing.T) {
	pricing := config.PricingConfig{
		InputPerMillionTokens:  2,
		OutputPerMillionTokens: 8,
	}
	input, output := estimateIntentCostParts(pricing, 500_000, 250_000)
	if input != 1 || output != 2 {
		t.Fatalf("estimateIntentCostParts() = (%v, %v), want (1, 2)", input, output)
	}
	if got := estimateIntentCost(pricing, 500_000, 250_000); got != 3 {
		t.Fatalf("estimateIntentCost() = %v, want 3", got)
	}
}

func TestEstimateIntentCostWithoutPricing(t *testing.T) {
	if got := estimateIntentCost(config.PricingConfig{}, 1000, 1000); got != 0 {
		t.Fatalf("estimateIntentCost() = %v, want 0", got)
	}
}

func TestEstimateIntentCostWithDeepSeekTieredPricing(t *testing.T) {
	pricing := config.PricingConfig{
		Period:                       config.PricingPeriodIdle,
		InputCacheHitIdlePerMillion:  0.02,
		InputCacheHitPeakPerMillion:  0.04,
		InputCacheMissIdlePerMillion: 1,
		InputCacheMissPeakPerMillion: 2,
		OutputIdlePerMillion:         4,
		OutputPeakPerMillion:         8,
	}
	hit, miss, output := estimateIntentCostByCache(pricing, 500_000, 250_000, 250_000)
	if hit != 0.01 || miss != 0.25 || output != 1 {
		t.Fatalf("estimateIntentCostByCache() = (%v, %v, %v), want (0.01, 0.25, 1)", hit, miss, output)
	}

	pricing.Period = config.PricingPeriodPeak
	hit, miss, output = estimateIntentCostByCache(pricing, 500_000, 250_000, 250_000)
	if hit != 0.02 || miss != 0.5 || output != 2 {
		t.Fatalf("peak estimateIntentCostByCache() = (%v, %v, %v), want (0.02, 0.5, 2)", hit, miss, output)
	}
}

func TestNormalizeInputCacheTokensLegacyRows(t *testing.T) {
	hit, miss := normalizeInputCacheTokens(100, 0, 0)
	if hit != 0 || miss != 100 {
		t.Fatalf("normalizeInputCacheTokens() = (%d, %d), want (0, 100)", hit, miss)
	}
}

func TestEffectiveBillingPeriodUsesCreatedAtForLegacyRows(t *testing.T) {
	pricing := config.PricingConfig{
		Mode:     config.PricingModeAuto,
		Timezone: "Asia/Shanghai",
	}
	peakAt := time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC) // Monday 09:00 CST
	idleAt := time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC) // Sunday 09:00 CST

	if got := effectiveBillingPeriod(pricing, "", peakAt); got != config.PricingPeriodPeak {
		t.Fatalf("legacy peak row period = %q, want peak", got)
	}
	if got := effectiveBillingPeriod(pricing, "", idleAt); got != config.PricingPeriodIdle {
		t.Fatalf("legacy idle row period = %q, want idle", got)
	}
	if got := effectiveBillingPeriod(pricing, config.PricingPeriodIdle, peakAt); got != config.PricingPeriodIdle {
		t.Fatalf("stored period = %q, want idle", got)
	}
}
