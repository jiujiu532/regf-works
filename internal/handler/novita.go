package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/grok-fireworks-reg/internal/common"
	"github.com/grok-fireworks-reg/internal/config"
	"github.com/grok-fireworks-reg/internal/novita"
)

// NovitaRegisterRequest Novita 注册请求体
type NovitaRegisterRequest struct {
	Proxy         string `json:"proxy,omitempty"`
	Count         int    `json:"count,omitempty"`
	Concurrency   int    `json:"concurrency,omitempty"`
	EmailProvider string `json:"email_provider,omitempty"`
	Delay         int    `json:"delay,omitempty"` // 每个账号之间的间隔（秒）
}

// NovitaHandler Novita 注册处理器
type NovitaHandler struct {
	cfg     *config.Config
	storage *common.ResultStorage
}

// NewNovitaHandler 创建 NovitaHandler
func NewNovitaHandler(cfg *config.Config, storage *common.ResultStorage) *NovitaHandler {
	return &NovitaHandler{cfg: cfg, storage: storage}
}

// Register POST /api/novita/register
func (h *NovitaHandler) Register(c *gin.Context) {
	var req NovitaRegisterRequest
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
	maxConcurrent := h.cfg.Novita.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	if concurrency > maxConcurrent {
		concurrency = maxConcurrent
	}

	workerCfg := h.cfg.ToNovitaConfig()
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
	delay := req.Delay

	go func() {
		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			// 从第 2 个任务开始，等待间隔
			if i > 0 && delay > 0 {
				common.LogSend(logCh, fmt.Sprintf("[*] 等待 %d 秒后注册下一个...", delay))
				select {
				case <-ctx.Done():
					break
				case <-time.After(time.Duration(delay) * time.Second):
				}
			}

			select {
			case <-ctx.Done():
				break
			default:
			}

			semaphore <- struct{}{}
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				defer func() { <-semaphore }()

				taskProxy := proxy
				if pool := h.cfg.GetProxyPool(); len(pool) > 0 {
					taskProxy = pool[idx%len(pool)]
				}

				opts := novita.RegisterOpts{
					Proxy:  taskProxy,
					Config: workerCfg,
					LogCh:  logCh,
				}
				result := novita.Register(ctx, opts)
				resultCh <- result
			}(i)
		}
		wg.Wait()
		close(resultCh)
		close(logCh)
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
			result.Platform = "novita"
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
