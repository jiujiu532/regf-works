package common

// ProxyEntry 代理配置
type ProxyEntry struct {
	HTTP  string `json:"http"`
	HTTPS string `json:"https"`
}

// Config 平台配置
type Config map[string]string

// RegisterResult 注册结果
type RegisterResult struct {
	OK       bool                   `json:"ok"`
	Email    string                 `json:"email,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Platform string                 `json:"platform,omitempty"`
	Status   string                 `json:"status,omitempty"` // "success" | "failed"
	Time     string                 `json:"time,omitempty"`   // 时间戳
	Data     map[string]interface{} `json:"data,omitempty"`
}
