package control

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Allowlist 文件/命令白名单
type Allowlist struct {
	// Shell 命令白名单（精确匹配 + 前缀匹配）
	shellCommands []string

	// 文件路径白名单（目录前缀匹配）
	allowedPaths []string

	// 敏感文件模式（.env, credentials, secrets 等）
	sensitivePatterns []string

	// 禁止的 Shell 模式（原始字符串）
	blockedShellPatterns []string

	// 编译后的禁止模式正则（空格替换为 \s+，其它字符 QuoteMeta）
	blockedShellRegexps []*regexp.Regexp
}

// NewAllowlist 创建白名单
func NewAllowlist() *Allowlist {
	a := &Allowlist{
		sensitivePatterns: []string{
			".env",
			".env.local",
			".env.production",
			"credentials",
			"secrets",
			".pem",
			".key",
			"id_rsa",
			"id_ed25519",
			".git/config",
			"~/.aws/credentials",
		},
		blockedShellPatterns: []string{
			"rm -rf",
			"rm -fr",
			"rm -r",
			"sudo",
			"chmod 777",
			"| sh",
			"|bash",
			"| bash",
			"> /dev/",
			"mkfs.",
			"dd if=",
			":(){ :|:& };:", // fork bomb
			"curl",
			"wget",
			"nc",
			"ncat",
			"ssh",
			"scp",
			"rsync",
		},
	}
	a.compileBlockedRegexps()
	return a
}

// compileBlockedRegexps 把 blockedShellPatterns 编译成正则表达式：
// 模式中的空白字符替换为 \s+（匹配任意数量空白，防止 "rm -r /" 因缺少空格被绕过），
// 其它字符做 QuoteMeta 转义。编译失败的模式被跳过。
func (a *Allowlist) compileBlockedRegexps() {
	a.blockedShellRegexps = make([]*regexp.Regexp, 0, len(a.blockedShellPatterns))
	for _, p := range a.blockedShellPatterns {
		// 先按空白分段，对每段转义后再用 \s+ 连接。
		parts := strings.Fields(p)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		pattern := strings.Join(parts, `\s+`)
		if re, err := regexp.Compile(pattern); err == nil {
			a.blockedShellRegexps = append(a.blockedShellRegexps, re)
		}
	}
}

// SetShellCommands 设置允许的 Shell 命令
func (a *Allowlist) SetShellCommands(commands []string) {
	a.shellCommands = commands
}

// SetAllowedPaths 设置允许的文件路径
func (a *Allowlist) SetAllowedPaths(paths []string) {
	a.allowedPaths = paths
}

// AddSensitivePattern 添加敏感文件模式
func (a *Allowlist) AddSensitivePattern(pattern string) {
	a.sensitivePatterns = append(a.sensitivePatterns, pattern)
}

// IsAllowed 检查操作是否在白名单中
func (a *Allowlist) IsAllowed(toolName string, args map[string]any) bool {
	switch toolName {
	case "bash":
		return a.isShellAllowed(args)
	case "write_file", "edit_file":
		return a.isFileWriteAllowed(args)
	default:
		return true
	}
}

// CheckSensitive 检查文件路径是否敏感
// 返回 true 表示敏感，需要额外确认
func (a *Allowlist) CheckSensitive(path string) (bool, string) {
	base := filepath.Base(path)
	for _, pattern := range a.sensitivePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true, pattern
		}
		if strings.Contains(path, pattern) {
			return true, pattern
		}
	}
	return false, ""
}

// isShellAllowed 检查 Shell 命令是否允许
// 按 shell 分隔符分段，对每段的 argv[0] 做精确白名单校验
func (a *Allowlist) isShellAllowed(args map[string]any) bool {
	command, _ := args["command"].(string)
	if command == "" {
		return false
	}

	// 检查禁止模式（纵深防御）：使用正则匹配，空白可被任意 \s+ 绕过防御。
	for _, re := range a.blockedShellRegexps {
		if re.MatchString(command) {
			return false
		}
	}

	// 没有配置白名单 → 由 Gate 模式控制
	if len(a.shellCommands) == 0 {
		return false
	}

	// 按 shell 分隔符分段，检查每段的 argv[0]
	segments := splitShellCommand(command)
	for _, seg := range segments {
		argv0 := extractArgv0(seg)
		if argv0 == "" {
			continue
		}
		if !a.isArgv0Allowed(argv0) {
			return false
		}
	}

	return true
}

