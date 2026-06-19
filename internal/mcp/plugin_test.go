package mcp

import (
	"testing"
)

func TestPluginState(t *testing.T) {
	tests := []struct {
		state    PluginState
		expected string
	}{
		{PluginStopped, "stopped"},
		{PluginStarting, "starting"},
		{PluginRunning, "running"},
		{PluginError, "error"},
		{PluginState(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("PluginState.String() = %q, want %q", got, tt.expected)
		}
	}
}

// mockPlugin 模拟插件
type mockPlugin struct {
	info   PluginInfo
	state  PluginState
	started bool
}

func (p *mockPlugin) Init(info PluginInfo, config map[string]any) error {
	p.info = info
	return nil
}

func (p *mockPlugin) Start() error {
	p.started = true
	p.state = PluginRunning
	return nil
}

func (p *mockPlugin) Stop() error {
	p.started = false
	p.state = PluginStopped
	return nil
}

func (p *mockPlugin) GetInfo() PluginInfo {
	return p.info
}

func (p *mockPlugin) GetState() PluginState {
	return p.state
}

func TestPluginLifecycleManager(t *testing.T) {
	t.Run("NewPluginLifecycleManager", func(t *testing.T) {
		mgr := NewPluginLifecycleManager()
		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})

	t.Run("Register", func(t *testing.T) {
		mgr := NewPluginLifecycleManager()

		plugin := &mockPlugin{
			info: PluginInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
			},
		}

		err := mgr.Register(plugin)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Start", func(t *testing.T) {
		mgr := NewPluginLifecycleManager()

		plugin := &mockPlugin{
			info: PluginInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
			},
		}
		mgr.Register(plugin)

		err := mgr.Start("test-plugin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mgr.GetState("test-plugin") != PluginRunning {
			t.Error("expected plugin to be running")
		}
	})

	t.Run("Stop", func(t *testing.T) {
		mgr := NewPluginLifecycleManager()

		plugin := &mockPlugin{
			info: PluginInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
			},
		}
		mgr.Register(plugin)
		mgr.Start("test-plugin")

		err := mgr.Stop("test-plugin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mgr.GetState("test-plugin") != PluginStopped {
			t.Error("expected plugin to be stopped")
		}
	})
}

func TestPluginConfigManager(t *testing.T) {
	t.Run("NewPluginConfigManager", func(t *testing.T) {
		mgr := NewPluginConfigManager("/tmp/config.json")
		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})

	t.Run("SetAndGet", func(t *testing.T) {
		mgr := NewPluginConfigManager("/tmp/config.json")

		cfg := PluginConfig{
			Name:    "test-plugin",
			Version: "1.0.0",
			Enabled: true,
		}
		mgr.Set(cfg)

		got, ok := mgr.Get("test-plugin")
		if !ok {
			t.Error("expected to find config")
		}
		if got.Version != "1.0.0" {
			t.Errorf("expected 1.0.0, got %s", got.Version)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		mgr := NewPluginConfigManager("/tmp/config.json")

		mgr.Set(PluginConfig{Name: "test-plugin"})
		mgr.Delete("test-plugin")

		_, ok := mgr.Get("test-plugin")
		if ok {
			t.Error("expected config to be deleted")
		}
	})

	t.Run("List", func(t *testing.T) {
		mgr := NewPluginConfigManager("/tmp/config.json")

		mgr.Set(PluginConfig{Name: "plugin-b"})
		mgr.Set(PluginConfig{Name: "plugin-a"})

		list := mgr.List()
		if len(list) != 2 {
			t.Errorf("expected 2 configs, got %d", len(list))
		}
		if list[0].Name != "plugin-a" {
			t.Errorf("expected plugin-a first, got %s", list[0].Name)
		}
	})
}

func TestPluginInfo(t *testing.T) {
	info := PluginInfo{
		Name:         "test-plugin",
		Version:      "1.0.0",
		Description:  "A test plugin",
		Author:       "Test Author",
		Dependencies: []string{"dep1", "dep2"},
	}

	if info.Name != "test-plugin" {
		t.Errorf("expected test-plugin, got %s", info.Name)
	}
	if len(info.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(info.Dependencies))
	}
}
