package ui

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"goshell/internal/app"
	"goshell/internal/core/model"
	sftpService "goshell/internal/core/sftp"
)

// WorkspaceTabType 定义多标签类型
type WorkspaceTabType int

const (
	TabTypeOverview WorkspaceTabType = iota
	TabTypeSession
	TabTypeEditor
)

// WorkspaceTab 代表顶部标签栏中的一个视窗标签
type WorkspaceTab struct {
	Type    WorkspaceTabType
	Title   string
	Session *TerminalSession
	Editor  *EditorFile
}

// AppView 管理桌面主窗口 UI 状态与事件循环，深度复刻 docker-mate 设计规范
type AppView struct {
	app *app.App

	// 主题配置
	theme *ui.Theme

	// 布局尺寸
	sidebarWidth float32

	// 侧边栏当前激活 Tab: 0 = 主机列表, 1 = 远程文件
	sidebarTab int

	// 本机网络 IP
	localIP string

	// 交互状态与多标签工作区
	searchQuery    string
	activeTabIndex int
	tabs           []WorkspaceTab
	selectedHostID string

	// 新建/编辑主机弹窗表单状态
	modalOpen    bool
	formID       string
	formName     string
	formAddress  string
	formPort     string
	formUsername string
	formAuthType string // "password", "key", "local"
	formPassword string
	formKeyPath  string
	formGroupID  string
	formErrorMsg string
}

// NewAppView 初始化主窗口 UI 状态
func NewAppView(application *app.App) *AppView {
	v := &AppView{
		app:          application,
		theme:        DockerMateTheme(),
		sidebarWidth: 280, // 加宽侧边栏以舒适展示双栏信息
		sidebarTab:   0,   // 默认展示主机节点
		localIP:      getLocalIP(),
		formPort:     "22",
		formUsername: "root",
		formAuthType: "password",
		tabs:         []WorkspaceTab{{Type: TabTypeOverview, Title: "概览"}},
	}

	// 默认选中第一个主机以便展示详情
	if len(application.Hosts) > 0 {
		v.selectedHostID = application.Hosts[0].ID
	}

	return v
}

// View 每一帧渲染入口（GPU 自绘）
func (v *AppView) View(c *ui.Context) {
	c.SetTheme(v.theme)
	t := c.Theme()

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 1. 主体工作区（左侧双 Tab 侧边栏 + 右侧全屏工作区，顶满渲染无多余顶栏挤占）
		ui.Split(c, &v.sidebarWidth, func() {
			v.renderSidebar(c, t)
		}, func() {
			v.renderWorkspace(c, t)
		}).Grow(1)

		// 2. 底部状态栏
		v.renderStatusBar(c, t)
	})

	// 3. 对话框弹窗（新建/编辑配置，复刻 docker-mate 的 Dialog）
	if v.modalOpen {
		v.renderHostModal(c, t)
	}
}

