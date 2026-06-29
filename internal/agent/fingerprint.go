package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
)

// FingerprintTracker prefix 指纹追踪器。
//
// 对 prefix（静态 system prompt + tools 定义）计算 SHA256，请求前比对指纹
// 判断"理论上应命中"，请求后比对 LLM 返回的实际 cached tokens。当预期命中
// 但实际未命中时记录为 mismatch，便于定位 "为什么 prefix cache 没命中"。
type FingerprintTracker struct {
	mu              sync.Mutex
	lastFingerprint string
	lastHitExpected bool // 上次请求是否预期命中（指纹未变）
	totalRequests   int64
	expectedHits    int64 // 指纹未变的次数
	actualHits      int64 // 实际 cached > 0 的次数
	mismatchCount   int64 // 预期命中但实际未命中的次数
}

// NewFingerprintTracker 创建 FingerprintTracker。
func NewFingerprintTracker() *FingerprintTracker {
	return &FingerprintTracker{}
}

// ComputeFingerprint 计算给定内容的 SHA256 指纹，返回 hex 编码前 16 字符（便于日志显示）。
func ComputeFingerprint(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// RecordRequest 记录一次请求：
//   - 计算 prefix 指纹，与上次比对
//   - 指纹相同 → lastHitExpected = true, expectedHits++
//   - 指纹不同 → lastHitExpected = false, 更新 lastFingerprint
//   - totalRequests++
func (f *FingerprintTracker) RecordRequest(prefix string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	fp := ComputeFingerprint(prefix)
	if f.totalRequests > 0 && fp == f.lastFingerprint {
		f.lastHitExpected = true
		f.expectedHits++
	} else {
		f.lastHitExpected = false
		f.lastFingerprint = fp
	}
	f.totalRequests++
}

// RecordResponse 记录 LLM 响应中的实际缓存命中：
//   - cachedTokens > 0 → actualHits++
//   - 如果 lastHitExpected 且 cachedTokens == 0 → mismatchCount++，log.Printf 告警
func (f *FingerprintTracker) RecordResponse(cachedTokens int64) {
	f.mu.Lock()
	expected := f.lastHitExpected
	if cachedTokens > 0 {
		f.actualHits++
	}
	mismatch := false
	if cachedTokens == 0 && expected {
		f.mismatchCount++
		mismatch = true
	}
	mismatchCount := f.mismatchCount
	f.mu.Unlock()

	if mismatch {
		log.Printf("[fingerprint] WARN: prefix cache miss mismatch: expected hit but cached=0 (mismatch=%d)",
			mismatchCount)
	}
}

// Stats 返回累计统计：totalRequests, expectedHits, actualHits, mismatchCount。
func (f *FingerprintTracker) Stats() (totalRequests, expectedHits, actualHits, mismatchCount int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.totalRequests, f.expectedHits, f.actualHits, f.mismatchCount
}

// MismatchRate 不匹配率 = mismatchCount / expectedHits（预期命中但实际没命中的比例）。
// expectedHits == 0 时返回 0（避免除零）。
func (f *FingerprintTracker) MismatchRate() float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.expectedHits == 0 {
		return 0
	}
	return float64(f.mismatchCount) / float64(f.expectedHits)
}

// String 返回可读的统计摘要，便于日志输出。
func (f *FingerprintTracker) String() string {
	total, expected, actual, mismatch := f.Stats()
	return fmt.Sprintf(
		"FingerprintTracker{total=%d expected=%d actual=%d mismatch=%d rate=%.2f}",
		total, expected, actual, mismatch, f.MismatchRate(),
	)
}
