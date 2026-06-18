package provider

import "time"

// Capabilities 声明 Provider 的能力
type Capabilities struct {
	SupportsReasoning   bool
	SupportsToolCall    bool
	SupportsPrefixCache bool
	SupportsStreaming   bool
	SupportsVision      bool
	SupportsVoice       bool
	SupportsOAuth       bool
	NeedsToolRepair     bool
	CacheTTL            time.Duration
	MaxToolCallsPerRound int
}
