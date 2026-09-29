package crm

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const (
	qualityGateExecutionTimeout = 5 * time.Minute
	// qualityGateStaleAfter exceeds the execution timeout: a run without any
	// progress update for this long is interrupted (server restart or failed
	// final write) and must be surfaced as failed instead of spinning forever.
	qualityGateStaleAfter = 6 * time.Minute

	qualityGateRunQueued  = "queued"
	qualityGateRunRunning = "running"
	qualityGateRunPassed  = "passed"
	qualityGateRunFailed  = "failed"

	qualityGateSampleQueued  = "queued"
	qualityGateSampleRunning = "running"
	qualityGateSamplePassed  = "passed"
	qualityGateSampleFailed  = "failed"
)

type qualityGateSampleProgress struct {
	Name          string     `json:"name"`
	Label         string     `json:"label"`
	ExpectedLevel string     `json:"expectedLevel"`
	Status        string     `json:"status"`
	Error         string     `json:"error"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

type qualityGateRunResponse struct {
	RunID           string                      `json:"runId"`
	ResourceType    string                      `json:"resourceType"`
	ResourceID      uint64                      `json:"resourceId"`
	ResourceName    string                      `json:"resourceName"`
	ResourceVersion string                      `json:"resourceVersion"`
	Status          string                      `json:"status"`
	Total           int                         `json:"total"`
	Completed       int                         `json:"completed"`
	CurrentSample   string                      `json:"currentSample"`
	Samples         []qualityGateSampleProgress `json:"samples"`
	Summary         string                      `json:"summary"`
	Metrics         map[string]interface{}      `json:"metrics"`
	Error           string                      `json:"error"`
	StartedAt       *time.Time                  `json:"startedAt"`
	FinishedAt      *time.Time                  `json:"finishedAt"`
	CreatedAt       time.Time                   `json:"createdAt"`
	UpdatedAt       time.Time                   `json:"updatedAt"`
}

type qualityGateRunManager struct {
	db     *gorm.DB
	mu     sync.Mutex
	serial uint64
}

func newQualityGateRunManager(db *gorm.DB) *qualityGateRunManager {
	return &qualityGateRunManager{db: db}
}

func (m *qualityGateRunManager) start(resourceType string, resourceID uint64, resourceName, resourceVersion string) (*model.SysAIQualityGateRun, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var existing model.SysAIQualityGateRun
	err := m.db.
		Where("resource_type = ? AND resource_id = ? AND status IN ?", resourceType, resourceID, []string{qualityGateRunQueued, qualityGateRunRunning}).
		Order("id DESC").
		First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	now := time.Now()
	samples := make([]qualityGateSampleProgress, 0, len(intentQualitySamples()))
	for _, sample := range intentQualitySamples() {
		samples = append(samples, qualityGateSampleProgress{
			Name:          sample.Name,
			Label:         qualitySampleLabel(sample.Name),
			ExpectedLevel: sample.ExpectedLevel,
			Status:        qualityGateSampleQueued,
		})
	}
	rawSamples, err := json.Marshal(samples)
	if err != nil {
		return nil, false, err
	}

	run := &model.SysAIQualityGateRun{
		RunID:           fmt.Sprintf("qgr_%d_%d", now.UnixNano(), atomic.AddUint64(&m.serial, 1)),
		ResourceType:    resourceType,
		ResourceID:      resourceID,
		ResourceName:    resourceName,
		ResourceVersion: resourceVersion,
		Status:          qualityGateRunQueued,
		Total:           len(samples),
		Samples:         string(rawSamples),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := m.db.Create(run).Error; err != nil {
		return nil, false, err
	}
	return run, true, nil
}

func (m *qualityGateRunManager) get(runID string) (*model.SysAIQualityGateRun, error) {
	var run model.SysAIQualityGateRun
	if err := m.db.Where("run_id = ?", runID).First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (m *qualityGateRunManager) update(runID string, fn func(*model.SysAIQualityGateRun)) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = m.updateOnce(runID, fn)
		if err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	log.Printf("quality gate run %s update failed after retries: %v", runID, err)
	return err
}

func (m *qualityGateRunManager) updateOnce(runID string, fn func(*model.SysAIQualityGateRun)) error {
	run, err := m.get(runID)
	if err != nil {
		return err
	}
	fn(run)
	run.UpdatedAt = time.Now()
	return m.db.Save(run).Error
}

// failStaleRun converts a run that stopped reporting progress into a failed
// run so the UI never waits forever on a dead executor.
func (m *qualityGateRunManager) failStaleRun(run *model.SysAIQualityGateRun) {
	if run.Status != qualityGateRunQueued && run.Status != qualityGateRunRunning {
		return
	}
	if time.Since(run.UpdatedAt) <= qualityGateStaleAfter {
		return
	}
	if err := m.update(run.RunID, func(current *model.SysAIQualityGateRun) {
		if current.Status != qualityGateRunQueued && current.Status != qualityGateRunRunning {
			return
		}
		now := time.Now()
		current.Status = qualityGateRunFailed
		current.Error = "执行中断或超时未更新进度，请重新执行质量门禁"
		if current.Summary == "" {
			current.Summary = current.Error
		}
		current.CurrentSample = ""
		current.FinishedAt = &now
	}); err == nil {
		run.Status = qualityGateRunFailed
		run.Error = "执行中断或超时未更新进度，请重新执行质量门禁"
		run.Summary = run.Error
		run.CurrentSample = ""
	}
}

func (m *qualityGateRunManager) reportSample(runID, sampleName, status, errorMessage string) error {
	return m.update(runID, func(run *model.SysAIQualityGateRun) {
		var samples []qualityGateSampleProgress
		_ = json.Unmarshal([]byte(run.Samples), &samples)
		now := time.Now()
		for i := range samples {
			if samples[i].Name != sampleName {
				continue
			}
			if status == qualityGateSampleRunning {
				samples[i].StartedAt = &now
				run.CurrentSample = sampleName
			} else {
				samples[i].FinishedAt = &now
				samples[i].Error = errorMessage
				if status == qualityGateSamplePassed || status == qualityGateSampleFailed {
					run.Completed++
				}
				if run.CurrentSample == sampleName {
					run.CurrentSample = ""
				}
			}
			samples[i].Status = status
			break
		}
		rawSamples, _ := json.Marshal(samples)
		run.Samples = string(rawSamples)
	})
}

func (m *qualityGateRunManager) finish(runID string, passed bool, summary, metrics, errorMessage string) error {
	return m.update(runID, func(run *model.SysAIQualityGateRun) {
		now := time.Now()
		run.Status = qualityGateRunFailed
		if passed {
			run.Status = qualityGateRunPassed
		}
		run.Summary = summary
		run.Metrics = metrics
		run.Error = errorMessage
		run.CurrentSample = ""
		run.FinishedAt = &now
	})
}

func qualityGateRunView(run *model.SysAIQualityGateRun) qualityGateRunResponse {
	response := qualityGateRunResponse{
		RunID:           run.RunID,
		ResourceType:    run.ResourceType,
		ResourceID:      run.ResourceID,
		ResourceName:    run.ResourceName,
		ResourceVersion: run.ResourceVersion,
		Status:          run.Status,
		Total:           run.Total,
		Completed:       run.Completed,
		CurrentSample:   run.CurrentSample,
		Summary:         run.Summary,
		Error:           run.Error,
		StartedAt:       run.StartedAt,
		FinishedAt:      run.FinishedAt,
		CreatedAt:       run.CreatedAt,
		UpdatedAt:       run.UpdatedAt,
		Metrics:         map[string]interface{}{},
	}
	_ = json.Unmarshal([]byte(run.Samples), &response.Samples)
	if stringsTrimmed := run.Metrics; stringsTrimmed != "" {
		_ = json.Unmarshal([]byte(stringsTrimmed), &response.Metrics)
	}
	return response
}

func qualitySampleLabel(name string) string {
	switch name {
	case "high-intent":
		return "高意向样本"
	case "medium-intent":
		return "中意向样本"
	case "low-intent":
		return "低意向样本"
	case "unknown-intent":
		return "信息不足样本"
	default:
		return name
	}
}

func qualityGateRunNotFound(c *gin.Context, err error) {
	if err == nil || err == gorm.ErrRecordNotFound {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	common.Fail(c, common.CodeDBError)
}
