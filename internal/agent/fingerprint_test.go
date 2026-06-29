package agent

import (
	"strings"
	"testing"
)

// TestComputeFingerprint 验证 ComputeFingerprint 的确定性与区分度：
//   - 相同输入产生相同指纹
//   - 不同输入产生不同指纹
//   - 输出为 16 字符 hex（SHA256 前 8 字节）
func TestComputeFingerprint(t *testing.T) {
	a := "You are Helix, an AI coding assistant."
	b := "You are Helix, an AI coding assistant."
	c := "You are a different assistant."

	fa := ComputeFingerprint(a)
	fb := ComputeFingerprint(b)
	fc := ComputeFingerprint(c)

	// 相同输入 → 相同输出
	if fa != fb {
		t.Errorf("identical input produced different fingerprints: %s vs %s", fa, fb)
	}

	// 不同输入 → 不同输出
	if fa == fc {
		t.Errorf("different input produced same fingerprint: %s", fa)
	}

	// 长度应为 16（hex 编码前 8 字节）
	if len(fa) != 16 {
		t.Errorf("expected 16-char fingerprint, got %d: %q", len(fa), fa)
	}

	// 不应为空
	if fa == "" {
		t.Error("fingerprint should not be empty")
	}
}

// TestComputeFingerprint_Empty 验证空字符串也能稳定计算指纹（不 panic）。
func TestComputeFingerprint_Empty(t *testing.T) {
	fp := ComputeFingerprint("")
	if len(fp) != 16 {
		t.Errorf("expected 16-char fingerprint for empty input, got %d", len(fp))
	}
	// 两次调用应一致
	if fp != ComputeFingerprint("") {
		t.Error("fingerprint of empty string should be deterministic")
	}
}

// TestFingerprintTracker_RecordRequest 验证请求计数与预期命中计数：
//   - 第一次请求：totalRequests=1, expectedHits=0（无前次指纹可比）
//   - 第二次相同 prefix：totalRequests=2, expectedHits=1（指纹未变 → 预期命中）
func TestFingerprintTracker_RecordRequest(t *testing.T) {
	f := NewFingerprintTracker()

	prefix := "static-system-prompt + tools-json"

	// 第一次请求
	f.RecordRequest(prefix)
	total, expected, _, _ := f.Stats()
	if total != 1 {
		t.Errorf("after 1st request: expected totalRequests=1, got %d", total)
	}
	if expected != 0 {
		t.Errorf("after 1st request: expected expectedHits=0, got %d", expected)
	}

	// 第二次相同 prefix → 预期命中
	f.RecordRequest(prefix)
	total, expected, _, _ = f.Stats()
	if total != 2 {
		t.Errorf("after 2nd request: expected totalRequests=2, got %d", total)
	}
	if expected != 1 {
		t.Errorf("after 2nd request: expected expectedHits=1, got %d", expected)
	}

	// 第三次不同 prefix → 不预期命中，expectedHits 不增加
	f.RecordRequest("different-prefix")
	total, expected, _, _ = f.Stats()
	if total != 3 {
		t.Errorf("after 3rd request: expected totalRequests=3, got %d", total)
	}
	if expected != 1 {
		t.Errorf("after 3rd request: expected expectedHits=1 (unchanged), got %d", expected)
	}
}

// TestFingerprintTracker_RecordResponse_Mismatch 验证预期命中但实际未命中 → mismatchCount++。
// 场景：两次相同 prefix（第二次预期命中），但响应 cached=0。
func TestFingerprintTracker_RecordResponse_Mismatch(t *testing.T) {
	f := NewFingerprintTracker()

	prefix := "stable-prefix"
	f.RecordRequest(prefix) // 第一次：建立指纹，不预期命中
	f.RecordRequest(prefix) // 第二次：指纹相同 → 预期命中

	// 响应：cached=0 但预期命中 → mismatch
	f.RecordResponse(0)

	_, expected, _, mismatch := f.Stats()
	if expected != 1 {
		t.Errorf("expected expectedHits=1, got %d", expected)
	}
	if mismatch != 1 {
		t.Errorf("expected mismatchCount=1, got %d", mismatch)
	}

	// 不匹配率应为 1.0（1/1）
	if rate := f.MismatchRate(); rate != 1.0 {
		t.Errorf("expected mismatch rate=1.0, got %f", rate)
	}
}

// TestFingerprintTracker_RecordResponse_Hit 验证预期命中且实际命中 → actualHits++，mismatchCount=0。
func TestFingerprintTracker_RecordResponse_Hit(t *testing.T) {
	f := NewFingerprintTracker()

	prefix := "stable-prefix"
	f.RecordRequest(prefix) // 第一次：建立指纹
	f.RecordRequest(prefix) // 第二次：预期命中

	// 响应：cached>0 且预期命中 → 实际命中
	f.RecordResponse(500)

	_, _, actual, mismatch := f.Stats()
	if actual != 1 {
		t.Errorf("expected actualHits=1, got %d", actual)
	}
	if mismatch != 0 {
		t.Errorf("expected mismatchCount=0, got %d", mismatch)
	}

	// 不匹配率应为 0
	if rate := f.MismatchRate(); rate != 0 {
		t.Errorf("expected mismatch rate=0, got %f", rate)
	}
}

