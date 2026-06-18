package provider

// Config Provider 适配器配置（从 TOML 解析后传入）
type Config struct {
	Name         string
	DisplayName  string
	BaseURL      string
	APIKey       string // 已从环境变量解析
	AuthMethod   string
	DefaultModel string
	Models       []ModelConfigItem
}

// ModelConfigItem 模型配置项
type ModelConfigItem struct {
	ID            string
	Name          string
	ContextWindow int
	CostInput     float64
	CostCachedInput float64
	CostOutput    float64
	Reasoning     bool
	ToolCall      bool
	PrefixCache   bool
	Vision        bool
	Voice         bool
}
