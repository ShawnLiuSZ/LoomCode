# LoomCode Dashboard 示例

## 启动 Dashboard

```bash
# 使用默认端口 (8080)
./bin/loomcode dashboard

# 指定端口
./bin/loomcode dashboard :9090
```

## 访问 Dashboard

打开浏览器访问: http://localhost:8080

## 功能说明

### 会话列表

左侧显示所有历史会话，点击可查看详细信息。

### 成本分析

显示当前会话的成本统计：
- 总成本
- 今日成本
- 成本历史图表

### Provider 状态

显示各 Provider 的连接状态：
- DeepSeek
- MiMo
- OpenAI

### 事件日志

实时显示 Agent 执行事件。

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/sessions` | GET | 获取会话列表 |
| `/api/sessions/:id` | GET | 获取会话详情 |
| `/api/cost` | GET | 获取成本统计 |
| `/api/status` | GET | 获取 Provider 状态 |
| `/ws` | WebSocket | 实时事件推送 |

## 自定义样式

Dashboard 使用 CSS 变量，可自定义主题：

```css
:root {
    --bg-primary: #1a1a2e;
    --bg-secondary: #16213e;
    --text-primary: #eee;
    --accent: #00d9ff;
}
```

## 更多信息

- [Web Dashboard 文档](../docs/explanation/web-dashboard.md)
- [API 参考](../docs/reference/api.md)
