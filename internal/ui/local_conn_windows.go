//go:build windows

package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"goshell/assets/natives"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreatePseudoConsole = kernel32.NewProc("CreatePseudoConsole")
	procGetOEMCP            = kernel32.NewProc("GetOEMCP")
	procMultiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")
	procWideCharToMultiByte = kernel32.NewProc("WideCharToMultiByte")
)

const (
	createUnicodeEnvironment = 0x00000400
	createNoWindow           = 0x08000000
)

// shouldFallbackPipe 检查系统是否缺少 ConPTY API (Windows 10 1809 之前版本，如 Win10 1709 / Win7 / Win8)
func shouldFallbackPipe() bool {
	return procCreatePseudoConsole.Find() != nil
}

type shellConfig struct {
	path        string
	args        []string
	isUTF8Shell bool
	name        string
}

// findWindowsShell 自动探测系统最优终端 Shell：优先遵循用户显式指定，其次探测 BusyBox 虚拟 Bash、Git Bash，最后降级 cmd.exe
func findWindowsShell(targetShell string) shellConfig {
	// 0. 支持用户显式传入参数或环境变量 (如 targetShell 或 GOSHELL_SHELL=cmd / powershell / busybox)
	preferred := strings.TrimSpace(targetShell)
	if preferred == "" || preferred == "auto" {
		preferred = strings.TrimSpace(os.Getenv("GOSHELL_SHELL"))
	}
	if preferred != "" && preferred != "auto" {
		if strings.EqualFold(preferred, "cmd") || strings.EqualFold(preferred, "cmd.exe") {
			cmdPath := os.Getenv("COMSPEC")
			if cmdPath == "" {
				cmdPath = "C:\\Windows\\System32\\cmd.exe"
			}
			return shellConfig{
				path:        cmdPath,
				args:        []string{cmdPath},
				isUTF8Shell: false,
				name:        "Command Prompt",
			}
		}
		if strings.EqualFold(preferred, "powershell") || strings.EqualFold(preferred, "powershell.exe") {
			psPath := "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
			if p, err := exec.LookPath("powershell.exe"); err == nil {
				psPath = p
			}
			return shellConfig{
				path:        psPath,
				args:        []string{psPath, "-NoLogo"},
				isUTF8Shell: false,
				name:        "PowerShell",
			}
		}
		if strings.EqualFold(preferred, "busybox") || strings.EqualFold(preferred, "busybox.exe") || strings.EqualFold(preferred, "bash") || strings.EqualFold(preferred, "sh") {
			if p, err := natives.EnsureBusybox(); err == nil && p != "" {
				return shellConfig{
					path:        p,
					args:        []string{p, "sh", "-l", "-i"},
					isUTF8Shell: true,
					name:        "BusyBox Bash",
				}
			}
		}
	}

	// 1. 优先使用内置或同级 BusyBox (支持单文件自动释放与绿色便携运行)
	if p, err := natives.EnsureBusybox(); err == nil && p != "" {
		return shellConfig{
			path:        p,
			args:        []string{p, "sh", "-l", "-i"},
			isUTF8Shell: true,
			name:        "BusyBox Bash",
		}
	}

	// 2. 检查环境变量 PATH 中是否有 busybox.exe
	if p, err := exec.LookPath("busybox.exe"); err == nil {
		return shellConfig{
			path:        p,
			args:        []string{p, "sh", "-l", "-i"},
			isUTF8Shell: true,
			name:        "BusyBox Bash",
		}
	}

	// 3. 检查系统是否安装了 Git for Windows 官方 Bash
	gitCandidates := []string{
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		gitCandidates = append(gitCandidates, filepath.Join(localAppData, `Programs\Git\bin\bash.exe`))
	}
	for _, cand := range gitCandidates {
		if stat, err := os.Stat(cand); err == nil && !stat.IsDir() {
			return shellConfig{
				path:        cand,
				args:        []string{cand, "-l", "-i"},
				isUTF8Shell: true,
				name:        "Git Bash",
			}
		}
	}

	// 4. 降级使用 Windows 系统默认 cmd.exe
	shellPath := os.Getenv("COMSPEC")
	if shellPath == "" {
		if p, err := exec.LookPath("cmd.exe"); err == nil {
			shellPath = p
		} else if p, err := exec.LookPath("powershell.exe"); err == nil {
			shellPath = p
		}
	}
	if shellPath == "" {
		shellPath = "C:\\Windows\\System32\\cmd.exe"
	}

	args := []string{shellPath}
	if strings.EqualFold(filepath.Base(shellPath), "powershell.exe") {
		args = append(args, "-NoLogo")
	}

	return shellConfig{
		path:        shellPath,
		args:        args,
		isUTF8Shell: false,
		name:        "Command Prompt",
	}
}

