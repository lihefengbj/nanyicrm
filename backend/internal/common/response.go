package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PageResult struct {
	Records  interface{} `json:"records"`
	Total    int64       `json:"total"`
	PageNum  int         `json:"pageNum"`
	PageSize int         `json:"pageSize"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: CodeSuccess, Message: CodeMessage(CodeSuccess), Data: data})
}

func OKPage(c *gin.Context, records interface{}, total int64, pageNum, pageSize int) {
	OK(c, PageResult{Records: records, Total: total, PageNum: pageNum, PageSize: pageSize})
}

func Fail(c *gin.Context, code int) {
	FailMsg(c, code, CodeMessage(code))
}

func FailMsg(c *gin.Context, code int, msg string) {
	status := http.StatusOK
	switch code {
	case CodeUnauthorized, CodeTokenExpired, CodeLoginFailed:
		status = http.StatusUnauthorized
	case CodeForbidden:
		status = http.StatusForbidden
	}
	c.JSON(status, Response{Code: code, Message: msg})
}

// Abort is Fail plus aborting the middleware chain.
func Abort(c *gin.Context, code int) {
	Fail(c, code)
	c.Abort()
}