// 左侧边栏 (复刻 docker-mate 的 app-sidebar 风格，采用 hsl(212, 31%, 95%) 底色)
func (v *AppView) renderSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(ColSidebarBg).Padding(t.Space(2)).Gap(t.Space(2)).Border(1, ColSidebarBorder).Children(func() {
		// 1. 紧凑边栏头部：品牌标识与操作图标组 (高度 28px)
		ui.Row(c).Height(28).Padding(0, t.Space(1)).AlignItems(ui.Center).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Image(c, goShellLogo).Size(24, 24)
				ui.Text(c, "GoShell").FontSize(13).Bold().TextColor(t.Text)
			})

			ui.Spacer(c)

			// 操作图标按钮组 (添加主机与刷新列表，完全居中，悬浮高亮)
			ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
				// 添加主机小图标按钮
				btnAdd := ui.ButtonBase(c).Tooltip("添加 SSH 主机").Width(24).Height(24).Radius(t.Radius - 2).Center()
				addIconCol := ColSidebarText
				if btnAdd.Hovered() {
					btnAdd.Background(ui.Hex("#E2E8F0"))
					addIconCol = t.Text
				}
				btnAdd.Children(func() {
					ui.Icon(c, IconPlus).Width(13).Height(13).TextColor(addIconCol)
				})
				if btnAdd.Clicked() {
					v.openCreateModal()
				}

				// 刷新小图标按钮
				btnRefresh := ui.ButtonBase(c).Tooltip("刷新列表").Width(24).Height(24).Radius(t.Radius - 2).Center()
				refreshIconCol := ColSidebarText
				if btnRefresh.Hovered() {
					btnRefresh.Background(ui.Hex("#E2E8F0"))
					refreshIconCol = t.Text
				}
				btnRefresh.Children(func() {
					ui.Icon(c, IconRefresh).Width(13).Height(13).TextColor(refreshIconCol)
				})
				if btnRefresh.Clicked() {
					_ = v.app.ReloadData()
				}
			})
		})

		// 2. 双 Tab 切换控制器 (Segmented Tab Bar)
		ui.Row(c).Height(28).Background(ui.Hex("#E2E8F0")).Radius(t.Radius).Padding(2, 2).Gap(2).AlignItems(ui.Center).Children(func() {
			// Tab 0: 主机列表 (平铺)
			hostTabBg := ui.Hex("#E2E8F0")
			hostTabFg := ColSidebarText
			if v.sidebarTab == 0 {
				hostTabBg = t.Surface
				hostTabFg = t.Text
			}
			ui.Row(c).Grow(1).Height(24).Background(hostTabBg).Radius(t.Radius - 2).Center().Gap(t.Space(1)).Children(func() {
				ui.Icon(c, IconServer).TextColor(hostTabFg).Width(12).Height(12)
				ui.Text(c, fmt.Sprintf("主机 (%d)", len(v.app.Hosts))).FontSize(11).Bold().TextColor(hostTabFg)
			}).OnClick(func() {
				v.sidebarTab = 0
			})

			// Tab 1: 远程文件
			fileTabBg := ui.Hex("#E2E8F0")
			fileTabFg := ColSidebarText
			if v.sidebarTab == 1 {
				fileTabBg = t.Surface
				fileTabFg = t.Text
			}
			ui.Row(c).Grow(1).Height(24).Background(fileTabBg).Radius(t.Radius - 2).Center().Gap(t.Space(1)).Children(func() {
				ui.Icon(c, IconFolder).TextColor(fileTabFg).Width(12).Height(12)
				ui.Text(c, "文件").FontSize(11).Bold().TextColor(fileTabFg)
			}).OnClick(func() {
				v.sidebarTab = 1
			})
		})

		ui.Box(c).Height(1).Background(ColSidebarBorder)

		// 3. 视窗内容分流：主机列表平铺或文件浏览
		if v.sidebarTab == 0 {
			v.renderSidebarHostList(c, t)
		} else {
			v.renderSidebarFilePanel(c, t)
		}
	})
}

// getActiveSession 获取当前与工作区或首选会话关联的 TerminalSession
func (v *AppView) getActiveSession() *TerminalSession {
	if v.activeTabIndex > 0 && v.activeTabIndex < len(v.tabs) {
		tab := v.tabs[v.activeTabIndex]
		if tab.Session != nil {
			return tab.Session
		}
	}
	for _, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil {
			return t.Session
		}
	}
	return nil
}

// getSessionCount 获取当前活跃的终端会话总数
func (v *AppView) getSessionCount() int {
	count := 0
	for _, t := range v.tabs {
		if t.Type == TabTypeSession {
			count++
		}
	}
	return count
}

// 侧边栏内容 1: 主机平铺列表 (完全平铺，无分组，顶部集成紧凑搜索)
func (v *AppView) renderSidebarHostList(c *ui.Context, t *ui.Theme) {
	// 边栏搜索框：直接位于列表顶部，消除对主区域空间的占用
	ui.TextInput(c, &v.searchQuery).Placeholder("搜索主机...")

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(1).Children(func() {
			query := strings.ToLower(strings.TrimSpace(v.searchQuery))

			count := 0
			for _, h := range v.app.Hosts {
				if query != "" && !strings.Contains(strings.ToLower(h.Name), query) && !strings.Contains(strings.ToLower(h.Address), query) {
					continue
				}
				v.renderHostRow(c, t, h)
				count++
			}

			if count == 0 {
				ui.Column(c).Padding(t.Space(4)).Center().Children(func() {
					ui.Text(c, "未找到匹配主机").FontSize(11.5).TextColor(t.TextMuted)
				})
			}
		})
	})
}