// startFallbackPipeTerminal 在不支持 ConPTY 的旧版 Windows 上，通过匿名管道和静默后台进程启动虚拟终端 Shell
func startFallbackPipeTerminal(dir string, shellType string, onIntercept func(line string) bool) (io.ReadWriteCloser, error) {
	shell := findWindowsShell(shellType)

	if shell.isUTF8Shell {
		_ = os.Setenv("BB_TERMINAL_MODE", "1")
		_ = os.Setenv("BB_SKIP_ANSI_EMULATION", "2")
		_ = os.Setenv("TERM", "xterm-256color")
		_ = os.Setenv("LANG", "zh_CN.UTF-8")
		_ = os.Setenv("LC_ALL", "zh_CN.UTF-8")
		// 将程序所在目录加入 PATH，使得外部调用 busybox 内置命令如 ls/grep/cat 直接生效
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			currPath := os.Getenv("PATH")
			if !strings.Contains(currPath, exeDir) {
				_ = os.Setenv("PATH", exeDir+string(os.PathListSeparator)+currPath)
			}
		}
	}

	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1

	var inR, inW, outR, outW syscall.Handle
	if err := syscall.CreatePipe(&inR, &inW, &sa, 0); err != nil {
		return nil, fmt.Errorf("CreatePipe in: %w", err)
	}
	_ = syscall.SetHandleInformation(inW, syscall.HANDLE_FLAG_INHERIT, 0)

	if err := syscall.CreatePipe(&outR, &outW, &sa, 0); err != nil {
		syscall.CloseHandle(inR)
		syscall.CloseHandle(inW)
		return nil, fmt.Errorf("CreatePipe out: %w", err)
	}
	_ = syscall.SetHandleInformation(outR, syscall.HANDLE_FLAG_INHERIT, 0)

	defer syscall.CloseHandle(inR)
	defer syscall.CloseHandle(outW)

	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = syscall.STARTF_USESTDHANDLES | syscall.STARTF_USESHOWWINDOW
	si.ShowWindow = syscall.SW_HIDE
	si.StdInput = inR
	si.StdOutput = outW
	si.StdErr = outW

	cmdlineStr := strings.Join(func() []string {
		var escaped []string
		for _, a := range shell.args {
			escaped = append(escaped, syscall.EscapeArg(a))
		}
		return escaped
	}(), " ")

	cmdline, err := syscall.UTF16PtrFromString(cmdlineStr)
	if err != nil {
		syscall.CloseHandle(inW)
		syscall.CloseHandle(outR)
		return nil, err
	}

	var dirPtr *uint16
	if dir != "" {
		if dirPtr, err = syscall.UTF16PtrFromString(dir); err != nil {
			syscall.CloseHandle(inW)
			syscall.CloseHandle(outR)
			return nil, err
		}
	}

	var pi syscall.ProcessInformation
	err = syscall.CreateProcess(nil, cmdline, nil, nil, true, createUnicodeEnvironment|createNoWindow, nil, dirPtr, &si, &pi)
	if err != nil {
		syscall.CloseHandle(inW)
		syscall.CloseHandle(outR)
		return nil, fmt.Errorf("启动本地 Shell 失败 (%s): %w", shell.path, err)
	}
	syscall.CloseHandle(pi.Thread)

	oemCP := uint32(0)
	if r, _, _ := procGetOEMCP.Call(); r != 0 {
		oemCP = uint32(r)
	}

	currentDir := dir
	if currentDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			currentDir = home
		}
	}

	pr, pw := io.Pipe()
	conn := &pipeConn{
		process:     pi.Process,
		in:          os.NewFile(uintptr(inW), "pipe-in"),
		out:         os.NewFile(uintptr(outR), "pipe-out"),
		pr:          pr,
		pw:          pw,
		oemCP:       oemCP,
		isUTF8Shell: shell.isUTF8Shell,
		onIntercept: onIntercept,
		workDir:     currentDir,
	}
	go conn.readProcessOutputLoop()

	return conn, nil
}

