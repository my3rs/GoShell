package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AutocompleteResult 定义 Tab 键补全的返回结果
type AutocompleteResult struct {
	NewLine      []rune   // 补全后的整行缓冲区内容
	AddedRunes   []rune   // 本次补全追加的字符 (可直接本地回显到屏幕)
	Suggestions  []string // 多重匹配时的所有候选词 (用于换行打印提示)
	HasMatch     bool     // 是否存在匹配
	IsExactMatch bool     // 是否为单一精确匹配
}

// BuiltinCommands 终端常用命令字典 (覆盖 Linux / BusyBox / Windows 核心指令)
var BuiltinCommands = []string{
	"busybox", "cat", "cd", "chmod", "chown", "clear", "cls",
	"cmd", "cp", "curl", "date", "df", "dir", "echo",
	"env", "exit", "find", "git", "grep", "head", "help",
	"history", "hostname", "id", "ip", "kill", "less", "ln",
	"ls", "mkdir", "more", "mv", "nano", "netstat", "ping",
	"powershell", "ps", "pwd", "rm", "rmdir", "sed", "sh",
	"sort", "ssh", "tail", "tar", "tee", "touch", "uname",
	"uniq", "uptime", "vi", "vim", "wc", "which", "whoami",
}

// Autocomplete 对终端当前命令行进行智能 Tab 补全计算
func Autocomplete(line []rune, currentDir string) AutocompleteResult {
	if len(line) == 0 {
		return AutocompleteResult{}
	}

	raw := string(line)
	word, wordStart := extractLastWord(raw)
	if word == "" && wordStart == len(raw) {
		// 光标在空格处且末尾为空，列出当前目录下所有文件
		return listAllDirEntries(line, currentDir)
	}

	isCmd := isFirstWord(raw, wordStart)

	var candidates []string
	if isCmd {
		candidates = matchCommands(word, currentDir)
	} else {
		candidates = matchFilesAndDirs(word, currentDir)
	}

	if len(candidates) == 0 {
		return AutocompleteResult{HasMatch: false}
	}

	// 1. 单项精确唯一匹配
	if len(candidates) == 1 {
		matched := candidates[0]
		// 计算剩余需要补齐的后缀
		var remainder string
		if len(matched) >= len(word) && strings.EqualFold(matched[:len(word)], word) {
			remainder = matched[len(word):]
		} else {
			remainder = matched
		}

		addedRunes := []rune(remainder)
		newLine := append(line, addedRunes...)

		return AutocompleteResult{
			NewLine:      newLine,
			AddedRunes:   addedRunes,
			HasMatch:     true,
			IsExactMatch: true,
		}
	}

	// 2. 多项匹配：先寻找最长公共前缀进行先行补全
	lcp := longestCommonPrefix(candidates)
	var addedRunes []rune
	newLine := line
	if len(lcp) > len(word) && strings.EqualFold(lcp[:len(word)], word) {
		remainder := lcp[len(word):]
		addedRunes = []rune(remainder)
		newLine = append(newLine, addedRunes...)
	}

	return AutocompleteResult{
		NewLine:      newLine,
		AddedRunes:   addedRunes,
		Suggestions:  candidates,
		HasMatch:     true,
		IsExactMatch: false,
	}
}

// extractLastWord 从命令行末尾提取正在输入的词及其起始下标
func extractLastWord(s string) (string, int) {
	if len(s) == 0 {
		return "", 0
	}
	i := len(s) - 1
	for i >= 0 && (s[i] == ' ' || s[i] == '\t') {
		i--
	}
	if i < 0 {
		return "", len(s)
	}
	end := i + 1
	for i >= 0 && s[i] != ' ' && s[i] != '\t' && s[i] != ';' && s[i] != '|' && s[i] != '&' {
		i--
	}
	start := i + 1
	return s[start:end], start
}

// isFirstWord 判断指定位置的词是否为命令行中的命令名
func isFirstWord(s string, start int) bool {
	prefix := strings.TrimSpace(s[:start])
	if prefix == "" {
		return true
	}
	lastChar := prefix[len(prefix)-1]
	return lastChar == ';' || lastChar == '|' || lastChar == '&'
}

