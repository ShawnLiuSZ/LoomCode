package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LSPServer LSP 服务器信息
type LSPServer struct {
	Language string
	Command  string
	Args     []string
}

// Discovery LSP 服务器发现器
type Discovery struct {
	servers map[string]*LSPServer
}

// NewDiscovery 创建 LSP 发现器
func NewDiscovery() *Discovery {
	d := &Discovery{
		servers: make(map[string]*LSPServer),
	}
	d.loadDefaults()
	return d
}

// loadDefaults 加载默认的 LSP 服务器配置
func (d *Discovery) loadDefaults() {
	// Go
	d.servers["go"] = &LSPServer{
		Language: "go",
		Command:  "gopls",
		Args:     []string{},
	}

	// TypeScript/JavaScript
	d.servers["typescript"] = &LSPServer{
		Language: "typescript",
		Command:  "typescript-language-server",
		Args:     []string{"--stdio"},
	}

	// Python
	d.servers["python"] = &LSPServer{
		Language: "python",
		Command:  "pylsp",
		Args:     []string{},
	}

	// Rust
	d.servers["rust"] = &LSPServer{
		Language: "rust",
		Command:  "rust-analyzer",
		Args:     []string{},
	}

	// Java
	d.servers["java"] = &LSPServer{
		Language: "java",
		Command:  "jdtls",
		Args:     []string{},
	}

	// C/C++
	d.servers["cpp"] = &LSPServer{
		Language: "cpp",
		Command:  "clangd",
		Args:     []string{},
	}

	// C#
	d.servers["csharp"] = &LSPServer{
		Language: "csharp",
		Command:  "omnisharp",
		Args:     []string{"--languageserver"},
	}

	// Ruby
	d.servers["ruby"] = &LSPServer{
		Language: "ruby",
		Command:  "solargraph",
		Args:     []string{"stdio"},
	}

	// PHP
	d.servers["php"] = &LSPServer{
		Language: "php",
		Command:  "intelephense",
		Args:     []string{"--stdio"},
	}

	// Kotlin
	d.servers["kotlin"] = &LSPServer{
		Language: "kotlin",
		Command:  "kotlin-language-server",
		Args:     []string{},
	}

	// Swift
	d.servers["swift"] = &LSPServer{
		Language: "swift",
		Command:  "sourcekit-lsp",
		Args:     []string{},
	}

	// Dart
	d.servers["dart"] = &LSPServer{
		Language: "dart",
		Command:  "dart",
		Args:     []string{"language-server"},
	}

	// Elixir
	d.servers["elixir"] = &LSPServer{
		Language: "elixir",
		Command:  "elixir-ls",
		Args:     []string{},
	}

	// Haskell
	d.servers["haskell"] = &LSPServer{
		Language: "haskell",
		Command:  "haskell-language-server-wrapper",
		Args:     []string{"--lsp"},
	}

	// Lua
	d.servers["lua"] = &LSPServer{
		Language: "lua",
		Command:  "lua-language-server",
		Args:     []string{},
	}

	// YAML
	d.servers["yaml"] = &LSPServer{
		Language: "yaml",
		Command:  "yaml-language-server",
		Args:     []string{"--stdio"},
	}

	// JSON
	d.servers["json"] = &LSPServer{
		Language: "json",
		Command:  "vscode-json-language-server",
		Args:     []string{"--stdio"},
	}

	// HTML
	d.servers["html"] = &LSPServer{
		Language: "html",
		Command:  "vscode-html-language-server",
		Args:     []string{"--stdio"},
	}

	// CSS
	d.servers["css"] = &LSPServer{
		Language: "css",
		Command:  "vscode-css-language-server",
		Args:     []string{"--stdio"},
	}
}

// Discover 发现指定目录的 LSP 服务器
func (d *Discovery) Discover(dir string) []*LSPServer {
	var result []*LSPServer

	// 根据文件扩展名检测语言
	langs := d.detectLanguages(dir)

	for _, lang := range langs {
		if server, ok := d.servers[lang]; ok {
			// 检查命令是否可用
			if d.isCommandAvailable(server.Command) {
				result = append(result, server)
			}
		}
	}

	return result
}

// DetectLanguage 检测目录的主要语言
func (d *Discovery) DetectLanguage(dir string) string {
	langs := d.detectLanguages(dir)
	if len(langs) > 0 {
		return langs[0]
	}
	return ""
}

// detectLanguages 检测目录中的所有语言
func (d *Discovery) detectLanguages(dir string) []string {
	langCount := make(map[string]int)

	// 文件扩展名到语言的映射
	extMap := map[string]string{
		".go":    "go",
		".mod":   "go",
		".ts":    "typescript",
		".tsx":   "typescript",
		".js":    "typescript",
		".jsx":   "typescript",
		".py":    "python",
		".rs":    "rust",
		".java":  "java",
		".c":     "cpp",
		".cpp":   "cpp",
		".h":     "cpp",
		".hpp":   "cpp",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".kt":    "kotlin",
		".swift": "swift",
		".dart":  "dart",
		".ex":    "elixir",
		".exs":   "elixir",
		".hs":    "haskell",
		".lua":   "lua",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".html":  "html",
		".css":   "css",
	}

	// 特殊文件名映射
	fileMap := map[string]string{
		"go.mod":       "go",
		"go.sum":       "go",
		"package.json": "typescript",
		"Cargo.toml":   "rust",
		"pom.xml":      "java",
		"build.gradle": "java",
		"Gemfile":      "ruby",
		"composer.json": "php",
		"pubspec.yaml":  "dart",
		"mix.exs":       "elixir",
		"cabal.yaml":    "haskell",
		".luacheckrc":   "lua",
	}

	// 遍历目录（最多 2 层）
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 限制深度
		rel, _ := filepath.Rel(dir, path)
		if strings.Count(rel, string(filepath.Separator)) > 2 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			// 跳过隐藏目录和 vendor
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查文件扩展名
		if lang, ok := extMap[filepath.Ext(path)]; ok {
			langCount[lang]++
		}

		// 检查特殊文件名
		if lang, ok := fileMap[info.Name()]; ok {
			langCount[lang]++
		}

		return nil
	})

	// 按数量排序
	var langs []string
	maxCount := 0
	for lang, count := range langCount {
		if count > maxCount {
			langs = []string{lang}
			maxCount = count
		} else if count == maxCount {
			langs = append(langs, lang)
		}
	}

	return langs
}

// isCommandAvailable 检查命令是否可用
func (d *Discovery) isCommandAvailable(command string) bool {
	// 在 Windows 上添加 .exe 后缀
	cmd := command
	if runtime.GOOS == "windows" {
		cmd += ".exe"
	}

	_, err := exec.LookPath(cmd)
	return err == nil
}

// GetServer 获取指定语言的 LSP 服务器
func (d *Discovery) GetServer(lang string) (*LSPServer, bool) {
	server, ok := d.servers[lang]
	return server, ok
}

// RegisterServer 注册自定义 LSP 服务器
func (d *Discovery) RegisterServer(lang string, server *LSPServer) {
	d.servers[lang] = server
}

// ListServers 列出所有已注册的 LSP 服务器
func (d *Discovery) ListServers() map[string]*LSPServer {
	result := make(map[string]*LSPServer)
	for k, v := range d.servers {
		result[k] = v
	}
	return result
}