// TestFingerprintTracker_RecordResponse_NoExpectation 验证：
// 指纹变化（不预期命中）时，即使 cached=0 也不计为 mismatch。
func TestFingerprintTracker_RecordResponse_NoExpectation(t *testing.T) {
	f := NewFingerprintTracker()

	f.RecordRequest("prefix-A") // 第一次：建立指纹
	f.RecordRequest("prefix-B") // 第二次：指纹变化 → 不预期命中

	// 响应：cached=0 但不预期命中 → 不算 mismatch
	f.RecordResponse(0)

	_, expected, _, mismatch := f.Stats()
	if expected != 0 {
		t.Errorf("expected expectedHits=0, got %d", expected)
	}
	if mismatch != 0 {
		t.Errorf("expected mismatchCount=0 (no expectation), got %d", mismatch)
	}
}

// TestFingerprintTracker_Stats 综合统计：混合多次请求与响应，验证最终计数。
// 注意：RecordResponse 读取的是最近一次 RecordRequest 设置的 lastHitExpected，
// 因此必须按 "请求→响应" 交替执行（与生产用法一致），不能批量调用。
func TestFingerprintTracker_Stats(t *testing.T) {
	f := NewFingerprintTracker()

	// req1: "p1"（首次，不预期）→ cached=0，不算 mismatch
	f.RecordRequest("p1") // total=1, expected=0, lastHitExpected=false
	f.RecordResponse(0)   // no expectation → no mismatch

	// req2: "p1"（指纹相同，预期命中）→ cached=0，mismatch
	f.RecordRequest("p1") // total=2, expected=1, lastHitExpected=true
	f.RecordResponse(0)   // mismatch=1

	// req3: "p1"（指纹相同，预期命中）→ cached=100，actual hit
	f.RecordRequest("p1") // total=3, expected=2, lastHitExpected=true
	f.RecordResponse(100) // actual=1, no mismatch

	// req4: "p2"（指纹变化，不预期）→ cached=0，不算 mismatch
	f.RecordRequest("p2") // total=4, expected=2, lastHitExpected=false
	f.RecordResponse(0)   // no expectation → no mismatch

	// req5: "p2"（指纹相同，预期命中）→ cached=0，mismatch
	f.RecordRequest("p2") // total=5, expected=3, lastHitExpected=true
	f.RecordResponse(0)   // mismatch=2

	total, expected, actual, mismatch := f.Stats()
	if total != 5 {
		t.Errorf("expected totalRequests=5, got %d", total)
	}
	if expected != 3 {
		t.Errorf("expected expectedHits=3, got %d", expected)
	}
	if actual != 1 {
		t.Errorf("expected actualHits=1, got %d", actual)
	}
	if mismatch != 2 {
		t.Errorf("expected mismatchCount=2, got %d", mismatch)
	}

	// mismatch rate = 2/3 ≈ 0.667
	if rate := f.MismatchRate(); rate < 0.66 || rate > 0.67 {
		t.Errorf("expected mismatch rate≈0.667, got %f", rate)
	}
}

// TestFingerprintTracker_MismatchRate_ZeroExpected 验证 expectedHits=0 时不除零。
func TestFingerprintTracker_MismatchRate_ZeroExpected(t *testing.T) {
	f := NewFingerprintTracker()
	// 仅一次请求，无预期命中
	f.RecordRequest("only-once")

	if rate := f.MismatchRate(); rate != 0 {
		t.Errorf("expected mismatch rate=0 when expectedHits=0, got %f", rate)
	}
}

// TestFingerprintTracker_String 验证 String() 输出包含关键字段。
func TestFingerprintTracker_String(t *testing.T) {
	f := NewFingerprintTracker()
	f.RecordRequest("p1")
	f.RecordRequest("p1")
	f.RecordResponse(0) // mismatch

	s := f.String()
	for _, key := range []string{"total=", "expected=", "actual=", "mismatch=", "rate="} {
		if !strings.Contains(s, key) {
			t.Errorf("String() output missing %q: %s", key, s)
		}
	}
}

// TestFingerprintTracker_Concurrent 验证并发安全：多 goroutine 同时记录不 panic、不丢计数。
func TestFingerprintTracker_Concurrent(t *testing.T) {
	f := NewFingerprintTracker()

	done := make(chan struct{})
	const goroutines = 10
	const iters = 50

	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < iters; j++ {
				f.RecordRequest("shared-prefix")
				f.RecordResponse(0)
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}

	total, _, _, _ := f.Stats()
	if total != int64(goroutines*iters) {
		t.Errorf("expected totalRequests=%d, got %d", goroutines*iters, total)
	}
}