// 侧边栏内容 2: 远程文件管理器
func (v *AppView) renderSidebarFilePanel(c *ui.Context, t *ui.Theme) {
	activeSession := v.getActiveSession()

	if activeSession == nil {
		ui.Column(c).Grow(1).Center().Gap(t.Space(3)).Padding(t.Space(4)).Children(func() {
			ui.Icon(c, IconFolder).TextColor(t.TextMuted).Width(32).Height(32)
			ui.Text(c, "暂无活跃终端连接").FontSize(13).Bold().TextColor(t.TextMuted)
			ui.Text(c, "请在「主机节点」中连接一台服务器或开启本地终端以浏览远程文件。").FontSize(11).TextColor(t.TextMuted)
			if ui.PrimaryButton(c, "前往主机节点").Clicked() {
				v.sidebarTab = 0
			}
		})
		return
	}

	activeSession.RenderSidebarFileExplorer(c, t, func(entry sftpService.FileEntry) {
		v.openFileInEditor(activeSession, entry)
	})
}

// 辅助渲染单行主机项 (复刻 docker-mate 的 Sidebar.MenuButton 选中/悬浮外观)
func (v *AppView) renderHostRow(c *ui.Context, t *ui.Theme, h *model.Host) {
	host := h
	isSelected := v.selectedHostID == host.ID

	rowBg := ui.Transparent
	borderCol := ui.Transparent
	textCol := ColSidebarText
	nameCol := t.Text

	if isSelected {
		rowBg = ColSidebarActive
		borderCol = ui.Hex("#B8C8D8")
		textCol = ColSidebarActiveText
		nameCol = ColSidebarActiveText
	}

	ui.Row(c).Height(38).Background(rowBg).Border(1, borderCol).Padding(0, t.Space(2)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		// 状态绿点
		ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)

		// 主机信息
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, host.Name).FontSize(12).Bold().TextColor(nameCol)
			if host.AuthType == model.AuthTypeLocal {
				ui.Text(c, "本地 Shell 终端").FontSize(10.5).TextColor(textCol)
			} else {
				ui.Text(c, fmt.Sprintf("%s@%s:%d", host.Username, host.Address, host.Port)).FontSize(10.5).TextColor(textCol)
			}
		})

		// 快捷连接按钮
		if ui.Button(c, "连接").Clicked() {
			v.openHostSession(host)
		}
	}).OnClick(func() {
		v.selectedHostID = host.ID
	})
}

