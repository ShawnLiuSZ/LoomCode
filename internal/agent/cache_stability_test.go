package agent

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ShawnLiuSZ/Helix/internal/testutil"
	"github.com/ShawnLiuSZ/Helix/internal/tool"
)

// TestPrefixStability 验证 buildStaticSystemPrompt 在连续两次调用间字节级一致。
// 静态 system prompt（身份 + 工作原则）位于消息列表 index 0，配合 tools 定义
// 构成 provider prefix cache 的稳定前缀。任何字节级漂移都会导致 cache miss，
// 因此这里同时断言字符串相等与字节切片相等。
func TestPrefixStability(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry() // 空 registry：最小依赖，聚焦 prefix 本身
	agent := New(p, r)

	s1 := agent.buildStaticSystemPrompt()
	s2 := agent.buildStaticSystemPrompt()

	// 字符串完全相等
	if s1 != s2 {
		t.Fatalf("buildStaticSystemPrompt() not stable: len(s1)=%d len(s2)=%d", len(s1), len(s2))
	}

	// 字节切片完全相等（防御性二次断言，避免 == 在某些场景的隐式转换歧义）
	b1 := []byte(s1)
	b2 := []byte(s2)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("buildStaticSystemPrompt() bytes not equal: %v vs %v", b1, b2)
	}

	// 防御性：空 prompt 会让断言退化成“空等于空”，失去意义
	if len(s1) == 0 {
		t.Fatal("buildStaticSystemPrompt() returned empty string; assertion is vacuous")
	}
}

// TestToolDefsOrder 验证 buildToolDefs 连续两次输出的 tools 数组顺序一致。
// tools.Registry.List() 已按名升序排序（见 registry_test.go 的
// TestRegistry_ListStableOrder），本测试聚焦 agent 层：构造 Agent 后连续两次
// 调用 buildToolDefs，断言 JSON 序列化结果完全一致，确保 provider 端看到的
// tools 数组在多次请求间稳定，从而命中 prefix cache。
func TestToolDefsOrder(t *testing.T) {
	p := testutil.NewStubProvider(nil)
	r := tool.NewRegistry()
	r.Register(&tool.ReadFileTool{})
	r.Register(&tool.GrepTool{})
	r.Register(&tool.GlobTool{})

	agent := New(p, r)

	defs1 := agent.buildToolDefs()
	defs2 := agent.buildToolDefs()

	if len(defs1) == 0 {
		t.Fatal("buildToolDefs() returned empty; assertion is vacuous")
	}
	if len(defs1) != len(defs2) {
		t.Fatalf("buildToolDefs() length drift: %d vs %d", len(defs1), len(defs2))
	}

	b1, err := json.Marshal(defs1)
	if err != nil {
		t.Fatalf("marshal defs1: %v", err)
	}
	b2, err := json.Marshal(defs2)
	if err != nil {
		t.Fatalf("marshal defs2: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Fatalf("buildToolDefs() JSON not stable between calls:\nfirst:  %s\nsecond: %s", b1, b2)
	}

	// 顺序稳定性：逐一比对工具名，确保不是“集合相等但顺序不同”
	for i := range defs1 {
		if defs1[i].Function.Name != defs2[i].Function.Name {
			t.Fatalf("tool order mismatch at index %d: %q vs %q",
				i, defs1[i].Function.Name, defs2[i].Function.Name)
		}
	}
}
