package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/grok-fireworks-reg/internal/common"
)

// ResultsHandler 结果管理处理器
type ResultsHandler struct {
	storage *common.ResultStorage
}

// NewResultsHandler 创建 ResultsHandler
func NewResultsHandler(storage *common.ResultStorage) *ResultsHandler {
	return &ResultsHandler{storage: storage}
}

// GetResults GET /api/results — 获取所有历史结果
func (h *ResultsHandler) GetResults(c *gin.Context) {
	results := h.storage.GetAll()
	c.JSON(http.StatusOK, results)
}

// ClearResults DELETE /api/results — 清空所有结果
func (h *ResultsHandler) ClearResults(c *gin.Context) {
	if err := h.storage.Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "所有结果已清空"})
}
