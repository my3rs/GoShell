package ui

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"goshell/assets/fonts"
	"goshell/internal/core/model"
	sftpService "goshell/internal/core/sftp"
)

// EditorFile 管理就地编辑的单个文件状态（内存缓冲、高亮语法、光标、保存等）
type EditorFile struct {
	Session    *TerminalSession
	Path       string
	Name       string
	Size       int64
	Buffer     *editor.Buffer
	State      *editor.State
	Language   string
	Loading    bool
	LoadErr    string
	IsBinary   bool
	Saving     bool
	SaveNotice string
	SaveErr    string
	mu         sync.Mutex
}

// NewEditorFile 创建并启动异步加载文件
func NewEditorFile(s *TerminalSession, filePath, fileName string, size int64) *EditorFile {
	ef := &EditorFile{
		Session: s,
		Path:    filePath,
		Name:    fileName,
		Size:    size,
		Loading: true,
	}
	return ef
}

// LoadAsync 异步从远端 SFTP 或本地磁盘加载文件内容
func (ef *EditorFile) LoadAsync() {
	ef.mu.Lock()
	ef.Loading = true
	ef.LoadErr = ""
	ef.IsBinary = false
	ef.SaveNotice = ""
	ef.SaveErr = ""
	ef.mu.Unlock()

	go func() {
		var data []byte
		var err error

		if ef.Session.Host.AuthType == model.AuthTypeLocal {
			data, err = os.ReadFile(ef.Path)
		} else if ef.Session.SFTPClient != nil {
			// 最大允许 5MB 文本文件就地编辑，防止超大日志耗尽内存
			data, err = ef.Session.SFTPClient.ReadFile(ef.Path, 5*1024*1024)
		} else {
			err = fmt.Errorf("SFTP 文件通道未连接")
		}

		ef.mu.Lock()
		defer ef.mu.Unlock()

		if err != nil {
			ef.LoadErr = err.Error()
			ef.Loading = false
			return
		}

		// 检查是否为二进制文件（检测前 1024 字节是否存在空字符 0x00）
		probeLen := len(data)
		if probeLen > 1024 {
			probeLen = 1024
		}
		if bytes.IndexByte(data[:probeLen], 0) != -1 {
			ef.IsBinary = true
			ef.Loading = false
			return
		}

		// 初始化代码编辑器 Buffer 与 State
		content := string(data)
		buf := editor.NewBuffer(content)
		lang := editor.LanguageFor(ef.Name)
		langName := "纯文本"
		if lang != nil {
			buf.SetLanguage(lang)
			langName = lang.Name
		}

		st := &editor.State{
			Buffer:      buf,
			Font:        ui.Font{Family: fonts.FallbackChain, Size: 13},
			LineNumbers: editor.LineNumbersOn, // 开启行号显示
		}

		// 挂载键盘输入快捷键监听 (Cmd+S / Ctrl+S)
		st.OnInput = func(ev ui.InputEvent) bool {
			if ev.Kind == ui.InputKeyDown && (ev.Key == ui.KeyS || ev.PhysicalKey == ui.KeyS) && (ev.Mods&ui.Cmd != 0 || ev.Mods&ui.Ctrl != 0 || ev.Mods&ui.Super != 0) {
				ef.Save()
				return true
			}
			return false
		}

		ef.Buffer = buf
		ef.State = st
		ef.Language = langName
		ef.Loading = false
	}()
}

// Reload 重新从远端加载当前文件（丢弃未保存更改）
func (ef *EditorFile) Reload() {
	ef.LoadAsync()
}