// 右侧工作区 (多标签管理 + 概览仪表盘 / 全屏终端 / 就地代码编辑器)
func (v *AppView) renderWorkspace(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// 顶部多标签栏 (复刻 docker-mate 的 Tabs 标签导航风格)
		tabToClose := -1
		ui.Row(c).Height(36).Shrink(0).Background(ColSidebarBg).Padding(4, t.Space(2)).Gap(4).AlignItems(ui.Center).Border(1, ColSidebarBorder).Children(func() {
			for i := range v.tabs {
				index := i
				tab := v.tabs[index]
				isActive := v.activeTabIndex == index

				tabBg := ui.Transparent
				textColor := t.TextMuted
				borderCol := ui.Transparent

				if isActive {
					tabBg = t.Surface
					textColor = t.Text
					borderCol = t.Border
				}

				title := tab.Title
				if tab.Type == TabTypeEditor && tab.Editor != nil && tab.Editor.Buffer != nil && tab.Editor.Buffer.Modified() {
					title += " ●"
				}

				// 方案 1: 同色系色标法 - 获取所属节点专属色彩
				var hColor HostThemeColor
				hasColor := false
				if tab.Session != nil {
					hColor = tab.Session.HostColor()
					hasColor = true
				}

				tabItem := ui.Column(c).Height(28).Background(tabBg).Border(1, borderCol).Radius(t.Radius).Clip()

				// 顶部 2px 归属色彩标记条 (激活时 100% 饱和，未激活时 40% 微透)
				if hasColor {
					topAlpha := float32(0.4)
					if isActive {
						topAlpha = 1.0
					}
					ui.Box(c).Height(2).FillWidth().Background(hColor.Accent.Alpha(topAlpha))
				} else {
					ui.Box(c).Height(2).FillWidth().Background(ui.Transparent)
				}

				// Tab 内容区域 (图标、标题、关闭按钮)
				ui.Row(c).Grow(1).Padding(0, t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					iconCol := textColor
					if hasColor {
						if isActive {
							iconCol = hColor.Accent
						} else {
							iconCol = hColor.Accent.Alpha(0.7)
						}
					}

					switch tab.Type {
					case TabTypeOverview:
						ui.Icon(c, IconActivity).TextColor(iconCol).Width(12).Height(12)
					case TabTypeSession:
						ui.Icon(c, IconTerminal).TextColor(iconCol).Width(12).Height(12)
					case TabTypeEditor:
						ui.Icon(c, IconFile).TextColor(iconCol).Width(12).Height(12)
					}

					ui.Text(c, title).FontSize(12).TextColor(textColor).Bold()

					// 非主概览标签提供微型矢量 X 关闭图标按钮 (无边框，极简原生桌面质感)
					if index > 0 {
						closeBtn := ui.ButtonBase(c).Width(16).Height(16).Radius(3).Center()
						closeIconCol := t.TextMuted
						if closeBtn.Hovered() {
							closeBtn.Background(ui.Hex("#E2E8F0"))
							closeIconCol = t.Text
						}
						closeBtn.Children(func() {
							ui.Icon(c, IconClose).Width(9).Height(9).TextColor(closeIconCol)
						})
						if closeBtn.Clicked() {
							tabToClose = index
						}
					}
				})

				// 悬停提示
				if tab.Type == TabTypeSession && tab.Session != nil {
					tabItem.Tooltip(fmt.Sprintf("主机会话: %s (%s)", tab.Session.Host.Name, tab.Session.Host.Address))
				} else if tab.Type == TabTypeEditor && tab.Editor != nil && tab.Session != nil {
					tabItem.Tooltip(fmt.Sprintf("所属主机: %s\n文件路径: %s", tab.Session.Host.Name, tab.Editor.Path))
				}

				if tabItem.Clicked() {
					v.activeTabIndex = index
				}
			}
		})

		if tabToClose >= 0 {
			v.closeTab(tabToClose)
		}

		// 标签视窗内容
		if v.activeTabIndex < 0 || v.activeTabIndex >= len(v.tabs) || v.activeTabIndex == 0 {
			v.renderInspectorDashboard(c, t)
		} else {
			currentTab := v.tabs[v.activeTabIndex]
			switch currentTab.Type {
			case TabTypeSession:
				if currentTab.Session != nil {
					currentTab.Session.Render(c, t, func() {
						v.closeTab(v.activeTabIndex)
					}, func() {
						v.sidebarTab = 1
					})
				} else {
					v.renderInspectorDashboard(c, t)
				}
			case TabTypeEditor:
				if currentTab.Editor != nil {
					currentTab.Editor.Render(c, t)
				} else {
					v.renderInspectorDashboard(c, t)
				}
			default:
				v.renderInspectorDashboard(c, t)
			}
		}
	})
}

// 主机属性审查与操作面板 (精简首页：不展示看板数量与功能特性，仅保留添加主机、刷新列表与当前选中主机信息及操作)
func (v *AppView) renderInspectorDashboard(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Grow(1).Padding(t.Space(6)).Children(func() {
		ui.Column(c).Gap(t.Space(4)).Children(func() {
			// 顶部功能条：仅「刷新列表」与「+ 添加 SSH 主机」
			ui.Row(c).Height(38).AlignItems(ui.Center).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Icon(c, IconServer).TextColor(t.Accent).Width(16).Height(16)
					ui.Text(c, "主机控制台").FontSize(15).Bold().TextColor(t.Text)
				})

				ui.Spacer(c)

				ui.Row(c).Gap(t.Space(2)).Children(func() {
					if ui.Button(c, "刷新列表").Clicked() {
						_ = v.app.ReloadData()
					}
					if ui.PrimaryButton(c, "+ 添加 SSH 主机").Clicked() {
						v.openCreateModal()
					}
				})
			})

			ui.Box(c).Height(1).Background(t.Border)

			// 下方展示当前选中主机的信息和操作
			v.renderSelectedHostCard(c, t)
		})
	})
}

