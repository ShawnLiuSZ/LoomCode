package tool

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// RepairPipeline 工具调用修复流水线
type RepairPipeline struct {
	steps []RepairStep
}

// RepairStep 修复步骤
type RepairStep interface {
	Name() string
	Repair(reasoning string, toolCallsJSON string) ([]RepairedCall, error)
}

// RepairedCall 修复后的工具调用
type RepairedCall struct {
	Name string
	Args map[string]any
}

// NewRepairPipeline 创建修复流水线
func NewRepairPipeline() *RepairPipeline {
	return &RepairPipeline{
		steps: []RepairStep{
			&FlattenStep{},
			&ScavengeStep{},
			&TruncationStep{},
		},
	}
}

// Repair 执行修复流水线
func (rp *RepairPipeline) Repair(reasoningContent string, rawJSON string) ([]RepairedCall, error) {
	// 首先尝试直接解析
	calls, err := parseToolCalls(rawJSON)
	if err == nil && len(calls) > 0 {
		return calls, nil
	}

	// 逐步骤修复
	for _, step := range rp.steps {
		calls, err = step.Repair(reasoningContent, rawJSON)
		if err == nil && len(calls) > 0 {
			return calls, nil
		}
	}

	return nil, fmt.Errorf("all repair steps failed")
}

// parseToolCalls 解析 tool_calls JSON 数组
func parseToolCalls(raw string) ([]RepairedCall, error) {
	var calls []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}

	if err := json.Unmarshal([]byte(raw), &calls); err != nil {
		return nil, err
	}

	var result []RepairedCall
	for _, c := range calls {
		var args map[string]any
		if err := json.Unmarshal([]byte(c.Function.Arguments), &args); err != nil {
			args = nil
		}
		result = append(result, RepairedCall{
			Name: c.Function.Name,
			Args: args,
		})
	}

	return result, nil
}

// FlattenStep flatten 修复：处理嵌套过深的参数
type FlattenStep struct{}

func (s *FlattenStep) Name() string { return "flatten" }

func (s *FlattenStep) Repair(reasoning string, raw string) ([]RepairedCall, error) {
	// 检测参数是否使用了点号扁平表示法
	// 例如: {"file.path": "/tmp/test"} → {"file": {"path": "/tmp/test"}}
	var flat map[string]any
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, err
	}

	// 检查是否有嵌套键（包含 "."）
	hasNested := false
	for key := range flat {
		if strings.Contains(key, ".") {
			hasNested = true
			break
		}
	}

	if !hasNested {
		return nil, fmt.Errorf("no nested keys to flatten")
	}

	// 还原嵌套结构
	unnested := make(map[string]any)
	for key, val := range flat {
		parts := strings.Split(key, ".")
		current := unnested
		for i, part := range parts {
			if i == len(parts)-1 {
				// H17 修复：若中间节点已存在同名叶子值（如同时出现 "a.b" 与 "a.b.c"），
				// 直接覆盖会丢失已展开的子结构；这里仅在尚未存在时赋值，保留已展开子树。
				if _, exists := current[part]; !exists {
					current[part] = val
				}
			} else {
				// 进入（或创建）下一层 map。若已存在的值不是 map（键冲突），
				// 用新 map 包裹，避免 current[part].(map[string]any) 类型断言 panic。
				next, ok := current[part].(map[string]any)
				if !ok {
					next = make(map[string]any)
					current[part] = next
				}
				current = next
			}
		}
	}

	return []RepairedCall{{Args: unnested}}, nil
}

// ScavengeStep scavenge 修复：从 reasoning_content 回收遗漏的工具调用
type ScavengeStep struct{}

func (s *ScavengeStep) Name() string { return "scavenge" }

func (s *ScavengeStep) Repair(reasoning string, raw string) ([]RepairedCall, error) {
	if reasoning == "" {
		return nil, fmt.Errorf("no reasoning content")
	}

	// 从推理内容中提取 tool_calls JSON
	re := regexp.MustCompile(`"name"\s*:\s*"(\w+)"`)
	matches := re.FindStringSubmatch(reasoning)
	if len(matches) < 2 {
		return nil, fmt.Errorf("no tool call found in reasoning")
	}

	toolName := matches[1]

	// 尝试提取 arguments
	argRe := regexp.MustCompile(`"arguments"\s*:\s*"({[^}]+})"`)
	argMatches := argRe.FindStringSubmatch(reasoning)

	var args map[string]any
	if len(argMatches) >= 2 {
		if err := json.Unmarshal([]byte(argMatches[1]), &args); err != nil {
			args = nil
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	return []RepairedCall{{Name: toolName, Args: args}}, nil
}

// TruncationStep truncation 修复：补全截断的 JSON
type TruncationStep struct{}

func (s *TruncationStep) Name() string { return "truncation" }

func (s *TruncationStep) Repair(reasoning string, raw string) ([]RepairedCall, error) {
	raw = strings.TrimSpace(raw)

	// 尝试直接解析
	if _, err := parseToolCalls(raw); err == nil {
		return parseToolCalls(raw)
	}

	// 检测不平衡的括号/引号
	openBraces := strings.Count(raw, "{")
	closeBraces := strings.Count(raw, "}")

	if openBraces > closeBraces {
		// 补全缺失的 }
		raw += strings.Repeat("}", openBraces-closeBraces)
	}

	openBrackets := strings.Count(raw, "[")
	closeBrackets := strings.Count(raw, "]")
	if openBrackets > closeBrackets {
		raw += strings.Repeat("]", openBrackets-closeBrackets)
	}

	// 补全未闭合的引号
	if strings.Count(raw, "\"")%2 != 0 {
		raw += "\""
	}

	return parseToolCalls(raw)
}
