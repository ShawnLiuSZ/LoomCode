# LoomCode 插件开发示例

## 插件结构

```
my-plugin/
├── plugin.json      # 插件配置
├── main.go          # 插件入口
└── README.md        # 说明文档
```

## plugin.json 格式

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "我的自定义插件",
  "author": "Your Name",
  "dependencies": [],
  "config": {
    "key": "value"
  }
}
```

## 插件接口

```go
package main

import "github.com/ShawnLiuSZ/loomcode/internal/mcp"

type MyPlugin struct {
    info   mcp.PluginInfo
    state  mcp.PluginState
}

func (p *MyPlugin) Init(info mcp.PluginInfo, config map[string]any) error {
    p.info = info
    return nil
}

func (p *MyPlugin) Start() error {
    p.state = mcp.PluginRunning
    return nil
}

func (p *MyPlugin) Stop() error {
    p.state = mcp.PluginStopped
    return nil
}

func (p *MyPlugin) GetInfo() mcp.PluginInfo {
    return p.info
}

func (p *MyPlugin) GetState() mcp.PluginState {
    return p.state
}
```

## 注册插件

```go
package main

import (
    "github.com/ShawnLiuSZ/loomcode/internal/mcp"
)

func main() {
    manager := mcp.NewPluginLifecycleManager()
    
    plugin := &MyPlugin{}
    manager.Register(plugin)
    
    // 启动插件
    manager.Start("my-plugin")
    
    // 停止插件
    manager.Stop("my-plugin")
}
```

## 生命周期钩子

```go
manager := mcp.NewPluginLifecycleManager()

// 添加启动钩子
manager.AddHook(func(plugin mcp.Plugin, event mcp.PluginLifecycle) error {
    if event == mcp.LifecycleStart {
        log.Printf("Plugin %s starting...", plugin.GetInfo().Name)
    }
    return nil
})
```

## 插件配置

```go
configManager := mcp.NewPluginConfigManager("/path/to/config.json")

// 设置配置
configManager.Set(mcp.PluginConfig{
    Name:    "my-plugin",
    Version: "1.0.0",
    Enabled: true,
    Config: map[string]any{
        "key": "value",
    },
})

// 获取配置
cfg, ok := configManager.Get("my-plugin")
```

## 更多信息

- [插件系统文档](../docs/explanation/plugin-system.md)
- [MCP 协议文档](../docs/reference/mcp-protocol.md)
