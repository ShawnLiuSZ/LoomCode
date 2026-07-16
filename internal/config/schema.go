package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GenerateJSONSchema 返回描述 loomcode.json 配置结构的 JSON Schema（Draft 7）字符串。
// 编辑器可据此提供自动补全与校验。description 字段源自 Go 结构体注释。
func GenerateJSONSchema() string {
	schema := map[string]interface{}{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "https://github.com/ShawnLiuSZ/loomcode/schema/loomcode.json",
		"title":       "LoomCode CLI Config",
		"description": "LoomCode CLI 顶层配置结构（loomcode.json）",
		"type":        "object",
		"properties": map[string]interface{}{
			"default_provider": map[string]interface{}{
				"type":        "string",
				"description": "默认 Provider 名称（可选，不填则使用第一个 provider）",
			},
			"providers": map[string]interface{}{
				"type":        "array",
				"description": "Provider 定义列表",
				"items": map[string]interface{}{
					"type":        "object",
					"description": "单个 Provider 配置",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Provider 唯一标识名",
						},
						"display_name": map[string]interface{}{
							"type":        "string",
							"description": "Provider 展示名称",
						},
						"kind": map[string]interface{}{
							"type":        "string",
							"description": "Provider 类型（deepseek / mimo / openai）",
							"enum":        []string{"deepseek", "mimo", "openai"},
						},
						"base_url": map[string]interface{}{
							"type":        "string",
							"description": "Provider API 基础 URL",
							"format":      "uri",
						},
						"api_key": map[string]interface{}{
							"type":        "string",
							"description": "直接填写 API Key（明文，优先级高于 api_key_env）",
						},
						"api_key_env": map[string]interface{}{
							"type":        "string",
							"description": "存放 API Key 的环境变量名",
						},
						"auth_method": map[string]interface{}{
							"type":        "string",
							"description": "鉴权方式（可选）",
						},
						"default_model": map[string]interface{}{
							"type":        "string",
							"description": "该 Provider 默认使用的模型 ID",
						},
						"models": map[string]interface{}{
							"type":        "array",
							"description": "该 Provider 支持的模型列表",
							"items": map[string]interface{}{
								"type":        "object",
								"description": "单个模型配置",
								"properties": map[string]interface{}{
									"id": map[string]interface{}{
										"type":        "string",
										"description": "模型唯一标识",
									},
									"name": map[string]interface{}{
										"type":        "string",
										"description": "模型展示名称",
									},
									"cost": map[string]interface{}{
										"type":        "object",
										"description": "成本配置（每百万 token 单价）",
										"properties": map[string]interface{}{
											"input": map[string]interface{}{
												"type":        "number",
												"description": "输入 token 单价",
												"minimum":     0,
											},
											"cached_input": map[string]interface{}{
												"type":        "number",
												"description": "缓存命中的输入 token 单价",
												"minimum":     0,
											},
											"output": map[string]interface{}{
												"type":        "number",
												"description": "输出 token 单价",
												"minimum":     0,
											},
										},
									},
									"context_window": map[string]interface{}{
										"type":        "integer",
										"description": "上下文窗口大小（token 数）",
										"minimum":     0,
									},
									"capabilities": map[string]interface{}{
										"type":        "object",
										"description": "模型能力配置",
										"properties": map[string]interface{}{
											"reasoning": map[string]interface{}{
												"type":        "boolean",
												"description": "是否支持推理",
											},
											"tool_call": map[string]interface{}{
												"type":        "boolean",
												"description": "是否支持工具调用",
											},
											"prefix_cache": map[string]interface{}{
												"type":        "boolean",
												"description": "是否支持前缀缓存",
											},
											"vision": map[string]interface{}{
												"type":        "boolean",
												"description": "是否支持视觉（图像）输入",
											},
											"voice": map[string]interface{}{
												"type":        "boolean",
												"description": "是否支持语音输入",
											},
										},
									},
								},
								"required": []string{"id"},
							},
						},
					},
					"required": []string{"name", "kind", "base_url"},
				},
			},
			"plugins": map[string]interface{}{
				"type":        "array",
				"description": "MCP 插件配置列表。command 非空走 stdio，url 非空走 HTTP/SSE（url 优先）",
				"items": map[string]interface{}{
					"type":        "object",
					"description": "MCP 插件配置",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "插件唯一标识名",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "stdio 传输时的可执行命令",
						},
						"args": map[string]interface{}{
							"type":        "array",
							"description": "传给 command 的参数列表",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"env": map[string]interface{}{
							"type":        "array",
							"description": "子进程环境变量（KEY=VALUE 形式）",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"url": map[string]interface{}{
							"type":        "string",
							"description": "HTTP/SSE 传输时的插件 URL",
							"format":      "uri",
						},
					},
					"required": []string{"name"},
				},
			},
			"permissions": map[string]interface{}{
				"type":        "object",
				"description": "权限配置",
				"properties": map[string]interface{}{
					"shell_allowlist": map[string]interface{}{
						"type":        "array",
						"description": "允许执行的 shell 命令白名单",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			"search": map[string]interface{}{
				"type":        "object",
				"description": "搜索配置",
				"properties": map[string]interface{}{
					"engine": map[string]interface{}{
						"type":        "string",
						"description": "搜索引擎（bing / baidu / searxng / tavily / perplexity）",
					},
				},
			},
			"experimental": map[string]interface{}{
				"type":        "object",
				"description": "实验性功能开关",
				"properties": map[string]interface{}{
					"maxMode": map[string]interface{}{
						"type":        "boolean",
						"description": "是否启用 maxMode",
					},
					"batchTool": map[string]interface{}{
						"type":        "boolean",
						"description": "是否启用 batchTool",
					},
				},
			},
			"agent": map[string]interface{}{
				"type":        "object",
				"description": "Agent 层配置（planner/executor 分离 session 等）",
				"properties": map[string]interface{}{
					"planner_model": map[string]interface{}{
						"type":        "string",
						"description": "规划器模型（非空时启用 planner/executor 分离 session 架构；空时退化为单 session 模式）",
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		// 不会发生：schema 仅由基本类型构成，且无循环引用。
		return "{}"
	}
	return string(data)
}

// WriteSchemaFile writes the JSON Schema to ~/.loomcode/schema.json.
// Returns the path written.
func WriteSchemaFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".loomcode")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(path, []byte(GenerateJSONSchema()), 0644); err != nil {
		return "", err
	}
	return path, nil
}
