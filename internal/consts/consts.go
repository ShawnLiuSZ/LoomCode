// Package consts 定义项目级共享常量，避免魔法数字分散在各模块。
package consts

import "time"

const (
	// DefaultHTTPTimeout 默认 HTTP 请求超时时间
	DefaultHTTPTimeout = 120 * time.Second

	// StreamChannelBufferSize 流式响应通道缓冲区大小
	StreamChannelBufferSize = 100

	// MaxRenderCacheEntries TUI 渲染缓存最大条目数
	MaxRenderCacheEntries = 500

	// CostGreenThreshold 成本绿色阈值（单轮 < 此值）
	CostGreenThreshold = 0.05

	// CostYellowThreshold 成本黄色阈值（单轮 < 此值为黄色，>= 为红色）
	CostYellowThreshold = 0.20
)
