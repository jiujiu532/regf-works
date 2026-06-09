package fireworks

import "github.com/grok-fireworks-reg/internal/common"

// RegisterOpts Fireworks 注册选项
type RegisterOpts struct {
	Proxy  *common.ProxyEntry
	Config common.Config
	LogCh  chan<- string
}

// ServiceRequest Python 服务请求体
type ServiceRequest struct {
	Email        string            `json:"email"`
	Password     string            `json:"password,omitempty"`
	Proxy        string            `json:"proxy,omitempty"`
	YYDSMailURL  string            `json:"yydsmail_url,omitempty"`
	YYDSMailKey  string            `json:"yydsmail_key,omitempty"`
	MailProvider string            `json:"mail_provider,omitempty"`
	MailMeta     map[string]string `json:"mail_meta,omitempty"`
}

// ServiceResult Python 服务返回结果
type ServiceResult struct {
	OK                   bool        `json:"ok"`
	Email                string      `json:"email"`
	Password             string      `json:"password"`
	APIKey               string      `json:"apikey"`
	AccountID            string      `json:"account_id"`
	RequestedAccountID   string      `json:"requested_account_id"`
	KeyID                string      `json:"key_id"`
	UserSub              string      `json:"user_sub"`
	FirstName            string      `json:"first_name"`
	LastName             string      `json:"last_name"`
	SuspendState         string      `json:"suspend_state"`
	AccountStatusCode    string      `json:"account_status_code"`
	AccountStatusMessage string      `json:"account_status_message"`
	ModelsCheck          string      `json:"models_check"`
	Warning              string      `json:"warning"`
	Error                string      `json:"error"`
	Retriable            bool        `json:"retriable"`
	QuotaSummary         interface{} `json:"quota_summary"`
	QuotaNames           interface{} `json:"quota_names"`
}
