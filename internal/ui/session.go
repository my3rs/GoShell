package ui

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/google/uuid"

	"goshell/assets/fonts"
	"goshell/internal/core/model"
	sftpService "goshell/internal/core/sftp"
	sshClient "goshell/internal/core/ssh"
	"goshell/internal/core/transfer"
)

// TerminalSession 代表一个活跃的终端会话实例，整合 SSH 终端与 SFTP 文件服务
type TerminalSession struct {
	ID       string
	Host     *model.Host
	Title    string
	Term     *terminal.Terminal
	Client   *sshClient.Client
	TermConn *sshClient.TerminalConn

	// SFTP 远程文件管理与传输管理器
	SFTPClient  *sftpService.Client
	TransferMgr *transfer.Manager
	CurrentDir  string
	PathInput   string // 路径输入栏双向绑定
	FileList    []sftpService.FileEntry
	FileLoading bool
	FileError   string

	// UI 交互控制
	ShowTransfer bool
	selectedPath string
	ShowHidden   bool                   // 是否在左侧文件列表中显示隐藏文件 (以 . 开头)
	scrollbar    terminalScrollbarState // 终端滚动条拖拽与悬停状态

	Connecting bool
	Connected  bool
	ErrorMsg   string

	StartTime time.Time
	mu        sync.Mutex

	// 节点专属配色分配索引
	ColorIndex int

	// UI 界面重绘通知回调
	OnUpdate func()

	// 本地终端拦截 SSH 命令回调 (用于在终端中输入 ssh 指令时自动唤起原生连接)
	OnSSHIntercept func(target *model.Host)
}

// notifyUpdate 通知主界面重新绘制（无锁安全调用）
func (s *TerminalSession) notifyUpdate() {
	if s == nil {
		return
	}
	if s.OnUpdate != nil {
		s.OnUpdate()
	}
}

