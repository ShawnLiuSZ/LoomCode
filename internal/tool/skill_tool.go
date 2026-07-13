package tool

import (
	"context"
	"fmt"
	"strings"
)

// SkillInfo 一个 skill 的元信息（用于列举）。
type SkillInfo struct {
	Name        string
	Description string
}

// SkillTool 让模型按需加载某个 skill 的完整说明（渐进式披露）。
// 系统提示里只放 skill 的名称+简介，模型决定需要时再用本工具拉取全文，
// 避免一次性把所有 skill 正文塞进上下文。
type SkillTool struct {
	listFn func() []SkillInfo
	loadFn func(name string) (string, error)
}

// NewSkillTool 创建 SkillTool。listFn 返回可用 skill 列表，loadFn 按名加载正文。
func NewSkillTool(listFn func() []SkillInfo, loadFn func(name string) (string, error)) *SkillTool {
	return &SkillTool{listFn: listFn, loadFn: loadFn}
}

func (t *SkillTool) Name() string     { return "skill" }
func (t *SkillTool) IsReadOnly() bool { return true }
func (t *SkillTool) Description() string {
	return "Load the full instructions of an available skill by name. Call with no name to list available skills."
}

func (t *SkillTool) Schema() Schema {
	return Schema{
		Type: "object",
		Properties: map[string]Property{
			"name": {Type: "string", Description: "The skill name to load. Omit to list all available skills."},
		},
	}
}

func (t *SkillTool) Execute(ctx context.Context, args map[string]any) (*Result, error) {
	name, _ := args["name"].(string)
	name = strings.TrimSpace(name)

	if name == "" {
		var sb strings.Builder
		sb.WriteString("Available skills (call `skill` with a name to load full instructions):\n")
		for _, s := range t.listFn() {
			fmt.Fprintf(&sb, "- %s: %s\n", s.Name, s.Description)
		}
		return &Result{Content: sb.String()}, nil
	}

	content, err := t.loadFn(name)
	if err != nil {
		return nil, err
	}
	return &Result{Content: content}, nil
}