type pipeConn struct {
	mu          sync.Mutex
	process     syscall.Handle
	in          *os.File
	out         *os.File
	pr          *io.PipeReader
	pw          *io.PipeWriter
	oemCP       uint32
	isUTF8Shell bool
	lineBuf     []rune
	onIntercept func(line string) bool
	closed      bool
	normalizer  crlfNormalizer
	workDir     string
	lastPrompt  string
}

// readProcessOutputLoop 在后台持续读取子进程输出，流式转码 GBK -> UTF-8 并写入管道供终端消费
func (c *pipeConn) readProcessOutputLoop() {
	raw := make([]byte, 4096)
	var data []byte
	var pending []byte

	for {
		n, err := c.out.Read(raw)
		if n > 0 {
			data = append(pending, raw[:n]...)
			pending = nil

			var outBytes []byte
			if c.oemCP == 0 || c.oemCP == 65001 {
				outBytes = data
				data = nil
			} else {
				valid, inc := isUTF8(data)
				if valid {
					if inc > 0 && err == nil {
						pending = append(pending, data[len(data)-inc:]...)
						data = data[:len(data)-inc]
					}
					outBytes = data
				} else {
					incTail := gbkIncompleteTail(data)
					if incTail > 0 && err == nil {
						pending = append(pending, data[len(data)-incTail:]...)
						data = data[:len(data)-incTail]
					}
					if len(data) > 0 {
						outBytes = cpToUTF8(c.oemCP, data)
					}
				}
				data = nil
			}

			if len(outBytes) > 0 {
				normalized := c.normalizer.Normalize(outBytes)
				c.recordPrompt(normalized)
				if _, werr := c.pw.Write(normalized); werr != nil {
					break
				}
			}
		}

		if err != nil {
			if len(pending) > 0 && c.oemCP != 0 && c.oemCP != 65001 {
				outBytes := cpToUTF8(c.oemCP, pending)
				normalized := c.normalizer.Normalize(outBytes)
				c.recordPrompt(normalized)
				_, _ = c.pw.Write(normalized)
			}
			_ = c.pw.CloseWithError(err)
			break
		}
	}
}

func (c *pipeConn) recordPrompt(b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	str := string(b)
	lastIdx := strings.LastIndex(str, "\n")
	if lastIdx >= 0 {
		c.lastPrompt = str[lastIdx+1:]
	} else {
		c.lastPrompt += str
	}
}

func (c *pipeConn) handleCD(cmdLine string) {
	trimmed := strings.TrimSpace(cmdLine)
	if !strings.HasPrefix(trimmed, "cd") {
		return
	}
	if len(trimmed) > 2 && trimmed[2] != ' ' && trimmed[2] != '\t' {
		return
	}
	target := strings.TrimSpace(trimmed[2:])
	if target == "" || target == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			c.workDir = home
		}
		return
	}
	target = strings.Trim(target, "\"'")
	if filepath.IsAbs(target) {
		if stat, err := os.Stat(target); err == nil && stat.IsDir() {
			c.workDir = target
		}
		return
	}
	newDir := filepath.Join(c.workDir, target)
	if stat, err := os.Stat(newDir); err == nil && stat.IsDir() {
		c.workDir = filepath.Clean(newDir)
	}
}

func (c *pipeConn) Read(b []byte) (int, error) {
	return c.pr.Read(b)
}

