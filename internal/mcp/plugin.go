package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// PluginState 插件状态
type PluginState int

const (
	// PluginStopped 已停止
	PluginStopped PluginState = iota
	// PluginStarting 启动中
	PluginStarting
	// PluginRunning 运行中
	PluginRunning
	// PluginError 错误
	PluginError
)

// String 返回状态字符串
func (s PluginState) String() string {
	switch s {
	case PluginStopped:
		return "stopped"
	case PluginStarting:
		return "starting"
	case PluginRunning:
		return "running"
	case PluginError:
		return "error"
	default:
		return "unknown"
	}
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Dependencies []string         `json:"dependencies,omitempty"`
	Config      map[string]any    `json:"config,omitempty"`
}

// PluginLifecycle 生命周期事件
type PluginLifecycle int

const (
	// LifecycleInit 初始化
	LifecycleInit PluginLifecycle = iota
	// LifecycleStart 启动
	LifecycleStart
	// LifecycleStop 停止
	LifecycleStop
	// LifecycleDestroy 销毁
	LifecycleDestroy
)

// PluginLifecycleManager 插件生命周期管理器
type PluginLifecycleManager struct {
	mu       sync.RWMutex
	plugins  map[string]Plugin
	infos    map[string]PluginInfo
	states   map[string]PluginState
	hooks    []LifecycleHook
}

// LifecycleHook 生命周期钩子
type LifecycleHook func(plugin Plugin, event PluginLifecycle) error

// NewPluginLifecycleManager 创建插件生命周期管理器
func NewPluginLifecycleManager() *PluginLifecycleManager {
	return &PluginLifecycleManager{
		plugins: make(map[string]Plugin),
		infos:   make(map[string]PluginInfo),
		states:  make(map[string]PluginState),
		hooks:   make([]LifecycleHook, 0),
	}
}

// Register 注册插件
func (m *PluginLifecycleManager) Register(plugin Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := plugin.GetInfo()
	m.plugins[info.Name] = plugin
	m.infos[info.Name] = info
	m.states[info.Name] = PluginStopped

	return nil
}

// Start 启动插件
func (m *PluginLifecycleManager) Start(name string) error {
	m.mu.Lock()
	plugin, ok := m.plugins[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin %q not found", name)
	}
	m.states[name] = PluginStarting
	m.mu.Unlock()

	// 触发钩子
	m.triggerHooks(plugin, LifecycleStart)

	// 启动插件
	if err := plugin.Start(); err != nil {
		m.mu.Lock()
		m.states[name] = PluginError
		m.mu.Unlock()
		return err
	}

	m.mu.Lock()
	m.states[name] = PluginRunning
	m.mu.Unlock()

	return nil
}

// Stop 停止插件
func (m *PluginLifecycleManager) Stop(name string) error {
	m.mu.Lock()
	plugin, ok := m.plugins[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin %q not found", name)
	}
	m.mu.Unlock()

	if err := plugin.Stop(); err != nil {
		return err
	}

	// 触发钩子
	m.triggerHooks(plugin, LifecycleStop)

	m.mu.Lock()
	m.states[name] = PluginStopped
	m.mu.Unlock()

	return nil
}

// GetState 获取插件状态
func (m *PluginLifecycleManager) GetState(name string) PluginState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[name]
}

// AddHook 添加生命周期钩子
func (m *PluginLifecycleManager) AddHook(hook LifecycleHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook)
}

// triggerHooks 触发钩子
func (m *PluginLifecycleManager) triggerHooks(plugin Plugin, event PluginLifecycle) {
	m.mu.RLock()
	hooks := make([]LifecycleHook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.RUnlock()

	for _, hook := range hooks {
		hook(plugin, event)
	}
}

// Plugin 插件接口
type Plugin interface {
	// Init 初始化插件
	Init(info PluginInfo, config map[string]any) error
	// Start 启动插件
	Start() error
	// Stop 停止插件
	Stop() error
	// GetInfo 获取插件信息
	GetInfo() PluginInfo
	// GetState 获取插件状态
	GetState() PluginState
}

// PluginConfigManager 插件配置管理器
type PluginConfigManager struct {
	mu       sync.RWMutex
	configs  map[string]PluginConfig
	filePath string
}

// PluginConfig 插件配置
type PluginConfig struct {
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	Enabled  bool           `json:"enabled"`
	Config   map[string]any `json:"config,omitempty"`
}

// NewPluginConfigManager 创建配置管理器
func NewPluginConfigManager(filePath string) *PluginConfigManager {
	return &PluginConfigManager{
		configs:  make(map[string]PluginConfig),
		filePath: filePath,
	}
}

// Load 加载配置
func (m *PluginConfigManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var configs []PluginConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}

	for _, cfg := range configs {
		m.configs[cfg.Name] = cfg
	}

	return nil
}

// Save 保存配置
func (m *PluginConfigManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var configs []PluginConfig
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

// Get 获取配置
func (m *PluginConfigManager) Get(name string) (PluginConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[name]
	return cfg, ok
}

// Set 设置配置
func (m *PluginConfigManager) Set(cfg PluginConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[cfg.Name] = cfg
}

// Delete 删除配置
func (m *PluginConfigManager) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.configs, name)
}

// List 列出所有配置
func (m *PluginConfigManager) List() []PluginConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []PluginConfig
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}
