package ui

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/google/uuid"

	"goshell/internal/core/model"
	sftpService "goshell/internal/core/sftp"
	sshClient "goshell/internal/core/ssh"
	"goshell/internal/core/transfer"
)

// TerminalSession 代表一个活跃的终端会话实例，整合 SSH 终端与 SFTP 文件服务
type TerminalSession struct {
	ID        string
	Host      *model.Host
	Title     string
	Term      *terminal.Terminal
	Client    *sshClient.Client
	TermConn  *sshClient.TerminalConn

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

	Connecting bool
	Connected  bool
	ErrorMsg   string

	StartTime time.Time
	mu        sync.Mutex
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

	// 1. 本地 Shell 与本地文件系统
	if s.Host.AuthType == model.AuthTypeLocal {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "/"
		}
		s.CurrentDir = home
		s.PathInput = home

		term, err := terminal.New(terminal.Options{
			Theme: terminal.LightTheme(),
			OnTitle: func(title string) {
				if title != "" {
					s.Title = title
				}
			},
			OnExit: func(code int) {
				s.mu.Lock()
				s.Connected = false
				s.ErrorMsg = fmt.Sprintf("本地终端已退出 (退出码: %d)", code)
				s.mu.Unlock()
			},
		})
		if err != nil {
			s.mu.Lock()
			s.Connecting = false
			s.ErrorMsg = fmt.Sprintf("启动终端引擎失败: %v", err)
			s.mu.Unlock()
			return
		}

		s.mu.Lock()
		s.Term = term
		s.Connecting = false
		s.Connected = true
		s.mu.Unlock()

		s.RefreshFiles()
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
			return
		}

		termConn, err := client.OpenTerminal(100, 30)
		if err != nil {
			_ = client.Close()
			s.mu.Lock()
			s.Connecting = false
			s.ErrorMsg = fmt.Sprintf("申请 PTY 终端通道失败: %v", err)
			s.mu.Unlock()
			return
		}

		term, err := terminal.New(terminal.Options{
			Conn:  termConn,
			Theme: terminal.LightTheme(),
			OnTitle: func(title string) {
				if title != "" {
					s.Title = title
				}
			},
			OnExit: func(code int) {
				s.mu.Lock()
				s.Connected = false
				s.ErrorMsg = fmt.Sprintf("远程连接已关闭 (退出码: %d)", code)
				s.mu.Unlock()
			},
		})
		if err != nil {
			_ = termConn.Close()
			_ = client.Close()
			s.mu.Lock()
			s.Connecting = false
			s.ErrorMsg = fmt.Sprintf("初始化终端引擎失败: %v", err)
			s.mu.Unlock()
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
					entries = append(entries, sftpService.FileEntry{
						Name:      e.Name(),
						Path:      filepath.Join(dir, e.Name()),
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
		defer s.mu.Unlock()
		s.FileLoading = false
		if err != nil {
			s.FileError = err.Error()
		} else {
			s.FileList = entries
		}
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
	parent := path.Dir(s.CurrentDir)
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
		s.NavigateTo("/")
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

// Close 关闭会话所有底层资源
func (s *TerminalSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Connected = false
	s.Connecting = false

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
	if s.TermConn != nil {
		_ = s.TermConn.Close()
		s.TermConn = nil
	}
	if s.Client != nil {
		_ = s.Client.Close()
		s.Client = nil
	}
}

// Render 在主工作区全高渲染终端界面（免除底部挤占，100% 满屏高度，深度复刻 docker-mate web 终端外观）
func (s *TerminalSession) Render(c *ui.Context, t *ui.Theme, onClose func(), onSwitchToFiles func()) {
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// 1. 会话控制栏 (复刻 docker-mate 的顶部会话状态栏，高度 36px)
		vStatusBadgeBg := ColBadgeGreenBg
		vStatusBadgeFg := ColBadgeGreenText
		vStatusDotColor := t.Success
		vStatusText := "已连接"
		if s.Connecting {
			vStatusBadgeBg = ColBadgeAmberBg
			vStatusBadgeFg = ColBadgeAmberText
			vStatusDotColor = t.Warning
			vStatusText = "连接中..."
		} else if !s.Connected {
			vStatusBadgeBg = ui.Hex("#FEE2E2")
			vStatusBadgeFg = t.Danger
			vStatusDotColor = t.Danger
			vStatusText = "已断开"
		}

		ui.Row(c).Height(36).Background(t.Surface).Border(1, t.Border).Padding(0, t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			// 状态药丸徽章
			ui.Row(c).Background(vStatusBadgeBg).Padding(2, 8).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Box(c).Width(6).Height(6).Radius(3).Background(vStatusDotColor)
				ui.Text(c, vStatusText).FontSize(10.5).Bold().TextColor(vStatusBadgeFg)
			})

			// 主机名与网络地址
			ui.Text(c, s.Host.Name).FontSize(12).Bold().TextColor(t.Text)
			if s.Host.AuthType != model.AuthTypeLocal {
				ui.Text(c, fmt.Sprintf("(%s:%d)", s.Host.Address, s.Host.Port)).FontSize(11).TextColor(t.TextMuted)
			}

			ui.Spacer(c)

			// 传输队列开关按钮
			var taskCount int
			var taskSpeed string
			if s.TransferMgr != nil {
				taskCount, taskSpeed = s.TransferMgr.GetActiveStats()
			}
			queueText := "传输队列"
			if taskCount > 0 {
				queueText = fmt.Sprintf("传输中 (%d) %s", taskCount, taskSpeed)
			}
			if ui.Button(c, queueText).Clicked() {
				s.ShowTransfer = !s.ShowTransfer
			}

			// 断开状态下的重连按钮
			if !s.Connected && !s.Connecting {
				if ui.PrimaryButton(c, "重新连接").Clicked() {
					s.Start()
				}
			}
		})

		// 2. 核心工作视窗 (满高渲染 Ghostty 终端，采用暗黑主题与微细圆角边框)
		if s.Connecting {
			ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
				ui.Box(c).Width(8).Height(8).Radius(4).Background(t.Warning)
				ui.Text(c, fmt.Sprintf("正在建立安全通道至 %s:%d ...", s.Host.Address, s.Host.Port)).FontSize(13).Bold().TextColor(t.Text)
				ui.Text(c, "通过加密通道验证身份与分配 PTY 终端").FontSize(11).TextColor(t.TextMuted)
				if ui.Button(c, "取消连接").Clicked() {
					s.Close()
				}
			})
		} else if !s.Connected && s.ErrorMsg != "" {
			ui.Column(c).Fill().Center().Gap(t.Space(3)).Children(func() {
				ui.Box(c).Width(10).Height(10).Radius(5).Background(t.Danger)
				ui.Text(c, "会话连接中断").FontSize(15).Bold().TextColor(t.Danger)
				ui.Text(c, s.ErrorMsg).FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					if ui.PrimaryButton(c, "重新连接").Clicked() {
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
				terminal.View(c, s.Term).Fill().AutoFocus()
			}
		}

		// 3. 传输队列抽屉 (底部浮动展示)
		if s.ShowTransfer && s.TransferMgr != nil {
			s.renderTransferDrawer(c, t)
		}
	})
}

