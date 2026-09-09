package system

import (
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
func (h *MenuHandler) Tree(c *gin.Context) {
	var menus []model.SysMenu
	if err := h.db.Order("sort, id").Find(&menus).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, buildMenuTree(menus, 0))
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
