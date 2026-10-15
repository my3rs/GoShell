package ui

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
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

// TerminalSession 代表一个活跃的终端会话实例，整合 SSH 终端与 SFTP 文件浏览器
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
	FileList    []sftpService.FileEntry
	FileLoading bool
	FileError   string

	// UI 交互与分屏控制
	ShowSFTP       bool
	ShowTransfer   bool
	termHeight     float32
	selectedPath   string

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
		ShowSFTP:   true,
		termHeight: 380, // 终端默认高度 380px，下方分屏留给 SFTP
		StartTime:  time.Now(),
		CurrentDir: "/",
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
			// 本地文件系统扫描
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
			// 远程 SFTP 文件扫描
			if s.SFTPClient != nil {
				entries, err = s.SFTPClient.ListDirectory(dir)
			} else {
				err = fmt.Errorf("SFTP 子信道未建立")
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

// NavigateTo 进入指定子目录
func (s *TerminalSession) NavigateTo(targetPath string) {
	s.mu.Lock()
	s.CurrentDir = targetPath
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
	s.selectedPath = ""
	s.mu.Unlock()
	s.RefreshFiles()
}

// DownloadEntry 下载远程文件到本地 Downloads 目录
func (s *TerminalSession) DownloadEntry(entry sftpService.FileEntry) {
	if entry.IsDir || s.TransferMgr == nil {
		return
	}
	home, _ := os.UserHomeDir()
	downDir := filepath.Join(home, "Downloads")
	_, _ = s.TransferMgr.AddDownloadTask(entry.Path, downDir)
	s.ShowTransfer = true
}

// DeleteEntry 删除文件
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

// Close 关闭会话所有资源
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

// Render 渲染主会话面板（支持上下垂直分屏：上终端 + 下 SFTP）
func (s *TerminalSession) Render(c *ui.Context, t *ui.Theme, onClose func()) {
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// 1. 会话控制栏 (28px 紧凑高度)
		vStatusColor := t.Success
		vStatusText := "已连接"
		if s.Connecting {
			vStatusColor = t.Warning
			vStatusText = "正在连接..."
		} else if !s.Connected {
			vStatusColor = t.Danger
			vStatusText = "已断开"
		}

		ui.Row(c).Height(28).Background(ui.Hex("#F8F9FA")).Border(1, t.Border).Padding(0, t.Space(3)).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(6).Height(6).Radius(3).Background(vStatusColor)
			ui.Text(c, fmt.Sprintf(" %s | %s (%s:%d)", vStatusText, s.Host.Name, s.Host.Address, s.Host.Port)).FontSize(11).TextColor(t.TextMuted)

			ui.Spacer(c)

			// SFTP 面板折叠/展开开关
			sftpToggleText := "显示文件管理"
			if s.ShowSFTP {
				sftpToggleText = "隐藏文件管理"
			}
			if ui.Button(c, sftpToggleText).Clicked() {
				s.ShowSFTP = !s.ShowSFTP
			}

			// 传输队列开关
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

			if !s.Connected && !s.Connecting {
				if ui.Button(c, "重新连接").Clicked() {
					s.Start()
				}
			}

			if ui.Button(c, "关闭会话").Clicked() {
				s.Close()
				if onClose != nil {
					onClose()
				}
			}
		})

		// 2. 核心工作视窗
		if s.Connecting {
			ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
				ui.Box(c).Width(8).Height(8).Radius(4).Background(t.Warning)
				ui.Text(c, fmt.Sprintf("正在建立安全通道至 %s:%s ...", s.Host.Address, strconv.Itoa(s.Host.Port))).FontSize(13).Bold().TextColor(t.Text)
				ui.Text(c, "同时初始化 PTY 交互终端与独立 SFTP 文件服务").FontSize(11).TextColor(t.TextMuted)
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
			// 终端与 SFTP 垂直上下分屏 (SplitVertical)
			if s.ShowSFTP {
				ui.SplitVertical(c, &s.termHeight, func() {
					if s.Term != nil {
						terminal.View(c, s.Term).Fill().AutoFocus()
					}
				}, func() {
					s.renderSFTPExplorer(c, t)
				}).Fill()
			} else {
				if s.Term != nil {
					terminal.View(c, s.Term).Fill().AutoFocus()
				}
			}
		}

		// 3. 传输队列抽屉 (底部浮动展示)
		if s.ShowTransfer && s.TransferMgr != nil {
			s.renderTransferDrawer(c, t)
		}
	})
}