// matchCommands 匹配系统与内置命令名
func matchCommands(prefix string, currentDir string) []string {
	var matches []string
	seen := make(map[string]bool)

	lowerPrefix := strings.ToLower(prefix)

	// 1. 匹配内置常用命令
	for _, cmd := range BuiltinCommands {
		if strings.HasPrefix(strings.ToLower(cmd), lowerPrefix) {
			if !seen[cmd] {
				seen[cmd] = true
				matches = append(matches, cmd+" ")
			}
		}
	}

	// 2. 匹配当前目录下的可执行文件
	if currentDir != "" {
		if entries, err := os.ReadDir(currentDir); err == nil {
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(strings.ToLower(name), lowerPrefix) {
					ext := strings.ToLower(filepath.Ext(name))
					if ext == ".exe" || ext == ".bat" || ext == ".cmd" || ext == ".sh" || e.IsDir() {
						item := name
						if e.IsDir() {
							item += "/"
						} else {
							item += " "
						}
						if !seen[item] {
							seen[item] = true
							matches = append(matches, item)
						}
					}
				}
			}
		}
	}

	sort.Strings(matches)
	return matches
}

// matchFilesAndDirs 匹配指定路径前缀的文件与文件夹
func matchFilesAndDirs(prefix string, currentDir string) []string {
	normalizedPrefix := strings.ReplaceAll(prefix, "\\", "/")
	dirPart := ""
	basePrefix := normalizedPrefix

	lastSlash := strings.LastIndex(normalizedPrefix, "/")
	if lastSlash >= 0 {
		dirPart = normalizedPrefix[:lastSlash+1]
		basePrefix = normalizedPrefix[lastSlash+1:]
	}

	targetDir := currentDir
	if dirPart != "" {
		if filepath.IsAbs(dirPart) {
			targetDir = filepath.Clean(dirPart)
		} else {
			targetDir = filepath.Join(currentDir, filepath.FromSlash(dirPart))
		}
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil
	}

	lowerBasePrefix := strings.ToLower(basePrefix)
	var matches []string

	for _, e := range entries {
		name := e.Name()
		if basePrefix == "" || strings.HasPrefix(strings.ToLower(name), lowerBasePrefix) {
			// 如果原始输入包含空格，补全时自动做友好处理
			item := dirPart + name
			isDir := e.IsDir()
			if !isDir && e.Type()&os.ModeSymlink != 0 {
				if fi, err := os.Stat(filepath.Join(targetDir, name)); err == nil {
					isDir = fi.IsDir()
				}
			}
			if isDir {
				item += "/"
			} else {
				item += " "
			}
			matches = append(matches, item)
		}
	}

	sort.Strings(matches)
	return matches
}

// listAllDirEntries 在空白处按 Tab 时列出当前目录下所有条目
func listAllDirEntries(line []rune, currentDir string) AutocompleteResult {
	if currentDir == "" {
		return AutocompleteResult{}
	}
	entries, err := os.ReadDir(currentDir)
	if err != nil || len(entries) == 0 {
		return AutocompleteResult{}
	}
	var suggestions []string
	for _, e := range entries {
		name := e.Name()
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(filepath.Join(currentDir, name)); err == nil {
				isDir = fi.IsDir()
			}
		}
		if isDir {
			name += "/"
		}
		suggestions = append(suggestions, name)
	}
	sort.Strings(suggestions)
	return AutocompleteResult{
		NewLine:      line,
		Suggestions:  suggestions,
		HasMatch:     true,
		IsExactMatch: false,
	}
}

// longestCommonPrefix 计算一组字符串的最长公共前缀
func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

// FormatSuggestions 将候选词格式化为规整的终端列表排版
func FormatSuggestions(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	// 计算每个候选的最大宽度
	maxLen := 0
	for _, s := range suggestions {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	colWidth := maxLen + 3
	if colWidth < 12 {
		colWidth = 12
	}

	cols := 80 / colWidth
	if cols < 1 {
		cols = 1
	}

	var sb strings.Builder
	for i, s := range suggestions {
		sb.WriteString(s)
		if (i+1)%cols == 0 || i == len(suggestions)-1 {
			sb.WriteString("\r\n")
		} else {
			padding := colWidth - len(s)
			if padding > 0 {
				sb.WriteString(strings.Repeat(" ", padding))
			}
		}
	}
	return strings.TrimRight(sb.String(), "\r\n")
}
