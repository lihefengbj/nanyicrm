package common

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type PageQuery struct {
	PageNum  int
	PageSize int
}

// ParsePageQuery reads pageNum/pageSize from query params with sane bounds.
func ParsePageQuery(c *gin.Context) PageQuery {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return PageQuery{PageNum: pageNum, PageSize: pageSize}
}

func (p PageQuery) Offset() int {
	return (p.PageNum - 1) * p.PageSize
}