// VisibleFiles 返回根据 ShowHidden 设置过滤后的文件列表
func (s *TerminalSession) VisibleFiles() []sftpService.FileEntry {
	if s.ShowHidden {
		return s.FileList
	}
	filtered := make([]sftpService.FileEntry, 0, len(s.FileList))
	for _, f := range s.FileList {
		if !strings.HasPrefix(f.Name, ".") {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// HostColor 返回当前会话节点的专属配色方案
func (s *TerminalSession) HostColor() HostThemeColor {
	if s == nil {
		return HostColorPalette[0]
	}
	idx := s.ColorIndex % len(HostColorPalette)
	if idx < 0 {
		idx = 0
	}
	return HostColorPalette[idx]
}

// NewTerminalSession 初始化一个新的会话实例
func NewTerminalSession(host *model.Host) *TerminalSession {
	return &TerminalSession{
		ID:         uuid.New().String(),
		Host:       host,
		Title:      host.Name,
		StartTime:  time.Now(),
		CurrentDir: "/",
		PathInput:  "/",
	}
}

// Start 启动终端与 SFTP 服务
func (s *TerminalSession) Start() {
	s.mu.Lock()
	if s.Connecting || s.Connected {
		s.mu.Unlock()
		return
	}
	s.Connecting = true
	s.ErrorMsg = ""
	s.mu.Unlock()
	s.notifyUpdate()

	_ = fonts.Register()

	// 1. 本地 Shell 与本地文件系统
	if s.Host.AuthType == model.AuthTypeLocal {
		home, _ := os.UserHomeDir()
		if home == "" {
			if runtime.GOOS == "windows" {
				home = "C:\\"
			} else {
				home = "/"
			}
		}
		s.CurrentDir = home
		s.PathInput = home

		go func() {
			opts := terminal.Options{
				Dir: home,
				Font: terminal.Font{
					Family: fonts.FallbackChain,
					Size:   13,
				},
				CopyOnSelect: terminal.CopyOnSelectClipboard,
				Theme:        terminal.DarkTheme(),
				DarkTheme:    terminal.DarkTheme(),
				OnTitle: func(title string) {
					if title != "" {
						s.mu.Lock()
						s.Title = title
						s.mu.Unlock()
						s.notifyUpdate()
					}
				},
				OnExit: func(code int) {
					s.mu.Lock()
					s.Connected = false
					exitMsg := fmt.Sprintf("本地终端已退出 (退出码: %d)", code)
					if code == -1 && s.Term != nil {
						if termText := strings.TrimSpace(s.Term.Text()); termText != "" {
							exitMsg = fmt.Sprintf("启动终端失败: %s", termText)
						}
					}
					s.ErrorMsg = exitMsg
					s.mu.Unlock()
					s.notifyUpdate()
				},
			}

			shellType := ""
			if s.Host != nil {
				shellType = s.Host.ShellType
			}

			// 在不支持 ConPTY 的旧版 Windows（如 Windows 10 1709 / Win7 / Win8）上，
			// 自动通过管道模式以静默方式启动虚拟终端 Shell，并通过 Conn 接口直接对接 Ghostty VT，
			// 彻底避免 ConPTY 检查失败与外部黑框弹窗
			if shouldFallbackPipe() {
				conn, err := startFallbackPipeTerminal(home, shellType, func(line string) bool {
					if host, ok := ParseSSHCommand(line); ok {
						if s.OnSSHIntercept != nil {
							s.OnSSHIntercept(host)
							return true
						}
					}
					return false
				})
				if err != nil {
					s.mu.Lock()
					s.Connecting = false
					s.ErrorMsg = fmt.Sprintf("启动本地终端失败: %v", err)
					s.mu.Unlock()
					s.notifyUpdate()
					return
				}
				opts.Conn = conn
			} else if runtime.GOOS == "windows" && shellType != "" && shellType != "auto" {
				// 支持 ConPTY 的新版 Windows 模式：若用户显式指定了 Shell 类型，配置给 Command
				conf := findWindowsShell(shellType)
				if len(conf.args) > 0 {
					opts.Command = conf.args
				}
			}

			term, err := terminal.New(opts)
			if err != nil {
				if opts.Conn != nil {
					_ = opts.Conn.Close()
				}
				s.mu.Lock()
				s.Connecting = false
				s.ErrorMsg = fmt.Sprintf("启动终端引擎失败: %v", err)
				s.mu.Unlock()
				s.notifyUpdate()
				return
			}

			s.mu.Lock()
			if !s.Connecting {
				s.mu.Unlock()
				_ = term.Close()
				return
			}
			s.Term = term
			s.Connecting = false
			s.Connected = true
			s.mu.Unlock()
			s.notifyUpdate()

			s.RefreshFiles()
		}()
		return
	}

	// 2. 远程 SSH 与 SFTP 独立信道
	go func() {
		client, err := sshClient.Dial(s.Host)
		if err != nil {
			s.mu.Lock()
			s.Connecting = false
			s.ErrorMsg = fmt.Sprintf("SSH 连接失败: %v", err)
			s.mu.Unlock()
			s.notifyUpdate()
			return
		}

		termConn, err := client.OpenTerminal(100, 30)
		if err != nil {
			_ = client.Close()
			s.mu.Lock()
			s.Connecting = false
			s.ErrorMsg = fmt.Sprintf("申请 PTY 终端通道失败: %v", err)
			s.mu.Unlock()
			s.notifyUpdate()
			return
		}

		term, err := terminal.New(terminal.Options{
			Conn: termConn,
			Font: terminal.Font{
				Family: fonts.FallbackChain,
				Size:   13,
			},
			CopyOnSelect: terminal.CopyOnSelectClipboard,
			Theme:        terminal.DarkTheme(),
			DarkTheme:    terminal.DarkTheme(),
			OnTitle: func(title string) {
				if title != "" {
					s.mu.Lock()
					s.Title = title
					s.mu.Unlock()
					s.notifyUpdate()
				}
			},
			OnExit: func(code int) {
				s.mu.Lock()
				s.Connected = false
				s.ErrorMsg = fmt.Sprintf("远程连接已关闭 (退出码: %d)", code)
				s.mu.Unlock()
				s.notifyUpdate()
			},
		})
		if err != nil {
			_ = termConn.Close()
			_ = client.Close()
			s.mu.Lock()
			s.Connecting = false
			s.ErrorMsg = fmt.Sprintf("初始化终端引擎失败: %v", err)
			s.mu.Unlock()
			s.notifyUpdate()
			return
		}

		// 创建独立 SFTP 子通道，与 PTY 隔离
		sftpCli, err := sftpService.NewClient(client.RawClient())
		var transMgr *transfer.Manager
		var defaultDir = "/"
		if err == nil {
			transMgr = transfer.NewManager(sftpCli, 3)
			if cwd, err := sftpCli.GetWorkingDirectory(); err == nil && cwd != "" {
				defaultDir = cwd
			}
		}

		s.mu.Lock()
		if !s.Connecting {
			s.mu.Unlock()
			_ = client.Close()
			_ = termConn.Close()
			_ = term.Close()
			if sftpCli != nil {
				_ = sftpCli.Close()
			}
			return
		}
		s.Client = client
		s.TermConn = termConn
		s.Term = term
		s.SFTPClient = sftpCli
		s.TransferMgr = transMgr
		s.CurrentDir = defaultDir
		s.PathInput = defaultDir
		s.Connecting = false
		s.Connected = true
		s.mu.Unlock()
		s.notifyUpdate()

		s.RefreshFiles()
	}()
}

// RefreshFiles 刷新当前目录下的文件列表
func (s *TerminalSession) RefreshFiles() {
	s.mu.Lock()
	s.FileLoading = true
	s.FileError = ""
	dir := s.CurrentDir
	s.mu.Unlock()

	go func() {
		var entries []sftpService.FileEntry
		var err error

		if s.Host.AuthType == model.AuthTypeLocal {
			osEntries, readErr := os.ReadDir(dir)
			if readErr != nil {
				err = readErr
			} else {
				for _, e := range osEntries {
					info, iErr := e.Info()
					if iErr != nil {
						continue
					}
					isDir := e.IsDir()
					fullPath := filepath.Join(dir, e.Name())

					// 若条目为符号链接，检查其实际指向的目标是否为目录 (例如 macOS 下的 Google Drive)
					if !isDir && (e.Type()&os.ModeSymlink != 0 || info.Mode()&os.ModeSymlink != 0) {
						if statInfo, sErr := os.Stat(fullPath); sErr == nil {
							isDir = statInfo.IsDir()
						}
					}

					entries = append(entries, sftpService.FileEntry{
						Name:      e.Name(),
						Path:      fullPath,
						Size:      info.Size(),
						SizeHuman: sftpService.FormatSize(info.Size(), isDir),
						Mode:      info.Mode().String(),
						IsDir:     isDir,
						ModTime:   info.ModTime(),
					})
				}
				sort.Slice(entries, func(i, j int) bool {
					if entries[i].IsDir != entries[j].IsDir {
						return entries[i].IsDir
					}
					return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
				})
			}
		} else {
			if s.SFTPClient != nil {
				entries, err = s.SFTPClient.ListDirectory(dir)
			} else {
				err = fmt.Errorf("SFTP 信道未就绪")
			}
		}

		s.mu.Lock()
		s.FileLoading = false
		if err != nil {
			s.FileError = err.Error()
		} else {
			s.FileList = entries
		}
		s.mu.Unlock()
		s.notifyUpdate()
	}()
}

// NavigateTo 进入指定目录
func (s *TerminalSession) NavigateTo(targetPath string) {
	if targetPath == "" {
		targetPath = "/"
	}
	s.mu.Lock()
	s.CurrentDir = targetPath
	s.PathInput = targetPath
	s.selectedPath = ""
	s.mu.Unlock()
	s.RefreshFiles()
}

// NavigateUp 返回上一级目录
func (s *TerminalSession) NavigateUp() {
	s.mu.Lock()
	var parent string
	if s.Host.AuthType == model.AuthTypeLocal {
		parent = filepath.Dir(s.CurrentDir)
	} else {
		parent = path.Dir(s.CurrentDir)
	}
	if parent == s.CurrentDir {
		s.mu.Unlock()
		return
	}
	s.CurrentDir = parent
	s.PathInput = parent
	s.selectedPath = ""
	s.mu.Unlock()
	s.RefreshFiles()
}

// NavigateHome 进入用户家目录 (~ 或 /root / /home/user)
func (s *TerminalSession) NavigateHome() {
	if s.Host.AuthType == model.AuthTypeLocal {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			s.NavigateTo(home)
			return
		}
		if runtime.GOOS == "windows" {
			s.NavigateTo("C:\\")
		} else {
			s.NavigateTo("/")
		}
		return
	}
	// 远程 SSH / SFTP 会话
	if s.Host.Username == "root" {
		s.NavigateTo("/root")
		return
	}
	if s.Host.Username != "" {
		s.NavigateTo("/home/" + s.Host.Username)
		return
	}
	s.NavigateTo("/")
}

// DownloadEntry 下载文件到本地 Downloads 目录
func (s *TerminalSession) DownloadEntry(entry sftpService.FileEntry) {
	if entry.IsDir || s.TransferMgr == nil {
		return
	}
	home, _ := os.UserHomeDir()
	downDir := filepath.Join(home, "Downloads")
	_, _ = s.TransferMgr.AddDownloadTask(entry.Path, downDir)
	s.ShowTransfer = true
}

// DeleteEntry 删除文件或空目录
func (s *TerminalSession) DeleteEntry(entry sftpService.FileEntry) {
	go func() {
		if s.Host.AuthType == model.AuthTypeLocal {
			if entry.IsDir {
				_ = os.RemoveAll(entry.Path)
			} else {
				_ = os.Remove(entry.Path)
			}
		} else if s.SFTPClient != nil {
			if entry.IsDir {
				_ = s.SFTPClient.RemoveDirectory(entry.Path)
			} else {
				_ = s.SFTPClient.Remove(entry.Path)
			}
		}
		s.RefreshFiles()
	}()
}

// Disconnect 主动断开当前终端连接通道，但保留该会话 Tab 供随时「重新连接」
func (s *TerminalSession) Disconnect() {
	s.mu.Lock()
	s.Connected = false
	s.Connecting = false
	s.ErrorMsg = "连接已主动断开"

	if s.TransferMgr != nil {
		s.TransferMgr.Close()
		s.TransferMgr = nil
	}
	if s.SFTPClient != nil {
		_ = s.SFTPClient.Close()
		s.SFTPClient = nil
	}
	if s.Term != nil {
		_ = s.Term.Close()
		s.Term = nil
	}
	s.scrollbar = terminalScrollbarState{}
	if s.TermConn != nil {
		_ = s.TermConn.Close()
		s.TermConn = nil
	}
	if s.Client != nil {
		_ = s.Client.Close()
		s.Client = nil
	}
	s.mu.Unlock()

	s.notifyUpdate()
}

// Close 关闭会话所有底层资源
func (s *TerminalSession) Close() {
	s.Disconnect()
}

// ShouldShowTransferQueue 返回当前会话是否需要展示传输队列按钮（本地终端无需展示）
func (s *TerminalSession) ShouldShowTransferQueue() bool {
	return s.Host != nil && s.Host.AuthType != model.AuthTypeLocal
}

// Render 在主工作区全高渲染终端界面（免除底部挤占，100% 满屏高度，深度复刻 docker-mate web 终端外观）
func (s *TerminalSession) Render(c *ui.Context, t *ui.Theme, onClose func(), onSwitchToFiles func(), onFocus func()) {
	ui.Column(c).Fill().Background(ColTerminalBg).Children(func() {
		// 1. 会话控制栏 (暗黑质感顶部会话状态栏，高度 36px，与终端融为一体)
		vStatusBadgeBg := ui.RGBA(34, 197, 94, 0.18)
		vStatusBadgeFg := ui.Hex("#4ADE80")
		vStatusDotColor := ui.Hex("#22C55E")
		vStatusText := "已连接"
		if s.Connecting {
			vStatusBadgeBg = ui.RGBA(245, 158, 11, 0.18)
			vStatusBadgeFg = ui.Hex("#FBBF24")
			vStatusDotColor = ui.Hex("#F59E0B")
			vStatusText = "连接中..."
		} else if !s.Connected {
			vStatusBadgeBg = ui.RGBA(239, 68, 68, 0.18)
			vStatusBadgeFg = ui.Hex("#F87171")
			vStatusDotColor = ui.Hex("#EF4444")
			vStatusText = "已断开"
		}

		sessionBar := ui.Row(c).Height(36).Shrink(0).Background(ColTerminalBg).Padding(0, t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2))
		if sessionBar.Clicked() && onFocus != nil {
			onFocus()
		}
		sessionBar.Children(func() {
			// 状态药丸徽章
			ui.Row(c).Background(vStatusBadgeBg).Padding(2, 8).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Box(c).Width(6).Height(6).Radius(3).Background(vStatusDotColor)
				ui.Text(c, vStatusText).FontSize(10.5).Bold().TextColor(vStatusBadgeFg)
			})

			// 主机名与网络地址 (暗黑终端高对比度文字)
			ui.Text(c, s.Host.Name).FontSize(12).Bold().TextColor(ui.Hex("#F4F4F5"))
			if s.Host.AuthType != model.AuthTypeLocal {
				ui.Text(c, fmt.Sprintf("(%s:%d)", s.Host.Address, s.Host.Port)).FontSize(11).TextColor(ui.Hex("#A1A1AA"))
			}

			ui.Spacer(c)

			// 传输队列开关按钮（仅远程主机展示，本地终端不涉及远程传输）
			if s.ShouldShowTransferQueue() {
				var taskCount int
				var taskSpeed string
				if s.TransferMgr != nil {
					taskCount, taskSpeed = s.TransferMgr.GetActiveStats()
				}
				queueText := "传输队列"
				if taskCount > 0 {
					queueText = fmt.Sprintf("传输中 (%d) %s", taskCount, taskSpeed)
				}
				// 深色工具栏按钮：展开和悬停时用柔和底色提示状态
				queueBtn := ui.ButtonBase(c).Height(24).Shrink(0).Padding(0, 8).Radius(4).
					Background(ui.Transparent).AlignItems(ui.Center).Gap(5).Label("传输队列")
				queueColor := ui.Hex("#A1A1AA")
				if s.ShowTransfer {
					queueBtn.Background(ui.Hex("#27272A")).Tooltip("收起传输队列")
					queueColor = ui.Hex("#D4D4D8")
				} else {
					queueBtn.Tooltip("展开传输队列")
				}
				if queueBtn.Hovered() {
					queueBtn.Background(ui.Hex("#323238"))
					queueColor = ui.Hex("#F4F4F5")
				}
				if taskCount > 0 {
					queueColor = ui.Hex("#93B4D8")
				}
				queueBtn.Children(func() {
					ui.Icon(c, IconTransfer).Width(13).Height(13).TextColor(queueColor)
					ui.Text(c, queueText).FontSize(11).TextColor(queueColor).SingleLine()
				})
				if queueBtn.Clicked() {
					s.ShowTransfer = !s.ShowTransfer
				}
			}

			// 断开状态下的重连按钮
			if !s.Connected && !s.Connecting {
				if terminalPrimaryButton(c, "重新连接").Clicked() {
					s.Start()
				}
			}
		})

		// 2. 核心工作视窗 (满高渲染 Ghostty 终端，采用暗黑终端背景)
		ui.Column(c).Grow(1).Shrink(1).Background(ColTerminalBg).Children(func() {
			if s.Connecting {
				ui.Column(c).Grow(1).Center().Gap(t.Space(2)).Children(func() {
					ui.Box(c).Width(8).Height(8).Radius(4).Background(t.Warning)
					connMsg := fmt.Sprintf("正在连接至 %s:%d ...", s.Host.Address, s.Host.Port)
					if s.Host.AuthType == model.AuthTypeLocal {
						connMsg = "正在启动本地终端..."
					}
					ui.Text(c, connMsg).FontSize(13).Bold().TextColor(ui.Hex("#F4F4F5"))
					if ui.Button(c, "取消连接").Clicked() {
						s.Close()
					}
				})
			} else if !s.Connected && s.ErrorMsg != "" {
				ui.Column(c).Grow(1).Center().Gap(t.Space(3)).Children(func() {
					ui.Box(c).Width(10).Height(10).Radius(5).Background(t.Danger)
					ui.Text(c, "会话连接中断").FontSize(15).Bold().TextColor(t.Danger)
					ui.Text(c, s.ErrorMsg).FontSize(12).TextColor(ColTerminalMuted)
					ui.Row(c).Gap(t.Space(2)).Children(func() {
						if terminalPrimaryButton(c, "重新连接").Clicked() {
							s.Start()
						}
						if ui.Button(c, "关闭标签页").Clicked() {
							s.Close()
							if onClose != nil {
								onClose()
							}
						}
					})
				})
			} else {
				// Ghostty VT 终端 GPU 自绘（占据主屏完整可用高度）
				if s.Term != nil {
					termElem := terminal.View(c, s.Term).Grow(1)
					attachTerminalScrollbar(c, termElem, s.Term, &s.scrollbar, onFocus)
					if termElem.Focused() && onFocus != nil {
						onFocus()
					}
				}
			}
		})

		// 3. 传输队列抽屉 (底部浮动展示)
		if s.ShouldShowTransferQueue() && s.ShowTransfer && s.TransferMgr != nil {
			s.renderTransferDrawer(c, t)
		}
	})
}

