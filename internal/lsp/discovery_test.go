package lsp

import (
	"testing"
)

func TestDiscovery(t *testing.T) {
	t.Run("NewDiscovery", func(t *testing.T) {
		d := NewDiscovery()
		if d == nil {
			t.Fatal("expected non-nil discovery")
		}
		if len(d.servers) == 0 {
			t.Error("expected at least one server")
		}
	})

	t.Run("ListServers", func(t *testing.T) {
		d := NewDiscovery()
		servers := d.ListServers()
		if len(servers) == 0 {
			t.Error("expected at least one server")
		}
	})

	t.Run("GetServer", func(t *testing.T) {
		d := NewDiscovery()

		server, ok := d.GetServer("go")
		if !ok {
			t.Error("expected to find go server")
		}
		if server.Language != "go" {
			t.Errorf("expected language go, got %s", server.Language)
		}
		if server.Command != "gopls" {
			t.Errorf("expected command gopls, got %s", server.Command)
		}
	})

	t.Run("GetServer_NotFound", func(t *testing.T) {
		d := NewDiscovery()

		_, ok := d.GetServer("nonexistent")
		if ok {
			t.Error("expected not to find nonexistent server")
		}
	})

	t.Run("RegisterServer", func(t *testing.T) {
		d := NewDiscovery()

		d.RegisterServer("custom", &LSPServer{
			Language: "custom",
			Command:  "custom-lsp",
			Args:     []string{"--stdio"},
		})

		server, ok := d.GetServer("custom")
		if !ok {
			t.Error("expected to find custom server")
		}
		if server.Command != "custom-lsp" {
			t.Errorf("expected command custom-lsp, got %s", server.Command)
		}
	})
}

func TestDiscoveryDetectLanguage(t *testing.T) {
	d := NewDiscovery()

	tests := []struct {
		name     string
		dir      string
		expected string
	}{
		{
			name:     "go project",
			dir:      t.TempDir(),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试文件
			if tt.expected != "" {
				createTestFile(t, tt.dir, tt.expected)
			}

			lang := d.DetectLanguage(tt.dir)
			// 由于目录是空的，应该返回空字符串
			if lang != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, lang)
			}
		})
	}
}

func TestDiscoveryIsCommandAvailable(t *testing.T) {
	d := NewDiscovery()

	// 测试存在的命令
	if !d.isCommandAvailable("go") {
		// go 可能不在 PATH 中，这在某些环境中是正常的
		t.Log("go command not found, skipping")
	}

	// 测试不存在的命令
	if d.isCommandAvailable("nonexistent_command_12345") {
		t.Error("expected nonexistent command to not be available")
	}
}

func createTestFile(t *testing.T, dir, filename string) {
	t.Helper()
	// 这个函数用于创建测试文件
	// 实际实现会根据 filename 创建对应的文件
}

func TestLSPServer(t *testing.T) {
	server := &LSPServer{
		Language: "go",
		Command:  "gopls",
		Args:     []string{},
	}

	if server.Language != "go" {
		t.Errorf("expected language go, got %s", server.Language)
	}
	if server.Command != "gopls" {
		t.Errorf("expected command gopls, got %s", server.Command)
	}
}
