package crm

import "testing"

func TestFitIntentScoreCalibration(t *testing.T) {
	samples := []intentScoreCalibrationSample{
		{rawScore: 80, targetScore: 90},
		{rawScore: 60, targetScore: 70},
		{rawScore: 40, targetScore: 50},
		{rawScore: 20, targetScore: 30},
		{rawScore: 10, targetScore: 20},
	}
	result := fitIntentScoreCalibration(samples, []int{10, 40, 60, 80})
	if result.Status != intentScoreCalibrationReady {
		t.Fatalf("status = %q, want %q", result.Status, intentScoreCalibrationReady)
	}
	if result.Slope != 1 || result.Intercept != 10 {
		t.Fatalf("mapping = y=%.3fx%+.3f, want y=1.000x+10.000", result.Slope, result.Intercept)
	}
	if result.MeanAbsoluteError != 0 {
		t.Fatalf("mean absolute error = %v, want 0", result.MeanAbsoluteError)
	}
	if result.CalibratedAverageScore != 57.5 {
		t.Fatalf("calibrated average = %v, want 57.5", result.CalibratedAverageScore)
	}
}

func TestFitIntentScoreCalibrationRequiresSafeSamples(t *testing.T) {
	tests := []struct {
		name    string
		samples []intentScoreCalibrationSample
		status  string
	}{
		{
			name: "not enough labels",
			samples: []intentScoreCalibrationSample{
				{rawScore: 20, targetScore: 25},
				{rawScore: 60, targetScore: 65},
			},
			status: intentScoreCalibrationInsufficientData,
		},
		{
			name: "single target level",
			samples: []intentScoreCalibrationSample{
				{rawScore: 10, targetScore: 25},
				{rawScore: 20, targetScore: 25},
				{rawScore: 30, targetScore: 25},
				{rawScore: 40, targetScore: 25},
				{rawScore: 50, targetScore: 25},
			},
			status: intentScoreCalibrationInsufficientRange,
		},
		{
			name: "non increasing mapping",
			samples: []intentScoreCalibrationSample{
				{rawScore: 10, targetScore: 90},
				{rawScore: 20, targetScore: 65},
				{rawScore: 30, targetScore: 25},
				{rawScore: 40, targetScore: 25},
				{rawScore: 50, targetScore: 25},
			},
			status: intentScoreCalibrationInsufficientRange,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fitIntentScoreCalibration(tt.samples, nil)
			if result.Status != tt.status {
				t.Fatalf("status = %q, want %q", result.Status, tt.status)
			}
		})
	}
}

func TestClampIntentScore(t *testing.T) {
	tests := []struct {
		value float64
		want  float64
	}{
		{value: -1, want: 0},
		{value: 42.5, want: 42.5},
		{value: 101, want: 100},
	}
	for _, tt := range tests {
		if got := clampIntentScore(tt.value); got != tt.want {
			t.Fatalf("clampIntentScore(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
