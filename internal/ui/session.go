package ui

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/google/uuid"

	"goshell/internal/core/model"
	sshClient "goshell/internal/core/ssh"
)

// TerminalSession 代表一个活跃的终端会话实例
type TerminalSession struct {
	ID        string
	Host      *model.Host
	Title     string
	Term      *terminal.Terminal
	Client    *sshClient.Client
	TermConn  *sshClient.TerminalConn

	Connecting bool
	Connected  bool
	ErrorMsg   string

	StartTime time.Time
	mu        sync.Mutex
}

// NewTerminalSession 初始化一个新的会话实例
func NewTerminalSession(host *model.Host) *TerminalSession {
	return &TerminalSession{
		ID:        uuid.New().String(),
		Host:      host,
		Title:     host.Name,
		StartTime: time.Now(),
	}
}

// Start 启动终端连接（支持本地 Shell 与远程 SSH）
func (s *TerminalSession) Start() {
	s.mu.Lock()
	if s.Connecting || s.Connected {
		s.mu.Unlock()
		return
	}
	s.Connecting = true
	s.ErrorMsg = ""
	s.mu.Unlock()

	// 本地终端启动
	if s.Host.AuthType == model.AuthTypeLocal {
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
		return
	}

	// 远程 SSH 启动（后台异步协程，绝不阻塞 UI 渲染）
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

		s.mu.Lock()
		s.Client = client
		s.TermConn = termConn
		s.Term = term
		s.Connecting = false
		s.Connected = true
		s.mu.Unlock()
	}()
}

// Close 关闭当前会话与所有底层连接
func (s *TerminalSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Connected = false
	s.Connecting = false

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

// Render 绘制终端会话面板
func (s *TerminalSession) Render(c *ui.Context, t *ui.Theme, onClose func()) {
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// 会话顶部辅助状态条 (28px 紧凑桌面高度)
		ui.Row(c).Height(28).Background(ui.Hex("#F8F9FA")).Border(1, t.Border).Padding(0, t.Space(3)).AlignItems(ui.Center).Children(func() {
			// 状态灯
			statusColor := t.Success
			statusText := "已连接"
			if s.Connecting {
				statusColor = t.Warning
				statusText = "正在连接..."
			} else if !s.Connected {
				statusColor = t.Danger
				statusText = "已断开"
			}

			ui.Box(c).Width(6).Height(6).Radius(3).Background(statusColor)
			ui.Text(c, fmt.Sprintf(" %s | %s@%s:%d", statusText, s.Host.Username, s.Host.Address, s.Host.Port)).FontSize(11).TextColor(t.TextMuted)

			ui.Spacer(c)

			// 快速重连按钮
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

		// 视窗主体
		if s.Connecting {
			ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
				ui.Box(c).Width(8).Height(8).Radius(4).Background(t.Warning)
				ui.Text(c, fmt.Sprintf("正在建立 SSH 安全连接至 %s:%s ...", s.Host.Address, strconv.Itoa(s.Host.Port))).FontSize(13).Bold().TextColor(t.Text)
				ui.Text(c, "通过加密通道验证身份与分配 PTY 终端").FontSize(11).TextColor(t.TextMuted)
				if ui.Button(c, "取消连接").Clicked() {
					s.Close()
				}
			})
		} else if !s.Connected && s.ErrorMsg != "" {
			ui.Column(c).Fill().Center().Gap(t.Space(3)).Children(func() {
				ui.Box(c).Width(10).Height(10).Radius(5).Background(t.Danger)
				ui.Text(c, "连接失败或已断开").FontSize(15).Bold().TextColor(t.Danger)
				ui.Text(c, s.ErrorMsg).FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					if ui.PrimaryButton(c, "重试连接").Clicked() {
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
		} else if s.Connected && s.Term != nil {
			// 原生 Ghostty VT 终端 GPU 自绘渲染
			terminal.View(c, s.Term).Fill().AutoFocus()
		}
	})
}