func (c *pipeConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.ErrClosedPipe
	}

	runes := []rune(string(b))
	for i := 0; i < len(runes); {
		r := runes[i]

		// 遇到 ANSI Escape 序列（如方向键 \x1b[A 等），跳过并忽略，避免污染行缓冲与屏幕
		if r == 0x1b {
			i = skipEscapeSequence(runes, i)
			continue
		}

		switch r {
		case '\t':
			// 触发智能 Tab 自动补全
			res := Autocomplete(c.lineBuf, c.workDir)
			if !res.HasMatch {
				i++
				continue
			}
			if res.IsExactMatch {
				// 单项精确匹配：直接在当前行补齐
				c.lineBuf = res.NewLine
				_, _ = c.pw.Write([]byte(string(res.AddedRunes)))
			} else {
				// 多项匹配：先补齐最长公共前缀（若有）
				if len(res.AddedRunes) > 0 {
					c.lineBuf = res.NewLine
					_, _ = c.pw.Write([]byte(string(res.AddedRunes)))
				}
				// 换行展示所有候选并重绘提示符行
				if len(res.Suggestions) > 0 {
					prompt := c.lastPrompt
					if prompt == "" {
						prompt = "~ # "
					}
					formatted := FormatSuggestions(res.Suggestions)
					_, _ = c.pw.Write([]byte("\r\n" + formatted + "\r\n" + prompt + string(c.lineBuf)))
				}
			}

		case '\r', '\n':
			// 跳过紧随 \r 后的 \n，避免双重换行
			if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			// 1. 本地回显换行
			_, _ = c.pw.Write([]byte("\r\n"))

			cmdLine := string(c.lineBuf)
			c.lineBuf = c.lineBuf[:0]
			c.lastPrompt = "" // 重置上一轮提示符
			c.handleCD(cmdLine)

			// 2. 检查是否拦截特殊指令 (例如直接输入 ssh 命令)
			if c.onIntercept != nil && c.onIntercept(cmdLine) {
				_, _ = c.pw.Write([]byte("[GoShell] 已识别 SSH 命令，正在为您开启原生连接...\r\n"))
				i++
				continue
			}

			// 3. 将当前行缓冲拼接 \r\n 并转码写入子进程的标准输入管道
			cmdToSend := cmdLine + "\r\n"
			var toWrite []byte
			if c.isUTF8Shell || c.oemCP == 0 || c.oemCP == 65001 {
				toWrite = []byte(cmdToSend)
			} else {
				toWrite = utf8ToCP(c.oemCP, []byte(cmdToSend))
			}
			if _, err := c.in.Write(toWrite); err != nil {
				return 0, err
			}

		case '\b', 0x7f: // Backspace 退格键
			if len(c.lineBuf) > 0 {
				lastRune := c.lineBuf[len(c.lineBuf)-1]
				c.lineBuf = c.lineBuf[:len(c.lineBuf)-1]

				// 在终端屏幕上擦除字符：单宽字符发 \b \b，双宽（汉字）发 \b\b  \b\b
				if runeWidth(lastRune) == 2 {
					_, _ = c.pw.Write([]byte("\b\b  \b\b"))
				} else {
					_, _ = c.pw.Write([]byte("\b \b"))
				}
			}

		case 0x03: // Ctrl+C 中断
			c.lineBuf = c.lineBuf[:0]
			_, _ = c.pw.Write([]byte("^C\r\n"))
			_, _ = c.in.Write([]byte("\x03\r\n"))

		default:
			// 普通可打印字符、中文、空格等 (Tab 已在上方独立处理)
			if r >= 0x20 {
				c.lineBuf = append(c.lineBuf, r)
				// 实时本地回显到终端屏幕
				_, _ = c.pw.Write([]byte(string(r)))
			}
		}
		i++
	}

	return len(b), nil
}

func (c *pipeConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	if c.pw != nil {
		_ = c.pw.Close()
	}
	if c.pr != nil {
		_ = c.pr.Close()
	}
	if c.in != nil {
		_ = c.in.Close()
	}
	if c.out != nil {
		_ = c.out.Close()
	}
	if c.process != 0 {
		_ = syscall.TerminateProcess(c.process, 1)
		_ = syscall.CloseHandle(c.process)
	}
	return nil
}

func cpToUTF8(cp uint32, b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	r, _, _ := procMultiByteToWideChar.Call(
		uintptr(cp),
		0,
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		0,
		0,
	)
	if r == 0 {
		return b
	}
	wbuf := make([]uint16, r)
	r2, _, _ := procMultiByteToWideChar.Call(
		uintptr(cp),
		0,
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		uintptr(unsafe.Pointer(&wbuf[0])),
		r,
	)
	if r2 == 0 {
		return b
	}
	return []byte(string(utf16.Decode(wbuf)))
}

func utf8ToCP(cp uint32, b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	allASCII := true
	for _, c := range b {
		if c >= 0x80 {
			allASCII = false
			break
		}
	}
	if allASCII {
		return b
	}
	wbuf := utf16.Encode([]rune(string(b)))
	if len(wbuf) == 0 {
		return b
	}
	r, _, _ := procWideCharToMultiByte.Call(
		uintptr(cp),
		0,
		uintptr(unsafe.Pointer(&wbuf[0])),
		uintptr(len(wbuf)),
		0,
		0,
		0,
		0,
	)
	if r == 0 {
		return b
	}
	mb := make([]byte, r)
	r2, _, _ := procWideCharToMultiByte.Call(
		uintptr(cp),
		0,
		uintptr(unsafe.Pointer(&wbuf[0])),
		uintptr(len(wbuf)),
		uintptr(unsafe.Pointer(&mb[0])),
		r,
		0,
		0,
	)
	if r2 == 0 {
		return b
	}
	return mb
}

