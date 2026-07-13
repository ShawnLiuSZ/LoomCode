# 会话管理

> 保存、恢复、管理对话历史

## 概述

LoomCode 支持会话持久化，允许您保存对话历史并在之后恢复。

## 会话存储位置

```
~/.loomcode/sessions/
├── session_1234567890.jsonl
├── session_1234567891.jsonl
└── ...
```

## 基本操作

### 创建会话

启动 LoomCode 时自动创建新会话：

```bash
loomcode
```

### 查看会话列表

在 TUI 中输入：

```
/sessions
```

### 恢复会话

```bash
# 启动时恢复指定会话
loomcode --session session_1234567890
```

## 会话文件格式

### JSONL 格式

每个会话存储为 JSONL 文件，第一行是元数据：

```json
{"id":"session_1234567890","name":"My Session","created_at":"2026-06-18T10:00:00Z","model":"deepseek-v4-flash","provider":"deepseek"}
{"timestamp":"2026-06-18T10:00:01Z","role":"user","content":"Hello"}
{"timestamp":"2026-06-18T10:00:02Z","role":"assistant","content":"Hi! How can I help you?"}
```

### 元数据字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 会话唯一标识 |
| `name` | string | 会话名称 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 最后更新时间 |
| `model` | string | 使用的模型 |
| `provider` | string | 使用的 Provider |

## 使用场景

### 场景 1：跨会话继续工作

```bash
# 昨天的工作
loomcode --session session_1234567890

# 继续之前的对话
```

### 场景 2：备份重要会话

```bash
# 复制会话文件
cp ~/.loomcode/sessions/session_1234567890.jsonl ~/backup/
```

### 场景 3：共享会话

```bash
# 导出会话
cat ~/.loomcode/sessions/session_1234567890.jsonl | base64 > session.txt

# 在另一台机器导入
base64 -d session.txt > ~/.loomcode/sessions/session_1234567890.jsonl
```

## 会话管理最佳实践

### 定期清理

```bash
# 删除旧会话（保留最近 30 天）
find ~/.loomcode/sessions -name "*.jsonl" -mtime +30 -delete
```

### 命名规范

使用有意义的会话名称：

```bash
# 好的命名
session_20260618_用户认证模块
session_20260618_Bug修复

# 避免的命名
session_1234567890
```

### 备份策略

```bash
# 自动备份脚本
#!/bin/bash
BACKUP_DIR=~/backups/loomcode-sessions
mkdir -p $BACKUP_DIR
tar -czf $BACKUP_DIR/sessions-$(date +%Y%m%d).tar.gz ~/.loomcode/sessions/
```

## 高级功能

### 会话元数据

查看会话详细信息：

```bash
# 查看会话文件
cat ~/.loomcode/sessions/session_1234567890.jsonl | head -1 | jq .
```

### 会话统计

```bash
# 统计会话数量
ls ~/.loomcode/sessions/*.jsonl | wc -l

# 统计总消息数
cat ~/.loomcode/sessions/*.jsonl | grep '"role"' | wc -l
```

### 会话搜索

```bash
# 搜索包含特定内容的会话
grep -l "用户认证" ~/.loomcode/sessions/*.jsonl
```

## 故障排除

### 会话文件损坏

```
Error: decode meta: invalid character
```

**解决**：
1. 检查文件格式
2. 尝试修复 JSON 格式
3. 恢复备份

### 会话未保存

**检查**：
1. 确认 `~/.loomcode/sessions/` 目录存在
2. 检查磁盘空间
3. 查看 LoomCode 日志

### 会话文件过大

```bash
# 压缩旧会话
gzip ~/.loomcode/sessions/old-session.jsonl

# 删除非常旧的会话
find ~/.loomcode/sessions -name "*.jsonl" -mtime +90 -delete
```

## 下一步

- [环境变量管理](environment-variables.md) - 配置 API 密钥
- [配置文件格式](../reference/config-format.md) - 完整语法
- [架构概览](../explanation/architecture.md) - 理解 LoomCode 设计