// RenderSidebarFileExplorer 在左侧边栏渲染现代文件目录浏览器 (左侧带 r/d 标识，支持双击目录进入/双击文件就地编辑，右键完整菜单)
func (s *TerminalSession) RenderSidebarFileExplorer(c *ui.Context, t *ui.Theme, onOpenFile func(entry sftpService.FileEntry)) {
	ui.Column(c).Grow(1).Gap(t.Space(1.5)).Children(func() {
		// 1. 路径输入框 (单层原生边框，对齐无偏移，回车即可跳转)
		pathInput := ui.TextInput(c, &s.PathInput).Placeholder("输入路径按回车跳转...")
		if pathInput.Submitted() {
			target := strings.TrimSpace(s.PathInput)
			if target != "" {
				s.NavigateTo(target)
			}
		}

		// 2. 快捷导航图标工具栏 (复刻桌面原生 Ghost 图标按钮，告别厚重网页 Button 质感)
		ui.Row(c).Height(26).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "快捷导航").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)

			ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
				// 辅助渲染扁平图标按钮
				renderToolBtn := func(tooltip string, icon *ui.SVG, isActive bool, onClick func()) {
					btn := ui.ButtonBase(c).Tooltip(tooltip).Width(24).Height(24).Radius(t.Radius - 2).Center()
					iconCol := ColSidebarText
					if isActive {
						btn.Background(ColSidebarActive)
						iconCol = ColSidebarActiveText
					}
					if btn.Hovered() {
						if isActive {
							btn.Background(ColSidebarActive)
						} else {
							btn.Background(ColSidebarHover)
							iconCol = t.Text
						}
					}
					btn.Children(func() {
						ui.Icon(c, icon).Width(13).Height(13).TextColor(iconCol)
					})
					if btn.Clicked() {
						onClick()
					}
				}

				// 返回上级
				renderToolBtn("返回上级", IconArrowUp, false, func() {
					s.NavigateUp()
				})

				// 根目录 (/)
				renderToolBtn("根目录 (/)", IconHardDrive, false, func() {
					s.NavigateTo("/")
				})

				// 家目录 (~)
				renderToolBtn("家目录 (~)", IconHome, false, func() {
					s.NavigateHome()
				})

				// 刷新列表
				renderToolBtn("刷新列表", IconRefresh, false, func() {
					s.RefreshFiles()
				})

				// 显示/隐藏隐藏文件，激活时使用中性灰底色。
				hiddenTooltip := "显示隐藏文件"
				hiddenIcon := IconEyeOff
				if s.ShowHidden {
					hiddenTooltip = "隐藏隐藏文件"
					hiddenIcon = IconEye
				}
				renderToolBtn(hiddenTooltip, hiddenIcon, s.ShowHidden, func() {
					s.ShowHidden = !s.ShowHidden
				})
			})
		})

		if s.FileError != "" {
			ui.Row(c).Background(ui.Hex("#FEE2E2")).Border(1, ui.Hex("#FCA5A5")).Padding(t.Space(1), t.Space(2)).Radius(t.Radius).Children(func() {
				ui.Text(c, "错误: "+s.FileError).FontSize(10.5).TextColor(t.Danger)
			})
		}

		ui.Box(c).Height(1).Background(ColSidebarBorder)

		// 紧凑文件列表 (滚动视图，每行 26px，使用左侧 r/d 语义标记，右键操作菜单)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				files := s.VisibleFiles()
				if len(files) == 0 {
					ui.Column(c).Padding(t.Space(4)).Center().Gap(t.Space(1)).Children(func() {
						if len(s.FileList) > 0 {
							ui.Text(c, "当前仅含隐藏文件").FontSize(11).TextColor(t.TextMuted)
							ui.Text(c, "点击上方眼睛按钮可开启显示").FontSize(10.5).TextColor(t.TextMuted)
						} else if !s.FileLoading {
							ui.Text(c, "目录为空").FontSize(11).TextColor(t.TextMuted)
						}
					})
				}

				for _, entry := range files {
					item := entry
					isHidden := strings.HasPrefix(item.Name, ".")
					isSel := s.selectedPath == item.Path

					rowBg := ui.Transparent
					borderCol := ui.Transparent
					if isSel {
						rowBg = ColSidebarActive
						borderCol = ColSidebarActive
					}

					row := ui.Row(c).Height(26).Background(rowBg).Border(1, borderCol).Padding(0, t.Space(1.5)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5))
					if !isSel && row.Hovered() {
						row.Background(ColSidebarHover)
					}

					isDir := item.IsDir
					// 防御性校验：若标记为文件但本地真实目标为目录（如软链接），自动识别为目录
					if !isDir && s.Host.AuthType == model.AuthTypeLocal {
						if fi, err := os.Stat(item.Path); err == nil && fi.IsDir() {
							isDir = true
						}
					}

					// 右键操作菜单（涵盖删除、编辑、下载等）
					row.ContextMenu(func(m *ui.Menu) {
						if isDir {
							if m.Item("进入目录").Chosen() {
								s.NavigateTo(item.Path)
							}
						} else {
							if m.Item("编辑文件").Chosen() {
								if onOpenFile != nil {
									onOpenFile(item)
								}
							}
							if s.ShouldShowTransferQueue() {
								if m.Item("下载到本地 (~/Downloads)").Chosen() {
									s.DownloadEntry(item)
								}
							}
						}
						m.Separator()
						if s.ShowHidden {
							if m.Item("隐藏隐藏文件").Chosen() {
								s.ShowHidden = false
							}
						} else {
							if m.Item("显示隐藏文件").Chosen() {
								s.ShowHidden = true
							}
						}
						m.Separator()
						if m.Item("删除").Chosen() {
							s.DeleteEntry(item)
						}
						m.Separator()
						if m.Item("刷新列表").Chosen() {
							s.RefreshFiles()
						}
					})

					// 双击进入目录或就地编辑文件
					if row.DoubleClicked() {
						if isDir {
							s.NavigateTo(item.Path)
						} else {
							if onOpenFile != nil {
								onOpenFile(item)
							}
						}
					}

					if row.Clicked() {
						s.selectedPath = item.Path
					}

					row.Children(func() {
						// 1. 文件左侧用 r、d 标识是文件还是目录（d 为目录采用黄色徽章，r 为常规文件采用静音灰色）
						if isDir {
							ui.Row(c).Width(16).Height(16).Radius(2).Background(ui.Hex("#FEF3C7")).Center().Children(func() {
								ui.Text(c, "d").FontSize(10.5).Bold().TextColor(ui.Hex("#D97706"))
							})
						} else {
							ui.Row(c).Width(16).Height(16).Radius(2).Background(ColBadgeGrayBg).Center().Children(func() {
								ui.Text(c, "r").FontSize(10.5).TextColor(t.TextMuted)
							})
						}

						// 2. 文件/目录名称保持单行，超宽省略，悬停显示完整名称
						nameCol := t.Text
						if isHidden {
							nameCol = t.TextMuted
						}
						nameElem := ui.Text(c, item.Name).FontSize(11).TextColor(nameCol).Grow(1).Shrink(1).MinWidth(0).SingleLine().Tooltip(item.Name)
						if item.IsDir {
							nameElem.Bold()
						}

						// 3. 文件大小 (右侧只显示大小，无进入/删除等按钮)
						if !item.IsDir {
							ui.Text(c, item.SizeHuman).FontSize(10).TextColor(t.TextMuted).Shrink(0).NoWrap()
						}
					})
				}
			})
		})
	})
}