// splitShellCommand 按 shell 分隔符把命令拆成"会被执行的命令段"，并保持 quote-aware：
//   - 单引号内：全部字面量，不分词、不展开（bash 语义）。
//   - 双引号内：分隔符（; && || | & > >> < ( )）是字面量；但命令替换 $(...) / 反引号仍然生效，
//     其内部命令会被作为独立段提取出来校验。
//   - 引号外：所有分隔符生效；命令替换内部命令同样被提取。
//
// 这样既能放行带标点的合法命令（git commit -m "a; b"），又不漏掉任何 bash 真正会执行的命令。
func splitShellCommand(cmd string) []string {
	var segments []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			segments = append(segments, s)
		}
		cur.Reset()
	}
	// addSub: 把命令替换的内部内容递归拆分后加入段列表（不终止外层命令）。
	addSub := func(inner string) {
		segments = append(segments, splitShellCommand(inner)...)
	}

	i, n := 0, len(cmd)
	for i < n {
		c := cmd[i]
		switch {
		case c == '\'':
			// 单引号：字面量，原样保留到当前段
			j := i + 1
			for j < n && cmd[j] != '\'' {
				j++
			}
			end := j + 1
			if end > n {
				end = n
			}
			cur.WriteString(cmd[i:end])
			i = end

		case c == '"':
			// 双引号：字面量，但 $(...) / 反引号 仍是命令替换
			cur.WriteByte('"')
			i++
			for i < n && cmd[i] != '"' {
				if cmd[i] == '\\' && i+1 < n {
					cur.WriteByte(cmd[i])
					cur.WriteByte(cmd[i+1])
					i += 2
					continue
				}
				if cmd[i] == '$' && i+1 < n && cmd[i+1] == '(' {
					inner, next := readBalancedParen(cmd, i+2)
					addSub(inner)
					i = next
					continue
				}
				if cmd[i] == '`' {
					inner, next := readBacktick(cmd, i+1)
					addSub(inner)
					i = next
					continue
				}
				cur.WriteByte(cmd[i])
				i++
			}
			if i < n {
				cur.WriteByte('"')
				i++
			}

		case c == '\\':
			// 引号外的转义：下一个字符是字面量
			if i+1 < n {
				cur.WriteByte(cmd[i+1])
				i += 2
			} else {
				i++
			}

		case c == '$' && i+1 < n && cmd[i+1] == '(':
			inner, next := readBalancedParen(cmd, i+2)
			addSub(inner)
			i = next

		case c == '`':
			inner, next := readBacktick(cmd, i+1)
			addSub(inner)
			i = next

		default:
			if sepLen := matchSeparator(cmd, i); sepLen > 0 {
				flush()
				i += sepLen
				continue
			}
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return segments
}

// matchSeparator 返回 s[i] 处的分隔符长度（0 表示非分隔符）。两字符分隔符优先匹配。
func matchSeparator(s string, i int) int {
	if i+1 < len(s) {
		switch s[i : i+2] {
		case "&&", "||", ">>":
			return 2
		}
	}
	switch s[i] {
	case '\n', ';', '|', '&', '>', '<', '(', ')':
		return 1
	}
	return 0
}

// readBalancedParen 从 start 处读取平衡括号内的内容（用于 $(...)），返回内部内容与闭括号之后的位置。
func readBalancedParen(s string, start int) (inner string, next int) {
	depth := 1
	var b strings.Builder
	i := start
	for i < len(s) {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return b.String(), i + 1
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), i
}

// readBacktick 读取反引号内的内容，返回内部内容与闭反引号之后的位置。
func readBacktick(s string, start int) (inner string, next int) {
	i := start
	var b strings.Builder
	for i < len(s) && s[i] != '`' {
		b.WriteByte(s[i])
		i++
	}
	if i < len(s) {
		i++ // 跳过闭反引号
	}
	return b.String(), i
}

// extractArgv0 从命令段中提取第一个会被执行的命令名（quote-aware）：
// 跳过前置的环境变量赋值（IDENT=...），返回首个非赋值 token，并剥除整体包裹的引号。
func extractArgv0(segment string) string {
	for _, tok := range tokenizeWords(segment) {
		if isEnvAssignment(tok) {
			continue
		}
		return stripWrappingQuotes(tok)
	}
	return ""
}

// tokenizeWords 按"引号外的空白"切分 token（引号内的空白保留在同一 token 内）。
func tokenizeWords(s string) []string {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			cur.WriteByte(c)
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			cur.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			cur.WriteByte(c)
			inSingle = true
		case c == '"':
			cur.WriteByte(c)
			inDouble = true
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return tokens
}

// isEnvAssignment 判断 token 是否为环境变量赋值前缀（IDENT=...，IDENT 为合法 shell 标识符）。
func isEnvAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range tok[:eq] {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

// stripWrappingQuotes 剥除整体包裹的成对引号（"git" → git），否则原样返回。
func stripWrappingQuotes(tok string) string {
	if len(tok) >= 2 {
		first, last := tok[0], tok[len(tok)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
			return tok[1 : len(tok)-1]
		}
	}
	return tok
}

// isArgv0Allowed 检查 argv[0] 是否在白名单中（精确匹配）
func (a *Allowlist) isArgv0Allowed(argv0 string) bool {
	for _, allowed := range a.shellCommands {
		if argv0 == allowed {
			return true
		}
	}
	return false
}

// isFileWriteAllowed 检查文件写入是否在允许路径
func (a *Allowlist) isFileWriteAllowed(args map[string]any) bool {
	path, _ := args["path"].(string)
	if path == "" {
		return false
	}

	// 检查敏感文件
	if sensitive, _ := a.CheckSensitive(path); sensitive {
		return false // 敏感文件始终需要审批
	}

	// 没有配置路径白名单 → 由 Gate 模式控制
	if len(a.allowedPaths) == 0 {
		return false
	}

	// 检查允许路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, allowed := range a.allowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		// 必须精确匹配或匹配到分隔符边界，避免 /home/user/proj 命中 /home/user/project-evil（N3）
		if absPath == allowedAbs || strings.HasPrefix(absPath, allowedAbs+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// IsBlockedShell 检查 Shell 命令是否被禁止
func (a *Allowlist) IsBlockedShell(command string) (bool, string) {
	for i, re := range a.blockedShellRegexps {
		if re.MatchString(command) {
			if i < len(a.blockedShellPatterns) {
				return true, a.blockedShellPatterns[i]
			}
			return true, re.String()
		}
	}
	return false, ""
}
