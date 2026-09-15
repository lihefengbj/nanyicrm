package system

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type MenuHandler struct {
	db *gorm.DB
}

func NewMenuHandler(db *gorm.DB) *MenuHandler {
	return &MenuHandler{db: db}
}

// Tree returns all enabled menus as a tree for role assignment and routing.
// @Summary  菜单树（角色授权/菜单管理）
// @Tags     系统管理-菜单
// @Description 需要权限：system:menu:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/menu/tree [get]
func (h *MenuHandler) Tree(c *gin.Context) {
	var menus []model.SysMenu
	query := h.db.Order("sort, id")
	if title := c.Query("title"); title != "" {
		query = query.Where("title LIKE ?", "%"+title+"%")
	}
	if status := c.Query("status"); status != "" {
		if v, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", v)
		}
	}
	if err := query.Find(&menus).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	// When filtering, keep the ancestor chain of each hit so the tree stays
	// navigable.
	if c.Query("title") != "" || c.Query("status") != "" {
		menus = withAncestors(h.db, menus)
	}
	common.OK(c, buildMenuTree(menus, 0))
}

// withAncestors loads all ancestors of the given menus and returns a
// deduplicated slice containing both.
func withAncestors(db *gorm.DB, menus []model.SysMenu) []model.SysMenu {
	seen := make(map[uint64]bool, len(menus))
	for _, m := range menus {
		seen[m.ID] = true
	}
	var queue []uint64
	for _, m := range menus {
		if m.ParentID != 0 {
			queue = append(queue, m.ParentID)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id == 0 || seen[id] {
			continue
		}
		var parent model.SysMenu
		if err := db.First(&parent, id).Error; err != nil {
			continue
		}
		seen[id] = true
		menus = append(menus, parent)
		queue = append(queue, parent.ParentID)
	}
	return menus
}

type MenuSaveRequest struct {
	ParentID  uint64 `json:"parentId"`
	Title     string `json:"title" binding:"required,max=64"`
	Type      int8   `json:"type" binding:"required,min=1,max=3"`
	Path      string `json:"path" binding:"max=128"`
	Component string `json:"component" binding:"max=128"`
	Perms     string `json:"perms" binding:"max=128"`
	Icon      string `json:"icon" binding:"max=64"`
	Sort      int    `json:"sort"`
	Visible   int8   `json:"visible"`
	Status    int8   `json:"status"`
}

// validateMenuRequest checks type-dependent field rules and parent/type
// constraints. It returns a user-facing message, empty means valid.
func (h *MenuHandler) validateMenuRequest(req *MenuSaveRequest, selfID uint64) string {
	if req.Type < 1 || req.Type > 3 {
		return "菜单类型只能是 1 目录 / 2 菜单 / 3 按钮"
	}
	switch req.Type {
	case 1, 2:
		if req.Path == "" {
			return "目录和菜单必须填写路由路径"
		}
	case 3:
		if req.Perms == "" {
			return "按钮必须填写权限标识"
		}
	}
	if req.Type == 2 && req.Component == "" {
		return "菜单必须填写前端组件路径"
	}
	if req.ParentID == 0 {
		if req.Type == 3 {
			return "按钮必须挂在菜单下"
		}
		return ""
	}
	if selfID != 0 && req.ParentID == selfID {
		return "上级菜单不能是自身"
	}
	var parent model.SysMenu
	if err := h.db.First(&parent, req.ParentID).Error; err != nil {
		return "上级菜单不存在"
	}
	if req.Type == 3 && parent.Type != 2 {
		return "按钮只能挂在菜单下"
	}
	if req.Type != 3 && parent.Type != 1 {
		return "目录和菜单只能挂在目录下"
	}
	if req.Perms != "" {
		var count int64
		query := h.db.Model(&model.SysMenu{}).Where("perms = ?", req.Perms)
		if selfID != 0 {
			query = query.Where("id <> ?", selfID)
		}
		if err := query.Count(&count).Error; err == nil && count > 0 {
			return "权限标识已存在"
		}
	}
	return ""
}

// @Summary  新增菜单
// @Tags     系统管理-菜单
// @Description 需要权限：system:menu:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/menu [post]
func (h *MenuHandler) Create(c *gin.Context) {
	var req MenuSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if msg := h.validateMenuRequest(&req, 0); msg != "" {
		common.FailMsg(c, common.CodeParamInvalid, msg)
		return
	}
	menu := model.SysMenu{
		ParentID:  req.ParentID,
		Title:     req.Title,
		Type:      req.Type,
		Path:      req.Path,
		Component: req.Component,
		Perms:     req.Perms,
		Icon:      req.Icon,
		Sort:      req.Sort,
		Visible:   req.Visible,
		Status:    req.Status,
	}
	if menu.Visible == 0 {
		menu.Visible = 1
	}
	if menu.Status == 0 {
		menu.Status = 1
	}
	if err := h.db.Create(&menu).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": menu.ID})
}

// isDescendant reports whether walking up from candidate ever reaches id,
// which would make the move create a cycle.
func (h *MenuHandler) isDescendant(id, candidate uint64) bool {
	for candidate != 0 {
		if candidate == id {
			return true
		}
		var parent model.SysMenu
		if err := h.db.Select("id", "parent_id").First(&parent, candidate).Error; err != nil {
			return false
		}
		candidate = parent.ParentID
	}
	return false
}

// @Summary  编辑菜单
// @Tags     系统管理-菜单
// @Description 需要权限：system:menu:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/menu/{id} [put]
func (h *MenuHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var menu model.SysMenu
	if err := h.db.First(&menu, id).Error; err != nil {
		common.Fail(c, common.CodeMenuNotFound)
		return
	}
	var req MenuSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if msg := h.validateMenuRequest(&req, id); msg != "" {
		common.FailMsg(c, common.CodeParamInvalid, msg)
		return
	}
	if req.ParentID != menu.ParentID && h.isDescendant(id, req.ParentID) {
		common.FailMsg(c, common.CodeParamInvalid, "上级菜单不能是自身的子孙节点")
		return
	}
	if req.Type != menu.Type {
		var children int64
		h.db.Model(&model.SysMenu{}).Where("parent_id = ?", id).Count(&children)
		if children > 0 {
			common.FailMsg(c, common.CodeParamInvalid, "存在子级的菜单不允许变更类型")
			return
		}
	}
	menu.ParentID = req.ParentID
	menu.Title = req.Title
	menu.Type = req.Type
	menu.Path = req.Path
	menu.Component = req.Component
	menu.Perms = req.Perms
	menu.Icon = req.Icon
	menu.Sort = req.Sort
	menu.Visible = req.Visible
	menu.Status = req.Status
	if err := h.db.Save(&menu).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除菜单
// @Tags     系统管理-菜单
// @Description 需要权限：system:menu:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/menu/{id} [delete]
func (h *MenuHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var menu model.SysMenu
	if err := h.db.First(&menu, id).Error; err != nil {
		common.Fail(c, common.CodeMenuNotFound)
		return
	}
	var children int64
	h.db.Model(&model.SysMenu{}).Where("parent_id = ?", id).Count(&children)
	if children > 0 {
		common.FailMsg(c, common.CodeParamInvalid, "请先删除子级菜单")
		return
	}
	var roleRefs int64
	h.db.Model(&model.SysRoleMenu{}).Where("menu_id = ?", id).Count(&roleRefs)
	if roleRefs > 0 {
		common.FailMsg(c, common.CodeParamInvalid, "菜单已被角色引用，请先解除角色授权")
		return
	}
	if err := h.db.Delete(&menu).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func buildMenuTree(menus []model.SysMenu, parentID uint64) []model.SysMenu {
	tree := make([]model.SysMenu, 0)
	for _, m := range menus {
		if m.ParentID == parentID {
			node := m
			node.Children = buildMenuTree(menus, m.ID)
			tree = append(tree, node)
		}
	}
	return tree
}