// 当前选中主机的详细审查卡片
func (v *AppView) renderSelectedHostCard(c *ui.Context, t *ui.Theme) {
	var host *model.Host
	if v.selectedHostID != "" {
		for _, h := range v.app.Hosts {
			if h.ID == v.selectedHostID {
				host = h
				break
			}
		}
	}

	if host == nil {
		ui.Column(c).Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Padding(t.Space(8)).Center().Gap(t.Space(2)).Children(func() {
			ui.Icon(c, IconServer).TextColor(t.TextMuted).Width(36).Height(36)
			ui.Text(c, "未选择主机").FontSize(14).Bold().TextColor(t.TextMuted)
			ui.Text(c, "请在左侧列表中点击选择一台主机查看配置，或点击「添加 SSH 主机」新建").FontSize(12).TextColor(t.TextMuted)
		})
		return
	}

	ui.Column(c).Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Padding(t.Space(5)).Gap(t.Space(4)).Children(func() {
		// 顶部主机名与操作按钮栏
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(t.Space(1)).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
					ui.Text(c, host.Name).FontSize(18).Bold().TextColor(t.Text)
					// 就绪绿色药丸徽章
					ui.Row(c).Background(ColBadgeGreenBg).Padding(2, 8).Radius(t.Radius).Children(func() {
						ui.Text(c, "就绪 (READY)").FontSize(10.5).Bold().TextColor(ColBadgeGreenText)
					})
				})

				if host.AuthType == model.AuthTypeLocal {
					ui.Text(c, fmt.Sprintf("本地操作系统原生终端会话 (%s)", host.Description)).FontSize(12).TextColor(t.TextMuted)
				} else {
					ui.Text(c, fmt.Sprintf("%s (%s:%d)", host.Description, host.Address, host.Port)).FontSize(12).TextColor(t.TextMuted)
				}
			})

			ui.Spacer(c)

			ui.Row(c).Gap(t.Space(2)).Children(func() {
				if ui.Button(c, "编辑配置").Clicked() {
					v.openEditModal(host)
				}
				if ui.Button(c, "删除主机").Clicked() {
					_ = v.app.Store.DeleteHost(host.ID)
					v.selectedHostID = ""
					_ = v.app.ReloadData()
				}
				if ui.PrimaryButton(c, "发起终端连接").Clicked() {
					v.openHostSession(host)
				}
			})
		})

		ui.Box(c).Height(1).Background(t.Border)

		// 属性配置明细网格 (现代化两栏表单设计)
		ui.Column(c).Background(ui.Hex("#FAFBFC")).Border(1, t.Border).Padding(t.Space(4)).Radius(t.Radius).Gap(t.Space(3)).Children(func() {
			ui.Text(c, "连接配置与安全认证").FontSize(12.5).Bold().TextColor(t.Text)

			// 属性行 1
			ui.Row(c).Gap(t.Space(8)).Children(func() {
				v.renderFieldItem(c, t, "主机别名", host.Name)
				v.renderFieldItem(c, t, "主机地址", host.Address)
				v.renderFieldItem(c, t, "SSH 端口", strconv.Itoa(host.Port))
			})

			// 属性行 2
			ui.Row(c).Gap(t.Space(8)).Children(func() {
				v.renderFieldItem(c, t, "登录用户名", host.Username)
				v.renderFieldItem(c, t, "认证方式", string(host.AuthType))
				if host.AuthType == model.AuthTypeKeyFile {
					v.renderFieldItem(c, t, "私钥路径", host.KeyPath)
				} else if host.AuthType == model.AuthTypeLocal {
					v.renderFieldItem(c, t, "终端类型", "系统原生 Shell (zsh/bash)")
				} else {
					v.renderFieldItem(c, t, "密码状态", "AES-256 已加密保存")
				}
			})

			// 属性行 3
			ui.Row(c).Gap(t.Space(8)).Children(func() {
				v.renderFieldItem(c, t, "创建时间", host.CreatedAt.Format("2006-01-02 15:04:05"))
				v.renderFieldItem(c, t, "更新时间", host.UpdatedAt.Format("2006-01-02 15:04:05"))
			})
		})
	})
}

// 辅助渲染单项属性
func (v *AppView) renderFieldItem(c *ui.Context, t *ui.Theme, label, value string) {
	ui.Column(c).Width(180).Gap(t.Space(0.5)).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(12).Bold().TextColor(t.Text)
	})
}