// RenderSidebarFileExplorer 在左侧边栏渲染现代文件目录浏览器 (左侧带 r/d 标识，支持双击目录进入/双击文件就地编辑，右键完整菜单)
func (s *TerminalSession) RenderSidebarFileExplorer(c *ui.Context, t *ui.Theme, onOpenFile func(entry sftpService.FileEntry)) {
	ui.Column(c).Fill().Background(ColSidebarBg).Padding(t.Space(1.5)).Gap(t.Space(1.5)).Children(func() {
		// 1. 路径输入框 (单层原生边框，对齐无偏移，回车即可跳转)
		pathInput := ui.TextInput(c, &s.PathInput).Placeholder("输入路径按回车跳转...")
		if pathInput.Submitted() {
			target := strings.TrimSpace(s.PathInput)
			if target != "" {
				s.NavigateTo(target)
			}
		}

		// 2. 快捷导航图标按钮组 (4 个图标按钮：返回上级、根目录、家目录、刷新，hover 时显示文字)
		ui.Row(c).Height(26).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			// 返回上级
			btnUp := ui.Button(c, "").Tooltip("返回上级").Grow(1).Height(26).Center()
			btnUp.Children(func() {
				ui.Icon(c, IconArrowUp).Width(13).Height(13)
			})
			if btnUp.Clicked() {
				s.NavigateUp()
			}

			// 根目录 (/)
			btnRoot := ui.Button(c, "").Tooltip("根目录 (/)").Grow(1).Height(26).Center()
			btnRoot.Children(func() {
				ui.Icon(c, IconHardDrive).Width(13).Height(13)
			})
			if btnRoot.Clicked() {
				s.NavigateTo("/")
			}

			// 家目录 (~)
			btnHome := ui.Button(c, "").Tooltip("家目录 (~)").Grow(1).Height(26).Center()
			btnHome.Children(func() {
				ui.Icon(c, IconHome).Width(13).Height(13)
			})
			if btnHome.Clicked() {
				s.NavigateHome()
			}

			// 刷新
			btnRefresh := ui.Button(c, "").Tooltip("刷新列表").Grow(1).Height(26).Center()
			btnRefresh.Children(func() {
				ui.Icon(c, IconRefresh).Width(13).Height(13)
			})
			if btnRefresh.Clicked() {
				s.RefreshFiles()
			}
		})

		if s.FileError != "" {
			ui.Row(c).Background(ui.Hex("#FEE2E2")).Border(1, ui.Hex("#FCA5A5")).Padding(t.Space(1), t.Space(2)).Radius(t.Radius).Children(func() {
				ui.Text(c, "错误: "+s.FileError).FontSize(10.5).TextColor(t.Danger)
			})
		}

		ui.Box(c).Height(1).Background(ColSidebarBorder)

		// 紧凑文件列表 (滚动视图，每行 26px，使用左侧 r/d 语义标记，右键操作菜单)
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				for _, entry := range s.FileList {
					item := entry
					isSel := s.selectedPath == item.Path

					rowBg := ui.Transparent
					borderCol := ui.Transparent
					if isSel {
						rowBg = ColSidebarActive
						borderCol = ui.Hex("#B8C8D8")
					}

					row := ui.Row(c).Height(26).Background(rowBg).Border(1, borderCol).Padding(0, t.Space(1.5)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5))

					// 右键操作菜单（涵盖删除、编辑、下载等）
					row.ContextMenu(func(m *ui.Menu) {
						if item.IsDir {
							if m.Item("进入目录").Chosen() {
								s.NavigateTo(item.Path)
							}
						} else {
							if m.Item("编辑文件").Chosen() {
								if onOpenFile != nil {
									onOpenFile(item)
								}
							}
							if m.Item("下载到本地 (~/Downloads)").Chosen() {
								s.DownloadEntry(item)
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
						if item.IsDir {
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
						// 1. 文件左侧用 r、d 标识是文件还是目录（d 为目录采用主题强调色，r 为常规文件采用静音色）
						if item.IsDir {
							ui.Row(c).Width(16).Height(16).Radius(2).Background(ui.Hex("#EFF6FF")).Center().Children(func() {
								ui.Text(c, "d").FontSize(10.5).Bold().TextColor(t.Accent)
							})
						} else {
							ui.Row(c).Width(16).Height(16).Radius(2).Background(ui.Hex("#F1F5F9")).Center().Children(func() {
								ui.Text(c, "r").FontSize(10.5).TextColor(t.TextMuted)
							})
						}

						// 2. 文件/目录名称
						nameElem := ui.Text(c, item.Name).FontSize(11).TextColor(t.Text).Grow(1)
						if item.IsDir {
							nameElem.Bold()
						}

						// 3. 文件大小 (右侧只显示大小，无进入/删除等按钮)
						if !item.IsDir {
							ui.Text(c, item.SizeHuman).FontSize(10).TextColor(t.TextMuted)
						}
					})
				}
			})
		})
	})
}