// 渲染 SFTP 资源管理器面板 (下方视窗，包含面包屑导航与文件列表表格)
func (s *TerminalSession) renderSFTPExplorer(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(t.Surface).Border(1, t.Border).Children(func() {
		// 导航工具栏 (紧凑 32px)
		ui.Row(c).Height(32).Background(ui.Hex("#F8F9FA")).Border(1, t.Border).Padding(0, t.Space(2.5)).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "文件浏览:").FontSize(11).Bold().TextColor(t.TextMuted)

			// 路径导航条
			ui.Row(c).Background(ui.Hex("#FFFFFF")).Border(1, t.Border).Padding(t.Space(0.5), t.Space(2)).Radius(t.Radius).AlignItems(ui.Center).Children(func() {
				ui.Text(c, s.CurrentDir).FontSize(11).Bold().TextColor(t.Text)
			})

			ui.Box(c).Width(1).Height(14).Background(t.Border)

			if ui.Button(c, "返回上级").Clicked() {
				s.NavigateUp()
			}

			if ui.Button(c, "根目录 /").Clicked() {
				s.NavigateTo("/")
			}

			if ui.Button(c, "刷新").Clicked() {
				s.RefreshFiles()
			}

			ui.Spacer(c)

			if s.FileLoading {
				ui.Text(c, "正在读取目录...").FontSize(11).TextColor(t.Warning)
			} else {
				ui.Text(c, fmt.Sprintf("共 %d 项", len(s.FileList))).FontSize(11).TextColor(t.TextMuted)
			}
		})

		// 报错提示
		if s.FileError != "" {
			ui.Row(c).Height(24).Background(ui.Hex("#FEE2E2")).Padding(0, t.Space(3)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "访问错误: "+s.FileError).FontSize(11).TextColor(t.Danger)
			})
		}

		// 文件表格列头 (24px)
		ui.Row(c).Height(24).Background(ui.Hex("#F1F3F5")).Border(1, t.Border).Padding(0, t.Space(3)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "文件名称").FontSize(11).Bold().TextColor(t.TextMuted).Width(320)
			ui.Text(c, "文件大小").FontSize(11).Bold().TextColor(t.TextMuted).Width(100)
			ui.Text(c, "权限属性").FontSize(11).Bold().TextColor(t.TextMuted).Width(110)
			ui.Text(c, "修改时间").FontSize(11).Bold().TextColor(t.TextMuted).Width(140)
			ui.Spacer(c)
			ui.Text(c, "操作").FontSize(11).Bold().TextColor(t.TextMuted).Width(90)
		})

		// 滚动文件列表
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Children(func() {
				for _, entry := range s.FileList {
					item := entry // 闭包变量捕获
					isSel := s.selectedPath == item.Path

					rowBg := t.Surface
					if isSel {
						rowBg = ui.Hex("#EFF6FF")
					}

					ui.Row(c).Height(26).Background(rowBg).Border(1, ui.Hex("#F1F3F5")).Padding(0, t.Space(3)).AlignItems(ui.Center).Children(func() {
						// 名称与类型指示
						typeLabel := "[文件]"
						typeColor := t.Text
						if item.IsDir {
							typeLabel = "[目录]"
							typeColor = t.Accent
						}

						ui.Row(c).Width(320).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
							ui.Text(c, typeLabel).FontSize(10).TextColor(typeColor)
							ui.Text(c, item.Name).FontSize(12).TextColor(typeColor).Bold()
						})

						// 大小
						ui.Text(c, item.SizeHuman).FontSize(11).TextColor(t.TextMuted).Width(100)

						// 权限
						ui.Text(c, item.Mode).FontSize(11).TextColor(t.TextMuted).Width(110)

						// 修改时间
						ui.Text(c, item.ModTime.Format("2006-01-02 15:04")).FontSize(11).TextColor(t.TextMuted).Width(140)

						ui.Spacer(c)

						// 操作按钮组
						ui.Row(c).Width(90).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
							if item.IsDir {
								if ui.Button(c, "进入").Clicked() {
									s.NavigateTo(item.Path)
								}
							} else {
								if ui.Button(c, "下载").Clicked() {
									s.DownloadEntry(item)
								}
							}
							if ui.Button(c, "删除").Clicked() {
								s.DeleteEntry(item)
							}
						})
					}).OnClick(func() {
						s.selectedPath = item.Path
					})
				}
			})
		})
	})
}

// 渲染传输队列抽屉
func (s *TerminalSession) renderTransferDrawer(c *ui.Context, t *ui.Theme) {
	tasks := s.TransferMgr.GetTasks()

	ui.Column(c).Height(160).Background(ui.Hex("#FAFAFC")).Border(1, t.Border).Padding(t.Space(2), t.Space(3)).Gap(t.Space(1.5)).Children(func() {
		// 抽屉头部
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("后台传输队列 (共 %d 项任务)", len(tasks))).FontSize(12).Bold().TextColor(t.Text)
			ui.Spacer(c)
			if ui.Button(c, "隐藏抽屉").Clicked() {
				s.ShowTransfer = false
			}
		})

		ui.Box(c).Height(1).Background(t.Border)

		// 任务滚动列表
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(t.Space(1.5)).Children(func() {
				if len(tasks) == 0 {
					ui.Text(c, "暂无进行中的文件传输任务").FontSize(11).TextColor(t.TextMuted)
				}

				for _, task := range tasks {
					item := task
					ui.Row(c).Height(26).Background(t.Surface).Border(1, t.Border).Padding(0, t.Space(2.5)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						// 类型微标
						ui.Text(c, string(item.Type)).FontSize(11).Bold().TextColor(t.Accent).Width(40)

						// 文件名与路径
						ui.Text(c, item.FileName).FontSize(12).Bold().TextColor(t.Text).Width(200)

						// 进度百分比与速率
						progressText := fmt.Sprintf("%.1f%% (%s)", item.Percent, item.SpeedHuman)
						if item.Status == transfer.StatusCompleted {
							progressText = "已完成"
						} else if item.Status == transfer.StatusFailed {
							progressText = "失败: " + item.ErrorMsg
						} else if item.Status == transfer.StatusCanceled {
							progressText = "已取消"
						}
						ui.Text(c, progressText).FontSize(11).TextColor(t.TextMuted).Width(180)

						ui.Spacer(c)

						// 取消按钮
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
