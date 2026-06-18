package control

import (
	"fmt"
	"sync"

	"github.com/ShawnLiuSZ/Helix/internal/provider"
)

// CostLevel 成本等级
type CostLevel int

const (
	LevelGreen  CostLevel = iota // < $0.05
	LevelYellow                  // $0.05 - $0.20
	LevelRed                     // >= $0.20
)

func (l CostLevel) String() string {
	switch l {
	case LevelGreen:
		return "green"
	case LevelYellow:
		return "yellow"
	case LevelRed:
		return "red"
	default:
		return "unknown"
	}
}

// CostController 成本控制器
type CostController struct {
	mu sync.Mutex

	// 分层模型策略
	primaryModel   string // 主模型（flash）
	fallbackModel  string // 备用模型（pro）
	useFallback    bool   // 是否使用备用模型

	// 辅助调用模型（摘要、修复等强制低成本）
	auxModel string

	// 累计成本
	totalCost     float64
	sessionCost   float64
	lastTurnCost  float64

	// 成本阈值
	greenThreshold  float64 // < 此值为绿色
	yellowThreshold float64 // < 此值为黄色，>= 为红色

	// 压缩阈值
	compressThreshold int // 工具结果超过此 token 数时压缩
}

// NewCostController 创建成本控制器
func NewCostController(primaryModel, fallbackModel string) *CostController {
	return &CostController{
		primaryModel:     primaryModel,
		fallbackModel:    fallbackModel,
		auxModel:         primaryModel, // 辅助调用默认也用 flash
		greenThreshold:   0.05,
		yellowThreshold:  0.20,
		compressThreshold: 3000,
	}
}

// SetAuxModel 设置辅助调用模型
func (c *CostController) SetAuxModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auxModel = model
}

// ModelForMain 返回主对话使用的模型
func (c *CostController) ModelForMain() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.useFallback && c.fallbackModel != "" {
		return c.fallbackModel
	}
	return c.primaryModel
}

// ModelForAux 返回辅助调用使用的模型
func (c *CostController) ModelForAux() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.auxModel
}

// RequestUpgrade 请求升级到高级模型
func (c *CostController) RequestUpgrade() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.useFallback = true
}

// ResetUpgrade 重置为默认模型
func (c *CostController) ResetUpgrade() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.useFallback = false
}

// RecordCost 记录成本
func (c *CostController) RecordCost(cost provider.Cost) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.totalCost += cost.TotalCost
	c.sessionCost += cost.TotalCost
	c.lastTurnCost = cost.TotalCost
}

// TotalCost 返回累计总成本
func (c *CostController) TotalCost() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalCost
}

// SessionCost 返回当前会话成本
func (c *CostController) SessionCost() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionCost
}

// LastTurnCost 返回上一轮成本
func (c *CostController) LastTurnCost() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastTurnCost
}

// ResetSession 重置会话成本
func (c *CostController) ResetSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionCost = 0
	c.lastTurnCost = 0
}

// CostLevel 返回当前成本等级
func (c *CostController) CostLevel() CostLevel {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastTurnCost < c.greenThreshold {
		return LevelGreen
	}
	if c.lastTurnCost < c.yellowThreshold {
		return LevelYellow
	}
	return LevelRed
}

// ShouldCompress 判断工具结果是否需要压缩
func (c *CostController) ShouldCompress(tokenCount int) bool {
	return tokenCount > c.compressThreshold
}

// CompressResult 压缩工具结果
func (c *CostController) CompressResult(content string) string {
	// 简单截断 + 摘要
	const maxLen = 2000
	if len(content) <= maxLen {
		return content
	}

	head := content[:500]
	tail := ""
	if len(content) > 500 {
		remaining := len(content) - 500
		tail = content[len(content)-200:]
		return fmt.Sprintf("%s\n\n... (%d characters truncated) ...\n\n%s", head, remaining, tail)
	}
	return head + tail
}

// StatusReport 返回成本状态报告
func (c *CostController) StatusReport() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return fmt.Sprintf(
		"Cost: session=$%.4f | last turn=$%.4f (%s) | total=$%.4f | model=%s",
		c.sessionCost,
		c.lastTurnCost,
		c.currentLevel(),
		c.totalCost,
		c.currentModel(),
	)
}

func (c *CostController) currentLevel() string {
	if c.lastTurnCost < c.greenThreshold {
		return "green"
	}
	if c.lastTurnCost < c.yellowThreshold {
		return "yellow"
	}
	return "red"
}

func (c *CostController) currentModel() string {
	if c.useFallback && c.fallbackModel != "" {
		return c.fallbackModel
	}
	return c.primaryModel
}