// Save 将内存编辑内容保存回远端或本地磁盘
func (ef *EditorFile) Save() {
	ef.mu.Lock()
	if ef.Loading || ef.IsBinary || ef.Buffer == nil || ef.Saving {
		ef.mu.Unlock()
		return
	}
	ef.Saving = true
	ef.SaveErr = ""
	ef.SaveNotice = ""
	content := ef.Buffer.String()
	ef.mu.Unlock()

	go func() {
		var err error
		if ef.Session.Host.AuthType == model.AuthTypeLocal {
			err = os.WriteFile(ef.Path, []byte(content), 0644)
		} else if ef.Session.SFTPClient != nil {
			err = ef.Session.SFTPClient.WriteFile(ef.Path, []byte(content))
		} else {
			err = fmt.Errorf("SFTP 服务未连接")
		}

		ef.mu.Lock()
		defer ef.mu.Unlock()

		ef.Saving = false
		if err != nil {
			ef.SaveErr = err.Error()
		} else {
			ef.Buffer.MarkSaved()
			ef.SaveNotice = fmt.Sprintf("已保存 %s", time.Now().Format("15:04:05"))
			// 触发目录列表后台刷新文件大小与修改时间
			ef.Session.RefreshFiles()
		}
	}()
}

// Render 在主视窗渲染就地代码编辑器
func (ef *EditorFile) Render(c *ui.Context, t *ui.Theme, onFocus func()) {
	// 捕获全局 Cmd+S / Ctrl+S 快捷键
	if c.Shortcut(ui.Cmd, ui.KeyS) || c.Shortcut(ui.Ctrl, ui.KeyS) {
		ef.Save()
	}

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 1. 顶部编辑器工具栏 (高度 38px，浅色细底边框)
		topBar := ui.Row(c).Height(38).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Padding(0, t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2))
		if topBar.Clicked() && onFocus != nil {
			onFocus()
		}
		topBar.Children(func() {
			// 文件图标
			ui.Icon(c, IconFile).TextColor(t.Accent).Width(14).Height(14)

			// 文件名
			ui.Text(c, ef.Name).FontSize(13).Bold().TextColor(t.Text)

			// 主机名标识胶囊
			ui.Row(c).Background(ColBadgeBlueBg).Padding(1, 6).Radius(t.Radius).Children(func() {
				ui.Text(c, ef.Session.Host.Name).FontSize(10.5).Bold().TextColor(t.Accent)
			})

			// 完整远程绝对路径
			ui.Text(c, ef.Path).FontSize(11).TextColor(t.TextMuted)

			// 保存状态徽章 (参考终端页面药丸徽章规范)
			if ef.Buffer != nil && ef.Buffer.Modified() {
				ui.Row(c).Background(ColBadgeAmberBg).Padding(2, 8).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Warning)
					ui.Text(c, "未保存").FontSize(10.5).Bold().TextColor(ColBadgeAmberText)
				})
			} else if !ef.Loading && ef.LoadErr == "" && !ef.IsBinary {
				ui.Row(c).Background(ColBadgeGreenBg).Padding(2, 8).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
					ui.Text(c, "已保存").FontSize(10.5).Bold().TextColor(ColBadgeGreenText)
				})
			}


			ui.Spacer(c)

			// 动态提示文本
			if ef.Saving {
				ui.Text(c, "正在保存至服务器...").FontSize(11).TextColor(t.Warning)
			} else if ef.SaveErr != "" {
				ui.Text(c, "保存失败: "+ef.SaveErr).FontSize(11).TextColor(t.Danger)
			} else if ef.SaveNotice != "" {
				ui.Text(c, ef.SaveNotice).FontSize(11).TextColor(t.Success)
			}

			// 重新加载按钮
			if ui.Button(c, "重新读取").Clicked() {
				ef.Reload()
			}

			// 保存按钮 (针对 macOS 和 Windows 显示对应快捷键提示)
			if ui.PrimaryButton(c, SaveButtonLabel()).Tooltip("保存 (" + SaveShortcutText() + ")").Clicked() {
				ef.Save()
			}
		})

		// 2. 主体编辑内容区域
		if ef.Loading {
			ui.Column(c).Grow(1).Center().Gap(t.Space(2)).Children(func() {
				ui.Box(c).Width(8).Height(8).Radius(4).Background(t.Accent)
				ui.Text(c, fmt.Sprintf("正在读取文件: %s ...", ef.Path)).FontSize(13).Bold().TextColor(t.Text)
			})
		} else if ef.LoadErr != "" {
			ui.Column(c).Grow(1).Center().Gap(t.Space(3)).Children(func() {
				ui.Box(c).Width(10).Height(10).Radius(5).Background(t.Danger)
				ui.Text(c, "读取文件失败").FontSize(15).Bold().TextColor(t.Danger)
				ui.Text(c, ef.LoadErr).FontSize(12).TextColor(t.TextMuted)
				if ui.PrimaryButton(c, "重试读取").Clicked() {
					ef.Reload()
				}
			})
		} else if ef.IsBinary {
			ui.Column(c).Grow(1).Center().Gap(t.Space(3)).Children(func() {
				ui.Icon(c, IconFile).TextColor(t.TextMuted).Width(36).Height(36)
				ui.Text(c, "该文件为二进制格式，不支持在线代码就地编辑").FontSize(15).Bold().TextColor(t.Text)
				ui.Text(c, "在线编辑器专为 Shell 脚本、配置文件、Python、Go、JSON、YAML、SQL 等文本文件设计。").FontSize(12).TextColor(t.TextMuted)
				if ef.Session != nil && ef.Session.ShouldShowTransferQueue() {
					if ui.PrimaryButton(c, "下载到本地").Clicked() {
						ef.Session.DownloadEntry(sftpService.FileEntry{
							Name: ef.Name,
							Path: ef.Path,
							Size: ef.Size,
						})
					}
				}
			})
		} else if ef.State != nil {
			// MyGo GPU 自绘代码编辑器：行号由 LineNumbersOn 驱动，高亮由 Buffer 语法与主题驱动
			edElem := editor.View(c, ef.State).Grow(1).FillWidth()
			if edElem.Focused() && onFocus != nil {
				onFocus()
			}
		}

		// 3. 底部状态栏 (高度 24px)
		ui.Row(c).Height(24).Shrink(0).Background(ColSidebarBg).Border(1, ColSidebarBorder).Padding(0, t.Space(3)).AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
			if ef.State != nil && ef.Buffer != nil {
				pos := ef.State.CursorPos()
				ui.Text(c, fmt.Sprintf("行 %d, 列 %d", pos.Line+1, pos.Col+1)).FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("共 %d 行", ef.Buffer.LineCount())).FontSize(10.5).TextColor(t.TextMuted)

				le := "LF"
				if ef.Buffer.LineEnding() == "\r\n" {
					le = "CRLF"
				}
				ui.Text(c, "换行: "+le).FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, "编码: UTF-8").FontSize(10.5).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("大小: %s", sftpService.FormatSize(int64(ef.Buffer.Len()), false))).FontSize(10.5).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			ui.Text(c, "语法: "+ef.Language).FontSize(10.5).Bold().TextColor(t.Accent)
		})
	})
}

// SaveShortcutText 返回当前操作系统对应的保存快捷键提示（macOS 为 Cmd+S，Windows 及其他系统为 Ctrl+S）
func SaveShortcutText() string {
	return SaveShortcutTextForOS(runtime.GOOS)
}

// SaveShortcutTextForOS 根据操作系统标识返回保存快捷键提示
func SaveShortcutTextForOS(goos string) string {
	if goos == "darwin" {
		return "Cmd+S"
	}
	return "Ctrl+S"
}

// SaveButtonLabel 返回针对当前操作系统定制的保存按钮文案
func SaveButtonLabel() string {
	return SaveButtonLabelForOS(runtime.GOOS)
}

// SaveButtonLabelForOS 根据操作系统标识返回保存按钮文案
func SaveButtonLabelForOS(goos string) string {
	return fmt.Sprintf("保存 (%s)", SaveShortcutTextForOS(goos))
}

