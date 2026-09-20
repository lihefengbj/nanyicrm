package retention

import (
	"context"
	"testing"
	"time"
)

func TestPurgeIntentDataRejectsInvalidInput(t *testing.T) {
	if _, err := PurgeIntentData(context.Background(), nil, time.Now()); err == nil {
		t.Fatal("PurgeIntentData accepted a nil database")
	}
	if _, err := PurgeIntentData(context.Background(), nil, time.Time{}); err == nil {
		t.Fatal("PurgeIntentData accepted a zero cutoff")
	}
}

func TestDeleteCustomerIntentDataRejectsInvalidInput(t *testing.T) {
	if err := DeleteCustomerIntentData(context.Background(), nil, 1, 1); err == nil {
		t.Fatal("DeleteCustomerIntentData accepted a nil database")
	}
	if err := DeleteCustomerIntentData(context.Background(), nil, 1, 0); err == nil {
		t.Fatal("DeleteCustomerIntentData accepted a zero customer")
	}
}