// getLocalIP 获取本机局域网有效 IPv4 地址
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipv4 := ipnet.IP.To4(); ipv4 != nil {
					return ipv4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

// 底部紧凑状态栏 (复刻 docker-mate 的细长底栏：左侧就绪状态，右侧 IP 与版本)
func (v *AppView) renderStatusBar(c *ui.Context, t *ui.Theme) {
	// 左侧：仅显示就绪相关状态
	sessionCount := v.getSessionCount()
	statusText := "服务就绪"
	if sessionCount > 0 {
		statusText = fmt.Sprintf("服务就绪 · 活跃会话 (%d)", sessionCount)
	}

	// 右侧：当前节点 IP / 本机 IP 与版本号
	ipText := "IP: " + v.localIP

	// 如果当前打开了终端或关联会话，显示当前会话主机的 IP
	activeSession := v.getActiveSession()
	if activeSession != nil && activeSession.Host.AuthType != model.AuthTypeLocal && activeSession.Host.Address != "" {
		ipText = fmt.Sprintf("节点: %s | 本机: %s", activeSession.Host.Address, v.localIP)
	} else if v.selectedHostID != "" {
		// 否则若选中了远程主机，显示选中节点的 IP
		for _, h := range v.app.Hosts {
			if h.ID == v.selectedHostID && h.AuthType != model.AuthTypeLocal && h.Address != "" {
				ipText = fmt.Sprintf("节点: %s | 本机: %s", h.Address, v.localIP)
				break
			}
		}
	}

	ui.Row(c).Height(26).Shrink(0).Background(ColSidebarBg).Padding(0, t.Space(3)).AlignItems(ui.Center).Border(1, ColSidebarBorder).Children(func() {
		// 左边：状态指示点 + 就绪状态
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
			ui.Text(c, statusText).TextColor(t.Success).FontSize(11).Bold()
		})

		ui.Spacer(c)

		// 右边：IP 地址与版本号
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, ipText).FontSize(11).TextColor(t.TextMuted)
			ui.Box(c).Width(1).Height(12).Background(t.Border)
			ui.Text(c, "GoShell v1.0.0").FontSize(11).TextColor(t.TextMuted)
		})
	})
}

// 新建/编辑主机弹窗 (复刻 docker-mate 的 Dialog 规范)
func (v *AppView) renderHostModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.modalOpen, func() {
		ui.Column(c).Width(480).Background(t.Surface).Border(1, t.Border).Padding(t.Space(6)).Radius(8).Gap(t.Space(3.5)).Children(func() {
			title := "添加主机连接"
			if v.formID != "" {
				title = "编辑主机连接"
			}
			ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)
			ui.Text(c, "配置 SSH 远程主机连接凭据，密码与秘钥将由 AES-256 本地加密").FontSize(11.5).TextColor(t.TextMuted)

			if v.formErrorMsg != "" {
				ui.Row(c).Background(ui.Hex("#FEE2E2")).Border(1, ui.Hex("#FCA5A5")).Padding(t.Space(2), t.Space(3)).Radius(t.Radius).Children(func() {
					ui.Text(c, v.formErrorMsg).FontSize(11.5).TextColor(t.Danger)
				})
			}

			// 认证方式选择 (Segmented)
			ui.Row(c).Height(30).Background(ui.Hex("#E2E8F0")).Radius(t.Radius).Padding(2, 2).Gap(2).AlignItems(ui.Center).Children(func() {
				pwdBg := ui.Hex("#E2E8F0")
				pwdFg := ColSidebarText
				if v.formAuthType == "password" {
					pwdBg = t.Surface
					pwdFg = t.Text
				}
				ui.Row(c).Grow(1).Height(26).Background(pwdBg).Radius(t.Radius - 2).Center().Children(func() {
					ui.Text(c, "密码认证").FontSize(11).Bold().TextColor(pwdFg)
				}).OnClick(func() {
					v.formAuthType = "password"
				})

				keyBg := ui.Hex("#E2E8F0")
				keyFg := ColSidebarText
				if v.formAuthType == "key" {
					keyBg = t.Surface
					keyFg = t.Text
				}
				ui.Row(c).Grow(1).Height(26).Background(keyBg).Radius(t.Radius - 2).Center().Children(func() {
					ui.Text(c, "私钥认证").FontSize(11).Bold().TextColor(keyFg)
				}).OnClick(func() {
					v.formAuthType = "key"
				})

				localBg := ui.Hex("#E2E8F0")
				localFg := ColSidebarText
				if v.formAuthType == "local" {
					localBg = t.Surface
					localFg = t.Text
				}
				ui.Row(c).Grow(1).Height(26).Background(localBg).Radius(t.Radius - 2).Center().Children(func() {
					ui.Text(c, "本地 Shell").FontSize(11).Bold().TextColor(localFg)
				}).OnClick(func() {
					v.formAuthType = "local"
					if v.formName == "" {
						v.formName = "本地终端"
					}
				})
			})

			// 别名
			ui.Text(c, "主机别名:").FontSize(11.5).Bold().TextColor(t.Text)
			ui.TextInput(c, &v.formName)

			if v.formAuthType != "local" {
				// 地址与端口
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					ui.Column(c).Grow(1).Children(func() {
						ui.Text(c, "主机 IP / 域名:").FontSize(11.5).Bold().TextColor(t.Text)
						ui.TextInput(c, &v.formAddress)
					})
					ui.Column(c).Width(100).Children(func() {
						ui.Text(c, "端口:").FontSize(11.5).Bold().TextColor(t.Text)
						ui.TextInput(c, &v.formPort)
					})
				})

				// 用户名
				ui.Text(c, "登录用户名:").FontSize(11.5).Bold().TextColor(t.Text)
				ui.TextInput(c, &v.formUsername)

				if v.formAuthType == "password" {
					ui.Text(c, "SSH 登录密码:").FontSize(11.5).Bold().TextColor(t.Text)
					ui.TextInput(c, &v.formPassword)
				} else {
					ui.Text(c, "私钥绝对路径 (如 ~/.ssh/id_rsa):").FontSize(11.5).Bold().TextColor(t.Text)
					ui.TextInput(c, &v.formKeyPath)
				}
			}

			// 底部操作按钮
			ui.Row(c).Gap(t.Space(2)).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, "取消").Clicked() {
					v.modalOpen = false
				}
				if ui.PrimaryButton(c, "保存配置").Clicked() {
					v.saveHostForm()
				}
			})
		})
	})
}

