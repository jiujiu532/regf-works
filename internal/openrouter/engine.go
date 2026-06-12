package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/grok-fireworks-reg/internal/common"
	"github.com/grok-fireworks-reg/pkg/tempmail"
)

// ─── 邮箱域名黑名单 ───

var openrouterBlacklist = tempmail.NewDomainBlacklist("openrouter", 0, "data/blacklist.json")

// GetBlacklist 返回 OpenRouter 黑名单实例（供 API handler 访问）
func GetBlacklist() *tempmail.DomainBlacklist {
	return openrouterBlacklist
}

// Register 执行一次完整的 OpenRouter 注册流程
func Register(ctx context.Context, opts RegisterOpts) *common.RegisterResult {
	logf := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		common.LogSend(opts.LogCh, msg)
		log.Info().Str("platform", "openrouter").Msg(msg)
	}

	// 获取服务地址
	serviceURL := common.SettingOrDefault(opts.Config, "openrouter_reg_url", "http://127.0.0.1:5001")

	// 1. 创建临时邮箱（含黑名单过滤 + 重试）
	logf("创建临时邮箱...")
	mailProvider := tempmail.NewMultiProvider(opts.Config)
	if mailProvider.ProviderCount() == 0 {
		return &common.RegisterResult{OK: false, Error: "无可用邮箱 provider", Platform: "openrouter"}
	}

	var email string
	var meta map[string]string
	var err error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		email, meta, err = mailProvider.GenerateEmail(ctx)
		if err != nil {
			return &common.RegisterResult{OK: false, Error: fmt.Sprintf("创建邮箱失败: %s", err), Platform: "openrouter"}
		}

		domain := tempmail.ExtractDomain(email)
		if openrouterBlacklist.IsBanned(domain) {
			logf("[!] 域名 %s 已被拉黑，重新生成邮箱 (attempt %d/%d)", domain, attempt+1, maxRetries)
			go mailProvider.DeleteEmail(context.Background(), email, meta)
			continue
		}
		break
	}

	domain := tempmail.ExtractDomain(email)
	if openrouterBlacklist.IsBanned(domain) {
		logf("[-] 无法获取非黑名单邮箱，已重试 %d 次", maxRetries)
		return &common.RegisterResult{OK: false, Email: email, Error: "无法获取非黑名单邮箱", Platform: "openrouter"}
	}

	logf("邮箱已创建: %s (provider=%s)", email, meta["provider"])

	defer func() {
		_ = mailProvider.DeleteEmail(context.Background(), email, meta)
	}()

	// 2. 构造请求体
	proxyStr := ""
	if opts.Proxy != nil {
		proxyStr = opts.Proxy.HTTPS
		if proxyStr == "" {
			proxyStr = opts.Proxy.HTTP
		}
	}

	reqBody := ServiceRequest{
		Email:         email,
		Proxy:         proxyStr,
		SolverType:    common.SettingOrDefault(opts.Config, "openrouter_solver_type", "selfhost"),
		SolverAPI:     common.SettingOrDefault(opts.Config, "openrouter_solver_api", "http://localhost:5072"),
		YesCaptchaKey: common.SettingOrDefault(opts.Config, "yescaptcha_key", ""),
		MailProvider:  meta["provider"],
		MailMeta:      meta,
		YYDSMailURL:   common.SettingOrDefault(opts.Config, "yydsmail_base_url", ""),
		YYDSMailKey:   common.SettingOrDefault(opts.Config, "yydsmail_api_key", ""),
		AhemBaseURL:   common.SettingOrDefault(opts.Config, "ahem_base_url", ""),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return &common.RegisterResult{OK: false, Email: email, Error: fmt.Sprintf("序列化请求失败: %s", err), Platform: "openrouter"}
	}

	// 3. 调用 Python 服务
	logf("调用注册服务: %s", serviceURL)
	result, err := callRegService(ctx, serviceURL+"/openrouter/process", bodyBytes, opts.LogCh)
	if err != nil {
		return &common.RegisterResult{OK: false, Email: email, Error: fmt.Sprintf("注册服务调用失败: %s", err), Platform: "openrouter"}
	}

	// 4. 转换结果
	data := map[string]interface{}{
		"api_key":    result.APIKey,
		"session_id": result.SessionID,
		"password":   result.Password,
	}

	if result.OK {
		logf("注册成功: email=%s key=%s...", email, common.TruncStr(result.APIKey, 20))
	} else {
		logf("注册失败: %s (retriable=%v)", result.Error, result.Retriable)
		// 如果错误信息包含域名被 Clerk 拦截的关键词
		errLower := strings.ToLower(result.Error)
		if strings.Contains(errLower, "blocked") || strings.Contains(errLower, "拦截") ||
			strings.Contains(errLower, "临时邮箱") || strings.Contains(errLower, "form_email_address_blocked") {
			d := tempmail.ExtractDomain(email)
			openrouterBlacklist.Ban(d)
			logf("[!] 域名 %s 已拉黑 (Clerk 拒绝)", d)
		}
	}

	return &common.RegisterResult{
		OK:       result.OK,
		Email:    email,
		Error:    result.Error,
		Platform: "openrouter",
		Data:     data,
	}
}

// callRegService 调用 Python 注册服务
func callRegService(ctx context.Context, url string, body []byte, logCh chan<- string) (*ServiceResult, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody)[:300])
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)

	var lastLine string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "LOG:") {
			logMsg := strings.TrimPrefix(line, "LOG:")
			common.LogSend(logCh, logMsg)
			log.Debug().Str("platform", "openrouter").Msg(logMsg)
		} else {
			lastLine = line
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取响应流失败: %w", err)
	}

	if lastLine == "" {
		return nil, fmt.Errorf("注册服务未返回结果")
	}

	var result ServiceResult
	if err := json.Unmarshal([]byte(lastLine), &result); err != nil {
		return nil, fmt.Errorf("解析结果 JSON 失败: %w (raw=%s)", err, common.TruncStr(lastLine, 200))
	}

	return &result, nil
}