// 渲染与终端工具栏配套的紧凑深色传输面板
func (s *TerminalSession) renderTransferDrawer(c *ui.Context, t *ui.Theme) {
	tasks := s.TransferMgr.GetTasks()
	textColor := ui.Hex("#D4D4D8")
	mutedColor := ui.Hex("#A1A1AA")
	// 最多展示四行任务，空队列只占一行内容高度
	visibleRows := min(max(len(tasks), 1), 4)
	panelHeight := float32(29 + visibleRows*28)

	ui.Column(c).Height(panelHeight).Shrink(0).Background(ColTerminalBg).Children(func() {
		ui.Box(c).Height(1).Shrink(0).Background(ui.Hex("#323238"))
		ui.Row(c).Height(28).Shrink(0).Padding(0, t.Space(3)).AlignItems(ui.Center).Gap(5).Children(func() {
			ui.Icon(c, IconTransfer).TextColor(mutedColor).Width(13).Height(13)
			ui.Text(c, "传输队列").FontSize(11).TextColor(textColor).SingleLine()
			ui.Text(c, fmt.Sprintf("(%d)", len(tasks))).FontSize(10.5).TextColor(mutedColor)
			ui.Spacer(c)
			closeBtn := ui.ButtonBase(c).Width(24).Height(24).Radius(4).Center().
				Tooltip("收起传输队列").Label("收起传输队列")
			closeColor := mutedColor
			if closeBtn.Hovered() {
				closeBtn.Background(ui.Hex("#323238"))
				closeColor = ui.Hex("#F4F4F5")
			}
			closeBtn.Children(func() {
				ui.Icon(c, IconChevronDown).Width(13).Height(13).TextColor(closeColor)
			})
			if closeBtn.Clicked() {
				s.ShowTransfer = false
			}
		})

		ui.Scroll(c).Grow(1).Shrink(1).MinHeight(0).Children(func() {
			ui.Column(c).Children(func() {
				if len(tasks) == 0 {
					ui.Row(c).Height(28).Padding(0, t.Space(3)).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "暂无传输任务").FontSize(11).TextColor(mutedColor)
					})
				}

				for _, task := range tasks {
					item := task
					typeColor := ui.Hex("#93B4D8")
					if item.Type == transfer.TaskTypeDownload {
						typeColor = ui.Hex("#B6A4D6")
					}
					row := ui.Row(c).Height(28).Shrink(0).Padding(0, t.Space(3)).AlignItems(ui.Center).Gap(8)
					if row.Hovered() {
						row.Background(ui.Hex("#27272A"))
					}
					row.Children(func() {
						ui.Text(c, string(item.Type)).FontSize(10).TextColor(typeColor).Shrink(0).SingleLine()
						ui.Text(c, item.FileName).FontSize(11).TextColor(textColor).
							Grow(1).Shrink(1).MinWidth(0).SingleLine().Tooltip(item.FileName)

						progressText := fmt.Sprintf("%.1f%% (%s)", item.Percent, item.SpeedHuman)
						progressColor := mutedColor
						switch item.Status {
						case transfer.StatusPending:
							progressText = "等待中"
						case transfer.StatusCompleted:
							progressText = "已完成"
						case transfer.StatusFailed:
							progressText = "失败: " + item.ErrorMsg
							progressColor = ui.Hex("#F87171")
						case transfer.StatusCanceled:
							progressText = "已取消"
						}
						ui.Text(c, progressText).FontSize(10.5).TextColor(progressColor).
							Width(160).Shrink(1).MinWidth(0).SingleLine().Tooltip(progressText)

						// 预留相同操作宽度，保持各行进度对齐
						ui.Row(c).Width(36).Shrink(0).Children(func() {
							if item.Status == transfer.StatusRunning || item.Status == transfer.StatusPending {
								cancelBtn := ui.ButtonBase(c).Width(36).Height(22).Radius(4).Center().Tooltip("取消传输")
								cancelColor := mutedColor
								if cancelBtn.Hovered() {
									cancelBtn.Background(ui.Hex("#323238"))
									cancelColor = ui.Hex("#F4F4F5")
								}
								cancelBtn.Children(func() {
									ui.Text(c, "取消").FontSize(10.5).TextColor(cancelColor).SingleLine()
								})
								if cancelBtn.Clicked() {
									s.TransferMgr.CancelTask(item.ID)
								}
							}
						})
					})
				}
			})
		})
	})
}