// 渲染传输队列抽屉 (底部浮动展示，复刻 docker-mate 的 Card / Table 风格)
func (s *TerminalSession) renderTransferDrawer(c *ui.Context, t *ui.Theme) {
	tasks := s.TransferMgr.GetTasks()

	ui.Column(c).Height(150).Background(t.Surface).Border(1, t.Border).Padding(t.Space(2), t.Space(3)).Gap(t.Space(1.5)).Children(func() {
		// 抽屉头部
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, IconHardDrive).TextColor(t.Accent).Width(14).Height(14)
			ui.Text(c, fmt.Sprintf(" 后台传输队列 (共 %d 项)", len(tasks))).FontSize(12).Bold().TextColor(t.Text)
			ui.Spacer(c)
			if ui.Button(c, "隐藏抽屉").Clicked() {
				s.ShowTransfer = false
			}
		})

		ui.Box(c).Height(1).Background(t.Border)

		// 任务列表
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(t.Space(1)).Children(func() {
				if len(tasks) == 0 {
					ui.Text(c, "暂无进行中的文件传输任务").FontSize(11).TextColor(t.TextMuted)
				}

				for _, task := range tasks {
					item := task
					typeBg := ColBadgeBlueBg
					typeFg := ColBadgeBlueText
					if item.Type == transfer.TaskTypeDownload {
						typeBg = ColBadgePurpleBg
						typeFg = ColBadgePurpleText
					}

					ui.Row(c).Height(26).Background(ui.Hex("#FAFBFC")).Border(1, t.Border).Padding(0, t.Space(2)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						// 传输类型药丸
						ui.Row(c).Background(typeBg).Padding(1, 6).Radius(3).Children(func() {
							ui.Text(c, string(item.Type)).FontSize(10).Bold().TextColor(typeFg)
						})

						ui.Text(c, item.FileName).FontSize(11).Bold().TextColor(t.Text).Width(180)

						progressText := fmt.Sprintf("%.1f%% (%s)", item.Percent, item.SpeedHuman)
						if item.Status == transfer.StatusCompleted {
							progressText = "已完成"
						} else if item.Status == transfer.StatusFailed {
							progressText = "失败: " + item.ErrorMsg
						} else if item.Status == transfer.StatusCanceled {
							progressText = "已取消"
						}
						ui.Text(c, progressText).FontSize(10.5).TextColor(t.TextMuted).Width(160)

						ui.Spacer(c)

						if item.Status == transfer.StatusRunning || item.Status == transfer.StatusPending {
							if ui.Button(c, "取消").Clicked() {
								s.TransferMgr.CancelTask(item.ID)
							}
						}
					})
				}
			})
		})
	})
}

