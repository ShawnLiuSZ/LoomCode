package agent

import (
	"sync"
)

// EffortLevel 思考强度级别
type EffortLevel int

const (
	// EffortLow 低强度 - 快速响应
	EffortLow EffortLevel = iota
	// EffortMedium 中等强度 - 平衡模式
	EffortMedium
	// EffortHigh 高强度 - 深度思考
	EffortHigh
)

// String 返回强度级别字符串
func (e EffortLevel) String() string {
	switch e {
	case EffortLow:
		return "low"
	case EffortMedium:
		return "medium"
	case EffortHigh:
		return "high"
	default:
		return "medium"
	}
}

// ParseEffortLevel 解析强度级别
func ParseEffortLevel(s string) EffortLevel {
	switch s {
	case "low", "l":
		return EffortLow
	case "medium", "m", "":
		return EffortMedium
	case "high", "h":
		return EffortHigh
	default:
		return EffortMedium
	}
}

// EffortManager 思考强度管理器
type EffortManager struct {
	mu          sync.RWMutex
	level       EffortLevel
	maxSteps    map[EffortLevel]int
	reasonEffort map[EffortLevel]string
}

// NewEffortManager 创建思考强度管理器
func NewEffortManager() *EffortManager {
	return &EffortManager{
		level: EffortMedium,
		maxSteps: map[EffortLevel]int{
			EffortLow:    5,
			EffortMedium: 10,
			EffortHigh:   20,
		},
		reasonEffort: map[EffortLevel]string{
			EffortLow:    "low",
			EffortMedium: "medium",
			EffortHigh:   "high",
		},
	}
}

// SetLevel 设置强度级别
func (m *EffortManager) SetLevel(level EffortLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.level = level
}

// GetLevel 获取当前强度级别
func (m *EffortManager) GetLevel() EffortLevel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.level
}

// GetMaxSteps 获取当前强度对应的最大步数
func (m *EffortManager) GetMaxSteps() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxSteps[m.level]
}

// GetReasoningEffort 获取当前强度对应的推理强度参数
func (m *EffortManager) GetReasoningEffort() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reasonEffort[m.level]
}

// SetMaxSteps 设置指定强度的最大步数
func (m *EffortManager) SetMaxSteps(level EffortLevel, steps int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxSteps[level] = steps
}

// ListLevels 列出所有可用的强度级别
func (m *EffortManager) ListLevels() []EffortLevel {
	return []EffortLevel{EffortLow, EffortMedium, EffortHigh}
}

// GetDescription 获取强度级别的描述
func (m *EffortManager) GetDescription(level EffortLevel) string {
	switch level {
	case EffortLow:
		return "快速响应，适合简单任务"
	case EffortMedium:
		return "平衡模式，适合大多数任务"
	case EffortHigh:
		return "深度思考，适合复杂推理"
	default:
		return "未知级别"
	}
}
