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

// startFallbackPipeTerminal 在不支持 ConPTY 的旧版 Windows 上，通过匿名管道和静默后台进程启动 cmd.exe
func startFallbackPipeTerminal(dir string) (io.ReadWriteCloser, error) {
	// 优先寻找 cmd.exe (Win10 1809 之前在管道模式下 cmd.exe 毫秒级秒启、最稳定)
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
	// STARTF_USESHOWWINDOW 配合 SW_HIDE，并在 CreateProcess 中指定 CREATE_NO_WINDOW，
	// 彻底杜绝 Windows 为控制台子进程（cmd.exe）弹出一个独立的外部控制台黑窗口
	si.Flags = syscall.STARTF_USESTDHANDLES | syscall.STARTF_USESHOWWINDOW
	si.ShowWindow = syscall.SW_HIDE
	si.StdInput = inR
	si.StdOutput = outW
	si.StdErr = outW

	args := []string{shellPath}
	if strings.EqualFold(filepath.Base(shellPath), "powershell.exe") {
		args = append(args, "-NoLogo")
	}

	cmdlineStr := strings.Join(func() []string {
		var escaped []string
		for _, a := range args {
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
		return nil, fmt.Errorf("启动本地 Shell 失败 (%s): %w", shellPath, err)
	}
	syscall.CloseHandle(pi.Thread)

	oemCP := uint32(0)
	if r, _, _ := procGetOEMCP.Call(); r != 0 {
		oemCP = uint32(r)
	}

	pr, pw := io.Pipe()
	conn := &pipeConn{
		process: pi.Process,
		in:      os.NewFile(uintptr(inW), "pipe-in"),
		out:     os.NewFile(uintptr(outR), "pipe-out"),
		pr:      pr,
		pw:      pw,
		oemCP:   oemCP,
	}
	go conn.readProcessOutputLoop()

	return conn, nil
}

type pipeConn struct {
	mu      sync.Mutex
	process syscall.Handle
	in      *os.File
	out     *os.File
	pr      *io.PipeReader
	pw      *io.PipeWriter
	oemCP   uint32
	lineBuf []rune
	closed  bool
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
				if _, werr := c.pw.Write(outBytes); werr != nil {
					break
				}
			}
		}

		if err != nil {
			if len(pending) > 0 && c.oemCP != 0 && c.oemCP != 65001 {
				outBytes := cpToUTF8(c.oemCP, pending)
				_, _ = c.pw.Write(outBytes)
			}
			_ = c.pw.CloseWithError(err)
			break
		}
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
		case '\r', '\n':
			// 跳过紧随 \r 后的 \n，避免双重换行
			if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			// 1. 本地回显换行
			_, _ = c.pw.Write([]byte("\r\n"))

			// 2. 将当前行缓冲拼接 \r\n 并转码写入 cmd.exe 的标准输入管道
			cmdLine := string(c.lineBuf) + "\r\n"
			c.lineBuf = c.lineBuf[:0]

			var toWrite []byte
			if c.oemCP != 0 && c.oemCP != 65001 {
				toWrite = utf8ToCP(c.oemCP, []byte(cmdLine))
			} else {
				toWrite = []byte(cmdLine)
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
			// 普通可打印字符、中文、空格、Tab等
			if r >= 0x20 || r == '\t' {
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

