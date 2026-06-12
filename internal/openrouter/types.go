package openrouter

import "github.com/grok-fireworks-reg/internal/common"

// RegisterOpts OpenRouter 注册选项
type RegisterOpts struct {
	Proxy  *common.ProxyEntry
	Config common.Config
	LogCh  chan<- string
}

// ServiceRequest Python 服务请求体
type ServiceRequest struct {
	Email         string            `json:"email"`
	Password      string            `json:"password,omitempty"`
	Proxy         string            `json:"proxy,omitempty"`
	SolverType    string            `json:"solver_type,omitempty"`
	SolverAPI     string            `json:"solver_api,omitempty"`
	YesCaptchaKey string            `json:"yescaptcha_key,omitempty"`
	MailProvider  string            `json:"mail_provider,omitempty"`
	MailMeta      map[string]string `json:"mail_meta,omitempty"`
	YYDSMailURL   string            `json:"yydsmail_url,omitempty"`
	YYDSMailKey   string            `json:"yydsmail_key,omitempty"`
	AhemBaseURL   string            `json:"ahem_base_url,omitempty"`
}

// ServiceResult Python 服务返回结果
type ServiceResult struct {
	OK        bool   `json:"ok"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	APIKey    string `json:"api_key"`
	SessionID string `json:"session_id"`
	Error     string `json:"error"`
	Retriable bool   `json:"retriable"`
}
