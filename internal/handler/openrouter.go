package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/grok-fireworks-reg/internal/common"
	"github.com/grok-fireworks-reg/internal/config"
	"github.com/grok-fireworks-reg/internal/openrouter"
)

// OpenRouterRegisterRequest OpenRouter 注册请求体
type OpenRouterRegisterRequest struct {
	Proxy         string `json:"proxy,omitempty"`
	Count         int    `json:"count,omitempty"`
	Concurrency   int    `json:"concurrency,omitempty"`
	EmailProvider string `json:"email_provider,omitempty"`
}

// OpenRouterHandler OpenRouter 注册处理器
type OpenRouterHandler struct {
	cfg     *config.Config
	storage *common.ResultStorage
}

// NewOpenRouterHandler 创建 OpenRouterHandler
func NewOpenRouterHandler(cfg *config.Config, storage *common.ResultStorage) *OpenRouterHandler {
	return &OpenRouterHandler{cfg: cfg, storage: storage}
}

// Register POST /api/openrouter/register
func (h *OpenRouterHandler) Register(c *gin.Context) {
	var req OpenRouterRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %s", err)})
		return
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	maxConcurrent := h.cfg.OpenRouter.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	if concurrency > maxConcurrent {
		concurrency = maxConcurrent
	}

	workerCfg := h.cfg.ToOpenRouterConfig()
	if req.EmailProvider != "" {
		workerCfg["email_provider_priority"] = req.EmailProvider
	}

	proxy := h.cfg.GetDefaultProxy()
	if req.Proxy != "" {
		proxy = &common.ProxyEntry{HTTP: req.Proxy, HTTPS: req.Proxy}
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeaderNow()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		<-c.Writer.CloseNotify()
		cancel()
	}()

	logCh := make(chan string, 100)

	writeSSE := func(event, data string) {
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		c.Writer.Flush()
	}

	resultCh := make(chan *common.RegisterResult, count)
	semaphore := make(chan struct{}, concurrency)

	go func() {
		defer close(logCh)
		defer close(resultCh)
		for i := 0; i < count; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			semaphore <- struct{}{}
			go func(idx int) {
				defer func() { <-semaphore }()

				taskProxy := proxy
				if pool := h.cfg.GetProxyPool(); len(pool) > 0 {
					taskProxy = pool[idx%len(pool)]
				}

				opts := openrouter.RegisterOpts{
					Proxy:  taskProxy,
					Config: workerCfg,
					LogCh:  logCh,
				}
				result := openrouter.Register(ctx, opts)
				resultCh <- result
			}(i)
		}

		for i := 0; i < concurrency && i < count; i++ {
			semaphore <- struct{}{}
		}
	}()

	for {
		select {
		case msg, ok := <-logCh:
			if !ok {
				for result := range resultCh {
					data, _ := json.Marshal(result)
					writeSSE("result", string(data))
				}
				return
			}
			writeSSE("log", msg)

		case result, ok := <-resultCh:
			if !ok {
				for msg := range logCh {
					writeSSE("log", msg)
				}
				return
			}
			result.Platform = "openrouter"
			if result.OK {
				result.Status = "success"
			} else {
				result.Status = "failed"
			}
			result.Time = time.Now().Format("2006-01-02 15:04:05")
			if err := h.storage.Append(*result); err != nil {
				log.Error().Err(err).Msg("保存结果失败")
			}
			data, _ := json.Marshal(result)
			writeSSE("result", string(data))

		case <-ctx.Done():
			writeSSE("log", "任务已取消")
			return
		}
	}
}
