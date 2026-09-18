package crm

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const intentQueueKey = "nanyicrm:crm:customer-intent"

type IntentTask struct {
	TenantID      uint64 `json:"tenantId"`
	CustomerID    uint64 `json:"customerId"`
	TriggerUserID uint64 `json:"triggerUserId"`
	Attempts      int    `json:"attempts"`
}

type IntentEnqueuer func(ctx context.Context, tenantID, customerID, triggerUserID uint64) error

type IntentQueue struct {
	rdb     *redis.Client
	handler *IntentHandler
}

func NewIntentQueue(rdb *redis.Client, handler *IntentHandler) *IntentQueue {
	return &IntentQueue{rdb: rdb, handler: handler}
}

func (q *IntentQueue) Enqueue(ctx context.Context, tenantID, customerID, triggerUserID uint64) error {
	if q == nil || q.rdb == nil {
		return nil
	}
	task, err := json.Marshal(IntentTask{
		TenantID:      tenantID,
		CustomerID:    customerID,
		TriggerUserID: triggerUserID,
	})
	if err != nil {
		return err
	}
	return q.rdb.RPush(ctx, intentQueueKey, task).Err()
}

// Run consumes queued analyses until ctx is cancelled. It is deliberately
// independent of HTTP request lifetimes so a saved follow-up remains saved
// even when an AI request is slow or temporarily unavailable.
func (q *IntentQueue) Run(ctx context.Context) {
	if q == nil || q.rdb == nil || q.handler == nil {
		return
	}
	for {
		items, err := q.rdb.BLPop(ctx, 5*time.Second, intentQueueKey).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err != redis.Nil {
				log.Printf("intent queue pop: %v", err)
			}
			continue
		}
		if len(items) < 2 {
			continue
		}
		var task IntentTask
		if err := json.Unmarshal([]byte(items[1]), &task); err != nil {
			log.Printf("intent queue decode: %v", err)
			continue
		}

		taskCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		err = q.handler.AnalyzeCustomerByID(taskCtx, task.TenantID, task.CustomerID, task.TriggerUserID)
		cancel()
		if err == nil || task.Attempts >= 2 {
			if err != nil {
				log.Printf("intent analysis failed tenant=%d customer=%d: %v", task.TenantID, task.CustomerID, err)
			}
			continue
		}
		task.Attempts++
		payload, marshalErr := json.Marshal(task)
		if marshalErr != nil {
			log.Printf("intent queue retry encode: %v", marshalErr)
			continue
		}
		if pushErr := q.rdb.RPush(ctx, intentQueueKey, payload).Err(); pushErr != nil {
			log.Printf("intent queue retry push: %v", pushErr)
		}
	}
}
