package tool

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// genLines 生成 n 行内容，每行格式 "line-<i>"，行间以 "\n" 分隔。
func genLines(n int) string {
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	return strings.Join(lines, "\n")
}

// TestPruneResult_ShortContent 短内容（< maxToolResultLines）不剪枝，原样返回。
func TestPruneResult_ShortContent(t *testing.T) {
	content := genLines(100) // 100 < 200
	got := pruneResult(content, maxToolResultLines)
	if got != content {
		t.Errorf("short content should not be pruned\nwant len=%d\ngot  len=%d", len(content), len(got))
	}
}

// TestPruneResult_LongContent 长内容（> maxToolResultLines）剪枝：头部 50 行 + 尾部 50 行保留，中间被剪枝。
func TestPruneResult_LongContent(t *testing.T) {
	content := genLines(300) // 300 > 200
	got := pruneResult(content, maxToolResultLines)

	// 头部前 50 行保留
	if !strings.HasPrefix(got, "line-0") {
		t.Errorf("head should start with line-0")
	}
	if !strings.Contains(got, "line-49") {
		t.Errorf("head should contain line-49")
	}
	// 尾部后 50 行保留
	if !strings.HasSuffix(got, "line-299") {
		t.Errorf("tail should end with line-299")
	}
	if !strings.Contains(got, "line-250") {
		t.Errorf("tail should contain line-250")
	}
	// 中间被剪枝的行不应出现
	if strings.Contains(got, "line-100") {
		t.Errorf("middle line-100 should be pruned")
	}
	if strings.Contains(got, "line-200") {
		t.Errorf("middle line-200 should be pruned")
	}
}

// TestPruneResult_Deterministic 相同输入两次剪枝结果完全一致（bytes.Equal）。
func TestPruneResult_Deterministic(t *testing.T) {
	content := genLines(300)
	a := pruneResult(content, maxToolResultLines)
	b := pruneResult(content, maxToolResultLines)
	if !bytes.Equal([]byte(a), []byte(b)) {
		t.Errorf("pruning must be deterministic: two runs produced different output")
	}
}

// TestPruneResult_LineCount 剪枝后行数 = headKeepLines + 1（占位符）+ tailKeepLines。
func TestPruneResult_LineCount(t *testing.T) {
	content := genLines(300)
	got := pruneResult(content, maxToolResultLines)
	gotLines := strings.Split(got, "\n")
	want := headKeepLines + 1 + tailKeepLines // 50 + 1 + 50 = 101
	if len(gotLines) != want {
		t.Errorf("line count after prune = %d, want %d", len(gotLines), want)
	}
}

// TestPruneResult_PlaceholderFormat 占位符包含被剪枝的行数和总行数。
func TestPruneResult_PlaceholderFormat(t *testing.T) {
	total := 300
	content := genLines(total)
	got := pruneResult(content, maxToolResultLines)

	omitted := total - headKeepLines - tailKeepLines // 200
	want := fmt.Sprintf("... (pruned %d lines, %d total) ...", omitted, total)
	if !strings.Contains(got, want) {
		t.Errorf("placeholder should contain %q\ngot:\n%s", want, got)
	}
}
