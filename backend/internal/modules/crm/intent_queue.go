package crm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	intentStreamKey     = "nanyicrm:crm:customer-intent:stream"
	intentGroupName     = "nanyicrm-intent-workers"
	intentPendingPrefix = "nanyicrm:crm:customer-intent:pending:"
)

var ErrIntentTaskAlreadyQueued = errors.New("customer intent task already queued")

type IntentTask struct {
	TenantID      uint64 `json:"tenantId"`
	CustomerID    uint64 `json:"customerId"`
	TriggerUserID uint64 `json:"triggerUserId"`
	TaskID        uint64 `json:"taskId,omitempty"`
	ItemID        uint64 `json:"itemId,omitempty"`
	Attempts      int    `json:"attempts"`
	DedupKey      string `json:"dedupKey,omitempty"`
}

type IntentEnqueuer func(ctx context.Context, tenantID, customerID, triggerUserID uint64) error

type IntentQueue struct {
	rdb      *redis.Client
	handler  *IntentHandler
	consumer string
}

func NewIntentQueue(rdb *redis.Client, handler *IntentHandler) *IntentQueue {
	hostname, _ := os.Hostname()
	consumer := fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())
	return &IntentQueue{rdb: rdb, handler: handler, consumer: consumer}
}

func (q *IntentQueue) Enqueue(ctx context.Context, tenantID, customerID, triggerUserID uint64) error {
	if q == nil || q.rdb == nil {
		return nil
	}
	dedupKey := fmt.Sprintf("%s%d:%d", intentPendingPrefix, tenantID, customerID)
	created, err := q.rdb.SetNX(ctx, dedupKey, "1", 10*time.Minute).Result()
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	task := IntentTask{
		TenantID:      tenantID,
		CustomerID:    customerID,
		TriggerUserID: triggerUserID,
		DedupKey:      dedupKey,
	}
	if err := q.enqueueTask(ctx, task); err != nil {
		_ = q.rdb.Del(ctx, dedupKey).Err()
		return err
	}
	return nil
}

func (q *IntentQueue) EnqueueTask(ctx context.Context, task IntentTask) error {
	if q == nil || q.rdb == nil {
		return nil
	}
	acquired := false
	if task.DedupKey == "" {
		task.DedupKey = fmt.Sprintf("%s%d:%d", intentPendingPrefix, task.TenantID, task.CustomerID)
		created, err := q.rdb.SetNX(ctx, task.DedupKey, "1", 10*time.Minute).Result()
		if err != nil {
			return err
		}
		if !created {
			return ErrIntentTaskAlreadyQueued
		}
		acquired = true
	}
	if err := q.enqueueTask(ctx, task); err != nil {
		if acquired {
			_ = q.rdb.Del(ctx, task.DedupKey).Err()
		}
		return err
	}
	return nil
}

func (q *IntentQueue) enqueueTask(ctx context.Context, task IntentTask) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: intentStreamKey,
		Values: map[string]interface{}{"payload": string(payload)},
	}).Err()
}

func (q *IntentQueue) ensureGroup(ctx context.Context) error {
	err := q.rdb.XGroupCreateMkStream(ctx, intentStreamKey, intentGroupName, "0-0").Err()
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP") {
		return err
	}
	return nil
}

// Run consumes Redis Stream messages with explicit acknowledgement. Messages
// remain pending until analysis finishes, so a process restart can reclaim
// work that was taken but never acknowledged.
func (q *IntentQueue) Run(ctx context.Context) {
	if q == nil || q.rdb == nil || q.handler == nil {
		return
	}
	if err := q.ensureGroup(ctx); err != nil {
		log.Printf("intent queue group: %v", err)
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		q.recoverPending(ctx)
		streams, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    intentGroupName,
			Consumer: q.consumer,
			Streams:  []string{intentStreamKey, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err != redis.Nil {
				log.Printf("intent queue read: %v", err)
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				q.processMessage(ctx, message)
			}
		}
	}
}

func (q *IntentQueue) recoverPending(ctx context.Context) {
	messages, _, err := q.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   intentStreamKey,
		Group:    intentGroupName,
		Consumer: q.consumer,
		MinIdle:  2 * time.Minute,
		Start:    "0-0",
		Count:    20,
	}).Result()
	if err != nil && err != redis.Nil {
		log.Printf("intent queue reclaim: %v", err)
		return
	}
	for _, message := range messages {
		q.processMessage(ctx, message)
	}
}

func (q *IntentQueue) processMessage(ctx context.Context, message redis.XMessage) {
	payload, ok := message.Values["payload"]
	if !ok {
		_ = q.rdb.XAck(ctx, intentStreamKey, intentGroupName, message.ID).Err()
		return
	}
	var raw string
	switch value := payload.(type) {
	case string:
		raw = value
	case []byte:
		raw = string(value)
	default:
		raw = fmt.Sprint(value)
	}
	var task IntentTask
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		log.Printf("intent queue decode: %v", err)
		_ = q.rdb.XAck(ctx, intentStreamKey, intentGroupName, message.ID).Err()
		return
	}

	taskCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	err := q.handler.processQueuedTask(taskCtx, task)
	cancel()
	if err == nil {
		q.finishMessage(ctx, message.ID, task.DedupKey)
		return
	}
	if ctx.Err() != nil {
		return
	}
	if task.Attempts >= 2 {
		log.Printf("intent analysis failed tenant=%d customer=%d: %v", task.TenantID, task.CustomerID, err)
		q.finishMessage(ctx, message.ID, task.DedupKey)
		return
	}
	task.Attempts++
	if retryErr := q.enqueueTask(ctx, task); retryErr != nil {
		log.Printf("intent queue retry: %v", retryErr)
		return
	}
	q.finishMessage(ctx, message.ID, "")
}

func (q *IntentQueue) finishMessage(ctx context.Context, messageID, dedupKey string) {
	if err := q.rdb.XAck(ctx, intentStreamKey, intentGroupName, messageID).Err(); err != nil {
		log.Printf("intent queue ack: %v", err)
		return
	}
	_ = q.rdb.XDel(ctx, intentStreamKey, messageID).Err()
	if dedupKey != "" {
		_ = q.rdb.Del(ctx, dedupKey).Err()
	}
}
