package provider

// ModelInfo 模型信息
type ModelInfo struct {
	ID            string
	Name          string
	ContextWindow int
	Cost          ModelCost
}

// ModelCost 模型成本
type ModelCost struct {
	Input       float64
	CachedInput float64
	Output      float64
}

// Usage token 用量
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Cost 单次调用成本
type Cost struct {
	InputCost  float64
	OutputCost float64
	TotalCost  float64
	Currency   string
}