// 打开新建主机弹窗
func (v *AppView) openCreateModal() {
	v.formID = ""
	v.formName = ""
	v.formAddress = ""
	v.formPort = "22"
	v.formUsername = "root"
	v.formAuthType = "password"
	v.formPassword = ""
	v.formKeyPath = ""
	v.formErrorMsg = ""
	if len(v.app.Groups) > 0 {
		v.formGroupID = v.app.Groups[0].ID
	}
	v.modalOpen = true
}

// 打开编辑主机弹窗
func (v *AppView) openEditModal(h *model.Host) {
	v.formID = h.ID
	v.formName = h.Name
	v.formAddress = h.Address
	v.formPort = strconv.Itoa(h.Port)
	v.formUsername = h.Username
	v.formAuthType = string(h.AuthType)
	v.formPassword = h.Password
	v.formKeyPath = h.KeyPath
	v.formGroupID = h.GroupID
	v.formErrorMsg = ""
	v.modalOpen = true
}

// 保存表单
func (v *AppView) saveHostForm() {
	if strings.TrimSpace(v.formName) == "" {
		v.formErrorMsg = "请输入主机显示别名"
		return
	}

	authType := model.AuthType(v.formAuthType)
	address := strings.TrimSpace(v.formAddress)
	port := 22

	if authType == model.AuthTypeLocal {
		address = "localhost"
		port = 0
	} else {
		if address == "" {
			v.formErrorMsg = "请输入主机 IP 或域名"
			return
		}
		var err error
		port, err = strconv.Atoi(strings.TrimSpace(v.formPort))
		if err != nil || port <= 0 {
			v.formErrorMsg = "请输入有效端口号 (1-65535)"
			return
		}
	}

	h := &model.Host{
		ID:          v.formID,
		GroupID:     v.formGroupID,
		Name:        strings.TrimSpace(v.formName),
		Address:     address,
		Port:        port,
		Username:    strings.TrimSpace(v.formUsername),
		AuthType:    authType,
		Password:    v.formPassword,
		KeyPath:     strings.TrimSpace(v.formKeyPath),
		Description: "自建主机连接",
		ColorTag:    "#0366D6",
	}

	if err := v.app.Store.SaveHost(h); err != nil {
		v.formErrorMsg = fmt.Sprintf("保存失败: %v", err)
		return
	}

	_ = v.app.ReloadData()
	v.selectedHostID = h.ID
	v.modalOpen = false
}

