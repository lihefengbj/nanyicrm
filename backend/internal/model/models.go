package model

import (
	"time"

	"gorm.io/gorm"
)

// Base holds columns shared by every business table.
type Base struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// SysTenant is a SaaS tenant. tenant_id = 0 on other tables means
// "platform-level" data owned by the super admin (who has no tenant).
type SysTenant struct {
	Base
	Code     string     `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name     string     `gorm:"size:64;not null" json:"name"`
	Contact  string     `gorm:"size:64" json:"contact"`
	Phone    string     `gorm:"size:32" json:"phone"`
	ExpireAt *time.Time `json:"expireAt"` // nil = never expires
	Status   int8       `json:"status"`   // 1 enabled, 0 disabled
	Remark   string     `gorm:"size:255" json:"remark"`
	// AI intent quota overrides. nil inherits the platform default from
	// llm.quota in config.yaml; 0 means "no limit".
	AIDailyCalls  *int64 `json:"aiDailyCalls"`
	AIDailyTokens *int64 `json:"aiDailyTokens"`
	AIConcurrency *int64 `json:"aiConcurrency"`
}

func (SysTenant) TableName() string { return "sys_tenant" }

// Usable reports whether tenant users may log in right now.
func (t *SysTenant) Usable() bool {
	if t.Status != 1 {
		return false
	}
	if t.ExpireAt != nil && t.ExpireAt.Before(time.Now()) {
		return false
	}
	return true
}

type SysUser struct {
	Base
	TenantID uint64     `gorm:"index;default:0" json:"tenantId"` // 0 = platform super admin
	Username string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PwdHash  string     `gorm:"size:128;not null" json:"-"`
	Nickname string     `gorm:"size:64" json:"nickname"`
	Email    string     `gorm:"size:128" json:"email"`
	Phone    string     `gorm:"size:32" json:"phone"`
	DeptID   *uint64    `gorm:"index" json:"deptId"`
	Status   int8       `json:"status"` // 1 enabled, 0 disabled
	Remark   string     `gorm:"size:255" json:"remark"`
	Dept     *SysDept   `gorm:"foreignKey:DeptID" json:"dept,omitempty"`
	Tenant   *SysTenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Roles    []SysRole  `gorm:"many2many:sys_user_role;joinForeignKey:UserID;joinReferences:RoleID" json:"roles,omitempty"`
}

func (SysUser) TableName() string { return "sys_user" }

type SysDept struct {
	Base
	TenantID uint64    `gorm:"index;default:0" json:"tenantId"`
	ParentID uint64    `gorm:"index;default:0" json:"parentId"`
	Name     string    `gorm:"size:64;not null" json:"name"`
	Leader   string    `gorm:"size:64" json:"leader"`
	Sort     int       `gorm:"default:0" json:"sort"`
	Status   int8      `json:"status"`
	Children []SysDept `gorm:"-" json:"children,omitempty"`
}

func (SysDept) TableName() string { return "sys_dept" }

// SysRole is global: all tenants share one set of roles, only the platform
// side manages them. Code is globally unique.
type SysRole struct {
	Base
	Name   string    `gorm:"size:64;not null" json:"name"`
	Code   string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Sort   int       `gorm:"default:0" json:"sort"`
	Status int8      `json:"status"`
	Remark string    `gorm:"size:255" json:"remark"`
	Menus  []SysMenu `gorm:"many2many:sys_role_menu;joinForeignKey:RoleID;joinReferences:MenuID" json:"menus,omitempty"`
}

func (SysRole) TableName() string { return "sys_role" }

// SysMenu is a menu, a directory, or a button-level permission.
type SysMenu struct {
	Base
	ParentID  uint64    `gorm:"index;default:0" json:"parentId"`
	Title     string    `gorm:"size:64;not null" json:"title"`
	Type      int8      `gorm:"default:1" json:"type"` // 1 dir, 2 menu, 3 button
	Path      string    `gorm:"size:128" json:"path"`
	Component string    `gorm:"size:128" json:"component"`
	Perms     string    `gorm:"size:128" json:"perms"` // e.g. system:user:list
	Icon      string    `gorm:"size:64" json:"icon"`
	Sort      int       `gorm:"default:0" json:"sort"`
	Visible   int8      `json:"visible"`
	Status    int8      `json:"status"`
	Children  []SysMenu `gorm:"-" json:"children,omitempty"`
}

func (SysMenu) TableName() string { return "sys_menu" }

type SysUserRole struct {
	UserID uint64 `gorm:"primaryKey"`
	RoleID uint64 `gorm:"primaryKey"`
}

func (SysUserRole) TableName() string { return "sys_user_role" }

type SysRoleMenu struct {
	RoleID uint64 `gorm:"primaryKey"`
	MenuID uint64 `gorm:"primaryKey"`
}

func (SysRoleMenu) TableName() string { return "sys_role_menu" }

type SysDict struct {
	Base
	TenantID uint64        `gorm:"uniqueIndex:uk_dict_tenant_type;index;default:0" json:"tenantId"`
	Name     string        `gorm:"size:64;not null" json:"name"`
	Type     string        `gorm:"size:64;uniqueIndex:uk_dict_tenant_type;not null" json:"type"`
	Status   int8          `json:"status"`
	Remark   string        `gorm:"size:255" json:"remark"`
	Items    []SysDictItem `gorm:"foreignKey:DictID" json:"items,omitempty"`
}

func (SysDict) TableName() string { return "sys_dict" }

type SysDictItem struct {
	Base
	DictID uint64 `gorm:"index;not null" json:"dictId"`
	Label  string `gorm:"size:64;not null" json:"label"`
	Value  string `gorm:"size:64;not null" json:"value"`
	Sort   int    `gorm:"default:0" json:"sort"`
	Status int8   `json:"status"`
}

func (SysDictItem) TableName() string { return "sys_dict_item" }

type SysOperLog struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	TenantID   uint64    `gorm:"index;default:0" json:"tenantId"`
	UserID     uint64    `gorm:"index" json:"userId"`
	Username   string    `gorm:"size:64" json:"username"`
	Module     string    `gorm:"size:64" json:"module"`
	Action     string    `gorm:"size:64" json:"action"`
	Method     string    `gorm:"size:16" json:"method"`
	Path       string    `gorm:"size:255" json:"path"`
	IP         string    `gorm:"size:64" json:"ip"`
	Status     int       `json:"status"` // business code from response
	ErrorMsg   string    `gorm:"size:512" json:"errorMsg"`
	CostMillis int64     `json:"costMillis"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (SysOperLog) TableName() string { return "sys_oper_log" }

type SysLoginLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TenantID  uint64    `gorm:"index;default:0" json:"tenantId"`
	Username  string    `gorm:"size:64;index" json:"username"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:255" json:"userAgent"`
	Success   bool      `json:"success"`
	Message   string    `gorm:"size:255" json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func (SysLoginLog) TableName() string { return "sys_login_log" }

type SysAIModelConfig struct {
	Base
	Name             string     `gorm:"size:64;not null" json:"name"`
	Provider         string     `gorm:"size:64;not null" json:"provider"`
	BaseURL          string     `gorm:"size:255;not null" json:"baseUrl"`
	Model            string     `gorm:"size:128;not null" json:"model"`
	ConfigVersion    string     `gorm:"size:64;uniqueIndex;not null" json:"configVersion"`
	PromptVersion    string     `gorm:"size:32;not null" json:"promptVersion"`
	ResponseFormat   string     `gorm:"size:32" json:"responseFormat"`
	ThinkingMode     string     `gorm:"size:16" json:"thinkingMode"`
	MaxTokens        int        `json:"maxTokens"`
	Temperature      float32    `json:"temperature"`
	Status           string     `gorm:"size:16;index;not null" json:"status"` // draft, approved, active, canary, retired
	CanaryPercent    int        `json:"canaryPercent"`
	QualityPassed    bool       `json:"qualityPassed"`
	QualitySummary   string     `gorm:"type:text" json:"qualitySummary"`
	QualityMetrics   string     `gorm:"type:text" json:"qualityMetrics"`
	QualityCheckedAt *time.Time `json:"qualityCheckedAt"`
	ActivatedAt      *time.Time `json:"activatedAt"`
}

func (SysAIModelConfig) TableName() string { return "sys_ai_model_config" }

type SysAIModelChange struct {
	ID             uint64    `gorm:"primaryKey" json:"id"`
	ConfigID       uint64    `gorm:"index;not null" json:"configId"`
	FromConfigID   uint64    `json:"fromConfigId"`
	ToConfigID     uint64    `json:"toConfigId"`
	ActorID        uint64    `gorm:"index" json:"actorId"`
	Action         string    `gorm:"size:32;not null" json:"action"`
	Reason         string    `gorm:"size:512" json:"reason"`
	QualitySummary string    `gorm:"type:text" json:"qualitySummary"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (SysAIModelChange) TableName() string { return "sys_ai_model_change" }

type CrmCustomerIntentAnalysisArchive struct {
	ID                 uint64    `gorm:"primaryKey" json:"id"`
	OriginalAnalysisID uint64    `gorm:"uniqueIndex;not null" json:"originalAnalysisId"`
	TenantID           uint64    `gorm:"index;not null" json:"tenantId"`
	CustomerID         uint64    `gorm:"index;not null" json:"customerId"`
	Payload            string    `gorm:"type:longtext;not null" json:"payload"`
	CreatedAt          time.Time `json:"createdAt"`
	ArchivedAt         time.Time `gorm:"index" json:"archivedAt"`
}

func (CrmCustomerIntentAnalysisArchive) TableName() string {
	return "crm_customer_intent_analysis_archive"
}

// CrmCustomer is a customer profile owned by a tenant. OwnerID points to the
// sys_user responsible for it (data permission: privileged users see all,
// tenant users see their tenant's, and can filter down to their own).
type CrmCustomer struct {
	Base
	TenantID uint64   `gorm:"index;default:0" json:"tenantId"`
	Name     string   `gorm:"size:128;not null;index" json:"name"`
	Phone    string   `gorm:"size:32" json:"phone"`
	Source   string   `gorm:"size:32" json:"source"`   // e.g. 广告/转介绍/自拓
	Industry string   `gorm:"size:64" json:"industry"` // 行业
	Level    string   `gorm:"size:8" json:"level"`     // A/B/C
	Status   int8     `gorm:"default:1" json:"status"` // 1 跟进中, 2 已成交, 3 已流失
	OwnerID  *uint64  `gorm:"index" json:"ownerId"`
	Address  string   `gorm:"size:255" json:"address"`
	Remark   string   `gorm:"size:255" json:"remark"`
	Owner    *SysUser `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

func (CrmCustomer) TableName() string { return "crm_customer" }

type CrmContact struct {
	Base
	TenantID   uint64       `gorm:"index;default:0" json:"tenantId"`
	CustomerID uint64       `gorm:"index;not null" json:"customerId"`
	Name       string       `gorm:"size:64;not null" json:"name"`
	Phone      string       `gorm:"size:32" json:"phone"`
	Email      string       `gorm:"size:128" json:"email"`
	Position   string       `gorm:"size:64" json:"position"`
	IsPrimary  int8         `gorm:"default:0" json:"isPrimary"` // 1 = 首要联系人
	Remark     string       `gorm:"size:255" json:"remark"`
	Customer   *CrmCustomer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
}

func (CrmContact) TableName() string { return "crm_contact" }

// CrmFollowUp is one follow-up activity (call/visit/meeting) on a customer.
type CrmFollowUp struct {
	Base
	TenantID   uint64       `gorm:"index;default:0" json:"tenantId"`
	CustomerID uint64       `gorm:"index;not null" json:"customerId"`
	ContactID  *uint64      `gorm:"index" json:"contactId"`
	Type       int8         `gorm:"default:1" json:"type"` // 1 电话, 2 拜访, 3 会议, 4 其他
	Content    string       `gorm:"size:1024;not null" json:"content"`
	NextAt     *time.Time   `json:"nextAt"` // 下次跟进时间
	CreatorID  uint64       `gorm:"index" json:"creatorId"`
	Creator    string       `gorm:"size:64" json:"creator"` // username snapshot
	Customer   *CrmCustomer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Contact    *CrmContact  `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
}

func (CrmFollowUp) TableName() string { return "crm_follow_up" }

// CrmOpportunity is a sales opportunity tied to a customer. Stage walks the
// sales pipeline: 1 初步接触, 2 需求确认, 3 方案报价, 4 商务谈判, 5 赢单, 6 输单.
type CrmOpportunity struct {
	Base
	TenantID   uint64       `gorm:"index;default:0" json:"tenantId"`
	CustomerID uint64       `gorm:"index;not null" json:"customerId"`
	Name       string       `gorm:"size:128;not null" json:"name"`
	Stage      int8         `gorm:"default:1;index" json:"stage"` // 1-6, see above
	Amount     float64      `gorm:"type:decimal(12,2);default:0" json:"amount"`
	ExpectDate *time.Time   `json:"expectDate"` // 预计成交日期
	OwnerID    *uint64      `gorm:"index" json:"ownerId"`
	Remark     string       `gorm:"size:255" json:"remark"`
	Customer   *CrmCustomer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Owner      *SysUser     `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

func (CrmOpportunity) TableName() string { return "crm_opportunity" }

// CrmContract is a signed (or signing) contract, optionally born from an
// opportunity. Status: 1 草稿, 2 履行中, 3 已完成, 4 已作废.
type CrmContract struct {
	Base
	TenantID      uint64          `gorm:"index;default:0" json:"tenantId"`
	Code          string          `gorm:"size:64;index" json:"code"`
	Name          string          `gorm:"size:128;not null" json:"name"`
	CustomerID    uint64          `gorm:"index;not null" json:"customerId"`
	OpportunityID *uint64         `gorm:"index" json:"opportunityId"`
	Amount        float64         `gorm:"type:decimal(12,2);default:0" json:"amount"`
	SignDate      *time.Time      `json:"signDate"`
	StartDate     *time.Time      `json:"startDate"`
	EndDate       *time.Time      `json:"endDate"`
	Status        int8            `gorm:"default:1" json:"status"`
	OwnerID       *uint64         `gorm:"index" json:"ownerId"`
	Remark        string          `gorm:"size:255" json:"remark"`
	Customer      *CrmCustomer    `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Opportunity   *CrmOpportunity `gorm:"foreignKey:OpportunityID" json:"opportunity,omitempty"`
	Owner         *SysUser        `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

func (CrmContract) TableName() string { return "crm_contract" }

// CrmCustomerIntent stores the latest valid AI assessment for a customer.
// Textual lists are stored as JSON strings to keep the model portable across
// MySQL versions and allow the provider response to evolve independently.
type CrmCustomerIntent struct {
	Base
	TenantID          uint64     `gorm:"uniqueIndex:uk_customer_intent_customer;index;default:0" json:"tenantId"`
	CustomerID        uint64     `gorm:"uniqueIndex:uk_customer_intent_customer;not null" json:"customerId"`
	AnalysisID        uint64     `gorm:"index;not null;default:0" json:"analysisId"`
	IntentLevel       string     `gorm:"size:16;index;not null" json:"intentLevel"`
	IntentScore       *int       `json:"intentScore"`
	Confidence        *float64   `json:"confidence"`
	Summary           string     `gorm:"type:text" json:"summary"`
	Needs             string     `gorm:"type:text" json:"needs"`
	PainPoints        string     `gorm:"type:text" json:"painPoints"`
	Budget            string     `gorm:"size:255" json:"budget"`
	PurchaseTimeline  string     `gorm:"size:255" json:"purchaseTimeline"`
	DecisionRole      string     `gorm:"size:255" json:"decisionRole"`
	Risks             string     `gorm:"type:text" json:"risks"`
	NextAction        string     `gorm:"type:text" json:"nextAction"`
	SuggestedNextAt   *time.Time `json:"suggestedNextAt"`
	AnalyzedAt        time.Time  `json:"analyzedAt"`
	Provider          string     `gorm:"size:64" json:"provider"`
	Model             string     `gorm:"size:128" json:"model"`
	PromptVersion     string     `gorm:"size:32" json:"promptVersion"`
	Status            string     `gorm:"size:16;index;not null" json:"status"`
	ManualOverride    bool       `gorm:"default:false" json:"manualOverride"`
	ManualIntentLevel string     `gorm:"size:16" json:"manualIntentLevel"`
}

func (CrmCustomerIntent) TableName() string { return "crm_customer_intent" }

// CrmCustomerIntentAnalysis keeps every AI analysis attempt for audit and
// troubleshooting. InputSnapshot and ResultSnapshot are JSON documents.
type CrmCustomerIntentAnalysis struct {
	Base
	TenantID             uint64     `gorm:"index;default:0" json:"tenantId"`
	CustomerID           uint64     `gorm:"index;not null" json:"customerId"`
	TriggerUserID        uint64     `gorm:"index" json:"triggerUserId"`
	InputSnapshot        string     `gorm:"type:text" json:"inputSnapshot"`
	ResultSnapshot       string     `gorm:"type:text" json:"resultSnapshot"`
	Status               string     `gorm:"size:16;index;not null" json:"status"`
	ErrorMessage         string     `gorm:"size:1024" json:"errorMessage"`
	Provider             string     `gorm:"size:64" json:"provider"`
	Model                string     `gorm:"size:128" json:"model"`
	ActualModel          string     `gorm:"size:128" json:"actualModel"`
	ModelConfigVersion   string     `gorm:"size:64;index" json:"modelConfigVersion"`
	AdapterVersion       string     `gorm:"size:64" json:"adapterVersion"`
	PromptVersion        string     `gorm:"size:32" json:"promptVersion"`
	InputTokens          int        `json:"inputTokens"`
	InputCacheHitTokens  int        `json:"inputCacheHitTokens"`
	InputCacheMissTokens int        `json:"inputCacheMissTokens"`
	OutputTokens         int        `json:"outputTokens"`
	TotalTokens          int        `json:"totalTokens"`
	BillingPeriod        string     `gorm:"size:16" json:"billingPeriod"`
	ProviderRequestID    string     `gorm:"size:128" json:"providerRequestId"`
	ErrorType            string     `gorm:"size:32;index" json:"errorType"`
	InputHash            string     `gorm:"size:64;index" json:"inputHash"`
	CostMillis           int64      `json:"costMillis"`
	AnalyzedAt           *time.Time `json:"analyzedAt"`
}

func (CrmCustomerIntentAnalysis) TableName() string { return "crm_customer_intent_analysis" }

// CrmCustomerIntentFeedback stores human review without overwriting the
// original model output.
type CrmCustomerIntentFeedback struct {
	Base
	TenantID          uint64 `gorm:"index;default:0" json:"tenantId"`
	CustomerID        uint64 `gorm:"index;not null" json:"customerId"`
	AnalysisID        uint64 `gorm:"index;not null" json:"analysisId"`
	UserID            uint64 `gorm:"index;not null" json:"userId"`
	FeedbackType      string `gorm:"size:24;not null" json:"feedbackType"`
	Accepted          *bool  `json:"accepted"`
	ManualIntentLevel string `gorm:"size:16" json:"manualIntentLevel"`
	Note              string `gorm:"size:1024" json:"note"`
}

func (CrmCustomerIntentFeedback) TableName() string { return "crm_customer_intent_feedback" }

// CrmCustomerIntentTask tracks a batch analysis request independently from
// Redis, so progress and failures remain queryable after a restart.
type CrmCustomerIntentTask struct {
	Base
	TenantID      uint64     `gorm:"index;default:0" json:"tenantId"`
	CreatedBy     uint64     `gorm:"index;not null" json:"createdBy"`
	Status        string     `gorm:"size:16;index;not null" json:"status"`
	TotalCount    int        `json:"totalCount"`
	PendingCount  int        `json:"pendingCount"`
	RunningCount  int        `json:"runningCount"`
	SuccessCount  int        `json:"successCount"`
	FailedCount   int        `json:"failedCount"`
	CanceledCount int        `json:"canceledCount"`
	MaxAttempts   int        `json:"maxAttempts"`
	ErrorMessage  string     `gorm:"size:1024" json:"errorMessage"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

func (CrmCustomerIntentTask) TableName() string { return "crm_customer_intent_task" }

// CrmCustomerIntentTaskItem is one customer execution inside a batch task.
type CrmCustomerIntentTaskItem struct {
	Base
	TaskID        uint64     `gorm:"uniqueIndex:uk_intent_task_customer;index;not null" json:"taskId"`
	TenantID      uint64     `gorm:"uniqueIndex:uk_intent_task_customer;index;default:0" json:"tenantId"`
	CustomerID    uint64     `gorm:"uniqueIndex:uk_intent_task_customer;not null" json:"customerId"`
	TriggerUserID uint64     `gorm:"index;not null" json:"triggerUserId"`
	Status        string     `gorm:"size:16;index;not null" json:"status"`
	Attempts      int        `json:"attempts"`
	ErrorMessage  string     `gorm:"size:1024" json:"errorMessage"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

func (CrmCustomerIntentTaskItem) TableName() string { return "crm_customer_intent_task_item" }

// SysApi is one registered HTTP endpoint. The table is synchronized from the
// Gin route table at boot (code is the source of truth); only Title is
// maintained manually through the API-management page.
type SysApi struct {
	Base
	Method  string `gorm:"size:8;uniqueIndex:uk_api_method_path;not null" json:"method"`
	Path    string `gorm:"size:255;uniqueIndex:uk_api_method_path;not null" json:"path"`
	Handler string `gorm:"size:128" json:"handler"`
	Title   string `gorm:"size:64" json:"title"`
	Module  string `gorm:"size:32;index" json:"module"` // system / crm / auth / dashboard
	Perms   string `gorm:"size:128" json:"perms"`       // empty = no button-level perm required
	Status  int8   `gorm:"default:1" json:"status"`     // 1 enabled, 0 disabled (reserved)
}

func (SysApi) TableName() string { return "sys_api" }
