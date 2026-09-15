package system

import (
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// ApiEntry is one route registered in code, collected by the router while it
// wires handlers. It is the source of truth for SyncApis.
type ApiEntry struct {
	Method  string
	Path    string
	Handler string
	Perms   string
}

type ApiHandler struct {
	db       *gorm.DB
	registry *[]ApiEntry
}

func NewApiHandler(db *gorm.DB, registry *[]ApiEntry) *ApiHandler {
	return &ApiHandler{db: db, registry: registry}
}

// apiModule derives the module bucket from the route path:
// /api/v1/system/... -> system, /api/v1/crm/... -> crm, etc.
func apiModule(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return "other"
}

// SyncApis reconciles sys_api with the in-code registry. Idempotent: new
// routes are inserted, removed routes soft-deleted, changed perms updated.
// Manually maintained titles are never overwritten. Failures are logged, not
// fatal — the list simply refreshes on the next boot.
func SyncApis(db *gorm.DB, registry []ApiEntry) {
	seen := make(map[string]ApiEntry, len(registry))
	for _, e := range registry {
		seen[e.Method+" "+e.Path] = e
	}

	for _, e := range registry {
		var existing model.SysApi
		err := db.Unscoped().Where("method = ? AND path = ?", e.Method, e.Path).First(&existing).Error
		switch {
		case err == gorm.ErrRecordNotFound:
			row := model.SysApi{Method: e.Method, Path: e.Path, Handler: e.Handler, Module: apiModule(e.Path), Perms: e.Perms, Status: 1}
			if err := db.Create(&row).Error; err != nil {
				log.Printf("api sync: insert %s %s: %v", e.Method, e.Path, err)
			}
		case err != nil:
			log.Printf("api sync: lookup %s %s: %v", e.Method, e.Path, err)
		default:
			updates := map[string]interface{}{}
			if existing.DeletedAt.Valid {
				updates["deleted_at"] = nil // route restored in code
			}
			if existing.Perms != e.Perms {
				updates["perms"] = e.Perms
			}
			if existing.Handler != e.Handler {
				updates["handler"] = e.Handler
			}
			if existing.Module != apiModule(e.Path) {
				updates["module"] = apiModule(e.Path)
			}
			if len(updates) > 0 {
				if err := db.Unscoped().Model(&existing).Updates(updates).Error; err != nil {
					log.Printf("api sync: update %s %s: %v", e.Method, e.Path, err)
				}
			}
		}
	}

	// Soft-delete rows whose route disappeared from code.
	var rows []model.SysApi
	if err := db.Find(&rows).Error; err != nil {
		log.Printf("api sync: list: %v", err)
		return
	}
	for _, row := range rows {
		if _, ok := seen[row.Method+" "+row.Path]; !ok {
			if err := db.Delete(&row).Error; err != nil {
				log.Printf("api sync: remove %s %s: %v", row.Method, row.Path, err)
			}
		}
	}
}

// List 接口分页列表
// @Summary  接口分页列表
// @Tags     系统管理-接口
// @Description 需要权限：system:api:list
// @Param    method   query  string  false "HTTP 方法"
// @Param    path     query  string  false "路径模糊"
// @Param    module   query  string  false "所属模块"
// @Param    pageNum  query  int     false "页码"
// @Param    pageSize query  int     false "每页条数"
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/api [get]
func (h *ApiHandler) List(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 10
	}

	query := h.db.Model(&model.SysApi{})
	if method := c.Query("method"); method != "" {
		query = query.Where("method = ?", method)
	}
	if path := c.Query("path"); path != "" {
		query = query.Where("path LIKE ?", "%"+path+"%")
	}
	if module := c.Query("module"); module != "" {
		query = query.Where("module = ?", module)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var rows []model.SysApi
	if err := query.Order("module, path, method").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"records": rows, "total": total, "pageNum": pageNum, "pageSize": pageSize})
}

// All 全部接口
// @Summary  全部接口（分组展示用）
// @Tags     系统管理-接口
// @Description 需要权限：system:api:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/api/all [get]
func (h *ApiHandler) All(c *gin.Context) {
	var rows []model.SysApi
	if err := h.db.Order("module, path, method").Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, rows)
}

type ApiUpdateTitleRequest struct {
	Title string `json:"title" binding:"max=64"`
}

// UpdateTitle 编辑接口名称
// @Summary  编辑接口名称（其余字段由代码同步，只读）
// @Tags     系统管理-接口
// @Description 需要权限：system:api:update
// @Param    id    path  int                    true  "主键"
// @Param    body  body  ApiUpdateTitleRequest  true  "请求体"
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/api/{id} [put]
func (h *ApiHandler) UpdateTitle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var row model.SysApi
	if err := h.db.First(&row, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	var req ApiUpdateTitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if err := h.db.Model(&row).Update("title", req.Title).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// Sync 手动触发同步
// @Summary  手动触发一次接口同步（常规由启动时自动完成）
// @Tags     系统管理-接口
// @Description 需要权限：system:api:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/api/sync [post]
func (h *ApiHandler) Sync(c *gin.Context) {
	SyncApis(h.db, *h.registry)
	common.OK(c, nil)
}
