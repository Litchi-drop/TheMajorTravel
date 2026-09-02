package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 统一响应包约定（见 docs/项目开发报告.md §6）：
// 成功 {"ok":true,"data":...}；失败 {"ok":false,"error":"可直接展示的中文文案"}
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

func Err(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"ok": false, "error": msg})
}
