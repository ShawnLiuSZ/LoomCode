package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill 表示一个 skill
type Skill struct {
	Name        string
	Path        string
	Description string
	Source      string // "helix" 或 "agents"
}

// Manager skills 管理器
type Manager struct {
	skills map[string]*Skill
}

// NewManager 创建 skills 管理器
func NewManager() *Manager {
	return &Manager{skills: make(map[string]*Skill)}
}

// Load 加载所有 skills
// 优先级：~/.helix/skills > ~/.agents/skills
func (m *Manager) Load() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// 1. 先加载 ~/.agents/skills（低优先级）
	agentsDir := filepath.Join(home, ".agents", "skills")
	m.loadFromDir(agentsDir, "agents")

	// 2. 再加载 ~/.helix/skills（高优先级，覆盖同名）
	helixDir := filepath.Join(home, ".helix", "skills")
	m.loadFromDir(helixDir, "helix")

	return nil
}

// loadFromDir 从目录加载 skills
func (m *Manager) loadFromDir(dir, source string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		name := entry.Name()
		skillPath := filepath.Join(dir, name)

		// 读取 SKILL.md 获取描述
		desc := m.readDescription(skillPath)

		// ~/.helix/skills 优先：如果已存在且当前是 agents 源，跳过
		if _, ok := m.skills[name]; ok {
			if source == "agents" {
				continue // helix 优先
			}
			// 否则覆盖（helix 覆盖 agents）
		}

		m.skills[name] = &Skill{
			Name:        name,
			Path:        skillPath,
			Description: desc,
			Source:      source,
		}
	}
}

// readDescription 从 SKILL.md 第一行读取描述
func (m *Manager) readDescription(skillPath string) string {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return ""
	}

	// 取第一行非空非 # 开头的内容
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 跳过标题行
		line = strings.TrimPrefix(line, "# ")
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// List 列出所有 skills
func (m *Manager) List() []*Skill {
	result := make([]*Skill, 0, len(m.skills))
	for _, s := range m.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Get 获取指定 skill
func (m *Manager) Get(name string) (*Skill, bool) {
	s, ok := m.skills[name]
	return s, ok
}

// Count 返回 skills 数量
func (m *Manager) Count() int {
	return len(m.skills)
}

// Content 读取 skill 的完整内容
func (s *Skill) Content() (string, error) {
	skillFile := filepath.Join(s.Path, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
