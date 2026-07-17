# Task-11: feat: support env var expansion in api_key config field

## 原始需求（只读 - 请勿修改）

> 以下内容由 GitHub Action 从 Issue #11 自动提取

## Problem

Other AI agent CLIs (e.g. Claude Code, Cursor) support referencing environment variables in config using `${ENV_VAR}` syntax. Currently LoomCode requires a separate `api_key_env` field to specify the env var name.

Example from other tools:
```json
{
  "api_key": "${DEEPSEEK_API_KEY}"
}
```

This is more intuitive and consistent with how developers expect env var references to work.

## Current Behavior

LoomCode uses two separate fields:
```json
{
  "api_key": "sk-xxx",
  "api_key_env": "DEEPSEEK_API_KEY"
}
```

## Proposed Behavior

Support `${ENV_VAR}` syntax in `api_key` field, and simplify to a single field:
```json
{
  "api_key": "${DEEPSEEK_API_KEY}"
}
```

If `api_key` contains `${...}`, expand it from environment variables. If it's a direct value, use as-is. This also means `api_key_env` becomes optional/deprecated.

## Benefits

1. **Consistency** — matches other agent CLI tools
2. **Simplicity** — one field instead of two
3. **Familiarity** — developers already know `${ENV_VAR}` syntax
4. **Backward compatible** — existing configs with `api_key_env` continue to work

## Implementation Notes

- In `resolveAPIKeys()`, check if `api_key` matches `${...}` pattern
- If so, extract env var name and resolve from `Env` map → system env
- `api_key_env` remains as fallback for backward compatibility

---

## 进度核对表

- [ ] 需求解析完成
- [ ] 技术方案确认
- [ ] 核心功能开发
- [ ] 单元测试
- [ ] 自测通过
- [ ] PR 已创建
- [ ] Code Review 通过

## 变更日志与决策记录 (ADR)

<!-- 每次技术转向或重要决策，按以下格式记录 -->

## AI 开发过程（过程文档）

<!-- 记录与 AI 协作的关键过程 -->

### 需求解析

{待填写}

### 开发记录

{待填写}

## 知识沉淀（知识文档）

<!-- 开发中发现的可复用知识 -->

## 测试验收（验收文档）

<!-- PR 合并前填写 -->

| 验收项 | 预期结果 | 实际结果 | 状态 |
|--------|---------|---------|------|
|        |         |         |      |