// 快速开启本地终端
func (v *AppView) openLocalTerminal() {
	for _, h := range v.app.Hosts {
		if h.AuthType == model.AuthTypeLocal {
			v.openHostSession(h)
			return
		}
	}

	// 若未在列表中，临时构造一个并打开
	localHost := &model.Host{
		ID:          "local",
		Name:        "本地终端",
		Address:     "localhost",
		AuthType:    model.AuthTypeLocal,
		Description: "系统原生 Shell",
	}
	v.openHostSession(localHost)
}

// 打开或切换到主机的终端会话
func (v *AppView) openHostSession(h *model.Host) {
	for i, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil && t.Session.Host.ID == h.ID {
			v.activeTabIndex = i
			v.selectedHostID = h.ID
			return
		}
	}

	session := NewTerminalSession(h)

	// 分配在当前所有打开会话中未被使用的节点专属色彩（方案 1: 同色系色标法）
	usedColors := make(map[int]bool)
	for _, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil {
			usedColors[t.Session.ColorIndex] = true
		}
	}
	colorIdx := 0
	for i := 0; i < len(HostColorPalette); i++ {
		if !usedColors[i] {
			colorIdx = i
			break
		}
	}
	session.ColorIndex = colorIdx

	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeSession,
		Title:   h.Name,
		Session: session,
	})
	v.activeTabIndex = len(v.tabs) - 1
	v.selectedHostID = h.ID
	session.Start()
}

// openFileInEditor 在工作区打开或切换到指定文件的就地编辑器 Tab（方案 3：空间邻近吸附插入）
func (v *AppView) openFileInEditor(s *TerminalSession, entry sftpService.FileEntry) {
	for i, t := range v.tabs {
		if t.Type == TabTypeEditor && t.Editor != nil && t.Editor.Session == s && t.Editor.Path == entry.Path {
			v.activeTabIndex = i
			return
		}
	}

	edFile := NewEditorFile(s, entry.Path, entry.Name, entry.Size)
	edFile.LoadAsync()

	newTab := WorkspaceTab{
		Type:    TabTypeEditor,
		Title:   entry.Name,
		Session: s,
		Editor:  edFile,
	}

	// 方案 3: 查找所属主机的最后一个 Tab 位置进行就近插入，保持同一主机的文件在空间上紧密聚合
	insertIdx := -1
	for i := len(v.tabs) - 1; i >= 0; i-- {
		if v.tabs[i].Session == s {
			insertIdx = i
			break
		}
	}

	if insertIdx >= 0 {
		targetIdx := insertIdx + 1
		newTabs := make([]WorkspaceTab, 0, len(v.tabs)+1)
		newTabs = append(newTabs, v.tabs[:targetIdx]...)
		newTabs = append(newTabs, newTab)
		newTabs = append(newTabs, v.tabs[targetIdx:]...)
		v.tabs = newTabs
		v.activeTabIndex = targetIdx
	} else {
		v.tabs = append(v.tabs, newTab)
		v.activeTabIndex = len(v.tabs) - 1
	}
}

// closeTab 关闭指定索引的 Tab
func (v *AppView) closeTab(tabIndex int) {
	if tabIndex <= 0 || tabIndex >= len(v.tabs) {
		return
	}

	target := v.tabs[tabIndex]
	if target.Type == TabTypeSession && target.Session != nil {
		target.Session.Close()

		// 联动关闭属于该会话的所有已打开编辑文件 Tab
		var newTabs []WorkspaceTab
		for i, t := range v.tabs {
			if i == tabIndex {
				continue
			}
			if t.Type == TabTypeEditor && t.Editor != nil && t.Editor.Session == target.Session {
				continue
			}
			newTabs = append(newTabs, t)
		}
		v.tabs = newTabs
	} else {
		v.tabs = append(v.tabs[:tabIndex], v.tabs[tabIndex+1:]...)
	}

	if v.activeTabIndex >= len(v.tabs) {
		v.activeTabIndex = len(v.tabs) - 1
	}
	if v.activeTabIndex < 0 {
		v.activeTabIndex = 0
	}
}

// CloseAll 关闭所有会话与编辑器 Tab
func (v *AppView) CloseAll() {
	for _, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil {
			t.Session.Close()
		}
	}
	v.tabs = []WorkspaceTab{{Type: TabTypeOverview, Title: "概览"}}
	v.activeTabIndex = 0
}
