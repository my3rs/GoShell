package ui

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
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

// WorkspaceLayoutMode 定义工作区多窗格布局模式
type WorkspaceLayoutMode int

const (
	LayoutSingle    WorkspaceLayoutMode = iota // 单视窗布局 (默认全屏单 Tab)
	LayoutDualSplit                            // 左右双 Tab 并列布局
	LayoutQuadGrid                             // 四宫格 (2x2) 布局
)

// WorkspaceTab 代表顶部标签栏中的一个视窗标签
type WorkspaceTab struct {
	Type        WorkspaceTabType
	Title       string
	CustomTitle bool // 用户手动重命名的自定义标题（自定义名称不被自动序号覆盖）
	Session     *TerminalSession
	Editor      *EditorFile
}

// AppView 管理桌面主窗口 UI 状态与事件循环，深度复刻 docker-mate 设计规范
type AppView struct {
	app *app.App

	// 主题配置
	theme *ui.Theme

	// 布局尺寸
	sidebarWidth     float32
	sidebarCollapsed bool

	// 侧边栏当前激活 Tab: 0 = 主机列表, 1 = 远程文件
	sidebarTab int

	// 本机网络 IP
	localIP string

	// 交互状态与多标签工作区
	searchQuery     string
	activeTabIndex  int
	tabs            []WorkspaceTab
	tabPillReady    bool
	lastTabRect     ui.Rect
	selectedHostID  string
	showHiddenFiles bool
	prevTabTitles   []string
	recentHostIDs   []string

	// 快速布局与多窗格状态 (左右并列、四宫格、单视窗)
	layoutMode      WorkspaceLayoutMode
	activePaneIndex int
	paneTabIndices  [4]int

	// 新建/编辑主机弹窗表单状态
	modalOpen    bool
	formID       string
	formName     string
	formAddress  string
	formPort     string
	formUsername string
	formAuthType string // "password", "key", "local"
	formPassword  string
	formKeyPath   string
	formShellType string // "auto", "busybox", "powershell", "cmd"
	formGroupID   string
	formErrorMsg  string

	// 宿主原生窗口引用与退出拦截状态
	window        *mygo.Window
	quitConfirmed bool

	// 标签重命名弹窗状态
	renameModalOpen bool
	renameTabIndex  int
	renameInput     string
	renameDefault   string

	// 关于弹窗状态
	aboutModalOpen bool

	// 快速连接（临时 SSH 会话）弹窗状态
	quickConnectModalOpen bool
	quickConnectTarget    string // 原始输入或一键粘贴内容 (如 root@1.2.3.4:22 或 ssh root@1.2.3.4)
	quickConnectHost      string // 主机 IP 或域名
	quickConnectPort      string // 端口
	quickConnectUser      string // 用户名
	quickConnectPassword  string // 密码
	quickConnectErrorMsg  string // 错误信息
}

// NewAppView 初始化主窗口 UI 状态
func NewAppView(application *app.App) *AppView {
	v := &AppView{
		app:             application,
		theme:           DockerMateTheme(),
		sidebarWidth:    240, // 紧凑侧边栏宽度
		sidebarTab:      0,   // 默认展示主机节点
		localIP:         getLocalIP(),
		formPort:        "22",
		formUsername:    "root",
		formAuthType:    "password",
		tabs:            []WorkspaceTab{{Type: TabTypeOverview, Title: "概览"}},
		layoutMode:      LayoutSingle,
		activePaneIndex: 0,
		paneTabIndices:  [4]int{0, 1, 2, 3},
	}


	return v
}

// View 每一帧渲染入口（GPU 自绘）
func (v *AppView) View(c *ui.Context) {
	c.SetTheme(v.theme)
	t := c.Theme()

	// 全局快捷键 Cmd+B / Ctrl+B 切换侧边栏展开与折叠
	if c.Shortcut(ui.Cmd, ui.KeyB) || c.Shortcut(ui.Ctrl, ui.KeyB) {
		v.ToggleSidebar()
	}

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 1. 主体工作区（折叠时工作区 100% 铺满，展开时双栏并列可自由拖拽调节宽度）
		if v.sidebarCollapsed {
			v.renderWorkspace(c, t)
		} else {
			ui.Split(c, &v.sidebarWidth, func() {
				v.renderSidebar(c, t)
			}, func() {
				v.renderWorkspace(c, t)
			}).Grow(1)
		}

		// 2. 底部状态栏
		v.renderStatusBar(c, t)
	})

	// 3. 对话框弹窗（新建/编辑配置，复刻 docker-mate 的 Dialog）
	if v.modalOpen {
		v.renderHostModal(c, t)
	}

	// 4. 重命名标签页弹窗
	if v.renameModalOpen {
		v.renderRenameModal(c, t)
	}

	// 5. 关于 GoShell 弹窗 (注明 Ghostty 与 MyGo 开源引用)
	if v.aboutModalOpen {
		v.renderAboutModal(c, t)
	}

	// 6. 快速连接（临时 SSH 会话）弹窗
	if v.quickConnectModalOpen {
		v.renderQuickConnectModal(c, t)
	}
}

// ToggleSidebar 切换左侧边栏的展开与折叠状态 (支持快捷键 Cmd+B / Ctrl+B)
func (v *AppView) ToggleSidebar() {
	v.sidebarCollapsed = !v.sidebarCollapsed
	if !v.sidebarCollapsed && v.sidebarWidth < 180 {
		v.sidebarWidth = 240
	}
}

// 左侧边栏 (复刻 docker-mate 的 app-sidebar 风格，采用 hsl(212, 31%, 95%) 底色)
func (v *AppView) renderSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(ColSidebarBg).Padding(t.Space(2)).Gap(t.Space(2)).Children(func() {
		// 1. 紧凑边栏头部：品牌标识与操作图标组 (高度 28px)
		ui.Row(c).Height(28).Padding(0, t.Space(1)).AlignItems(ui.Center).Children(func() {
			brandRow := ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Padding(2, 4).Radius(t.Radius - 2).Tooltip("关于 GoShell")
			if brandRow.Hovered() {
				brandRow.Background(ui.Hex("#E2E8F0"))
			}
			brandRow.Children(func() {
				ui.Image(c, goShellLogo).Size(24, 24)
				ui.Text(c, "GoShell").FontSize(13).Bold().TextColor(t.Text)
			})
			if brandRow.Clicked() {
				v.aboutModalOpen = true
			}

			ui.Spacer(c)

			// 操作图标按钮组 (添加主机与折叠侧边栏，完全居中，悬浮高亮)
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

				// 展开与折叠侧边栏小图标按钮 (紧跟在添加按钮之后)
				btnCollapse := ui.ButtonBase(c).Tooltip("折叠侧边栏 (Cmd+B)").Width(24).Height(24).Radius(t.Radius - 2).Center()
				collapseIconCol := ColSidebarText
				if btnCollapse.Hovered() {
					btnCollapse.Background(ui.Hex("#E2E8F0"))
					collapseIconCol = t.Text
				}
				btnCollapse.Children(func() {
					ui.Icon(c, IconPanelLeftClose).Width(13).Height(13).TextColor(collapseIconCol)
				})
				if btnCollapse.Clicked() {
					v.ToggleSidebar()
				}
			})
		})

		// 2. 双 Tab 切换控制器 (Segmented Tab Bar，带平滑滑动高亮胶囊动画)
		segTrack := ui.Row(c).Key("sidebar_tab_segmented").Height(28).Background(ui.Hex("#E2E8F0")).Radius(t.Radius).Padding(2, 2).Gap(2).AlignItems(ui.Center)

		// 补间动画插值：0 (主机列表) 到 1 (远程文件)
		animPos := segTrack.AnimateWith("sidebar_tab_pos", float32(v.sidebarTab), 180*time.Millisecond, ui.EaseOut)

		segTrack.Draw(func(p *ui.Painter, r ui.Rect) {
			pad := float32(2)
			gap := float32(2)
			h := r.H - 2*pad
			w := (r.W - 2*pad - gap) / 2
			if w <= 0 {
				return
			}
			pillX := r.X + pad + animPos*(w+gap)
			pillY := r.Y + pad
			pillRect := ui.Rect{X: pillX, Y: pillY, W: w, H: h}
			// shadcn 风格精致微投影与纯白高亮底色
			p.Shadow(pillRect, t.Radius-2, 0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08))
			p.Fill(pillRect, t.Surface, t.Radius-2)
		})

		segTrack.Children(func() {
			// Tab 0: 主机列表 (平铺)
			hostTabFg := ColSidebarText
			if v.sidebarTab == 0 {
				hostTabFg = t.Text
			}
			ui.Row(c).Grow(1).Height(24).Background(ui.Transparent).Radius(t.Radius - 2).Center().Gap(t.Space(1)).Children(func() {
				ui.Icon(c, IconServer).TextColor(hostTabFg).Width(12).Height(12)
				ui.Text(c, fmt.Sprintf("主机 (%d)", len(v.app.Hosts))).FontSize(11).Bold().TextColor(hostTabFg)
			}).OnClick(func() {
				v.sidebarTab = 0
			})

			// Tab 1: 远程文件
			fileTabFg := ColSidebarText
			if v.sidebarTab == 1 {
				fileTabFg = t.Text
			}
			ui.Row(c).Grow(1).Height(24).Background(ui.Transparent).Radius(t.Radius - 2).Center().Gap(t.Space(1)).Children(func() {
				ui.Icon(c, IconFolder).TextColor(fileTabFg).Width(12).Height(12)
				ui.Text(c, "文件").FontSize(11).Bold().TextColor(fileTabFg)
			}).OnClick(func() {
				v.sidebarTab = 1
			})
		})

		// 3. 视窗内容分流：主机列表平铺或文件浏览
		if v.sidebarTab == 0 {
			v.renderSidebarHostList(c, t)
		} else {
			v.renderSidebarFilePanel(c, t)
		}
	})
}

// getActiveSession 获取当前与工作区或首选会话关联的 TerminalSession
// 在双列/四宫格多窗格平铺模式下，严格优先返回处于当前 active 聚焦窗格的主机会话
func (v *AppView) getActiveSession() *TerminalSession {
	if v.layoutMode != LayoutSingle && v.activeTabIndex > 0 {
		paneCount := v.getPaneCount()
		if v.activePaneIndex >= 0 && v.activePaneIndex < paneCount {
			tabIdx := v.paneTabIndices[v.activePaneIndex]
			if tabIdx >= 1 && tabIdx < len(v.tabs) {
				tab := v.tabs[tabIdx]
				if tab.Session != nil {
					return tab.Session
				}
			}
		}
	}

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

	ui.Scroll(c).Fill().Children(func() {
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

// 侧边栏内容 2: 远程文件管理器 (多窗格下自动展示处于 active 格主机的文件)
func (v *AppView) renderSidebarFilePanel(c *ui.Context, t *ui.Theme) {
	activeSession := v.getActiveSession()

	if activeSession == nil {
		ui.Column(c).Fill().Center().Gap(t.Space(3)).Padding(t.Space(4)).Children(func() {
			ui.Icon(c, IconFolder).TextColor(t.TextMuted).Width(32).Height(32)
			ui.Text(c, "暂无活跃终端连接").FontSize(13).Bold().TextColor(t.TextMuted)
			ui.Text(c, "请在主机列表中连接服务器以浏览文件。").FontSize(11).TextColor(t.TextMuted)
			if ui.PrimaryButton(c, "前往主机列表").Clicked() {
				v.sidebarTab = 0
			}
		})
		return
	}


	activeSession.RenderSidebarFileExplorer(c, t, func(entry sftpService.FileEntry) {
		v.openFileInEditor(activeSession, entry)
	})
	v.showHiddenFiles = activeSession.ShowHidden
}

// 辅助渲染单行主机项 (复刻现代侧边栏菜单按钮，仅保留悬停高亮，支持双击连接与右键快捷操作)
func (v *AppView) renderHostRow(c *ui.Context, t *ui.Theme, h *model.Host) {
	host := h
	isLocal := host.AuthType == model.AuthTypeLocal

	row := ui.Row(c).Key(host.ID).Height(38).Background(ui.Transparent).Padding(0, t.Space(2)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2))
	isHovered := row.Hovered()
	if isHovered {
		row.Background(ColSidebarHover)
	}

	// 右键上下文菜单：切换终端、新建终端、编辑、删除（本地终端受保护不可删除）
	row.ContextMenu(func(m *ui.Menu) {
		openCount := v.getHostSessionCount(host.ID)
		if openCount > 0 {
			if m.Item("切换到终端").Chosen() {
				v.openHostSession(host)
			}
			if m.Item("新建连接").Chosen() {
				v.openNewHostSession(host)
			}
		} else {
			if m.Item("连接").Chosen() {
				v.openNewHostSession(host)
			}
		}
		m.Separator()
		if m.Item("编辑").Chosen() {
			v.openEditModal(host)
		}
		m.Separator()
		if m.Item("删除主机").Disabled(isLocal).Chosen() {
			if !isLocal {
				_ = v.app.Store.DeleteHost(host.ID)
				_ = v.app.ReloadData()
			}
		}
	})

	row.Children(func() {
		// 主机信息
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				ui.Text(c, host.Name).FontSize(12).Bold().TextColor(t.Text)
				if isLocal {
					ui.Row(c).Background(ui.Hex("#E0F2FE")).Padding(1, 4).Radius(3).Children(func() {
						ui.Text(c, "置顶").FontSize(9).Bold().TextColor(ui.Hex("#0369A1"))
					})
				}
			})
			if isLocal {
				ui.Text(c, "本地终端").FontSize(10.5).TextColor(ColSidebarText)
			} else {
				ui.Text(c, fmt.Sprintf("%s@%s:%d", host.Username, host.Address, host.Port)).FontSize(10.5).TextColor(ColSidebarText)
			}
		})

		// 快捷连接按钮 (高度紧凑，hover时才显示，统一使用“连接”按钮且无边框)
		if isHovered {
			if ui.Button(c, "连接").Height(22).Padding(0, 8).FontSize(11).Radius(t.Radius - 2).Border(0, ui.Transparent).Clicked() {
				v.openNewHostSession(host)
			}
		}
	})
	if row.DoubleClicked() {
		v.openHostSession(host)
	}
}

// 右侧工作区 (多标签管理 + 快速多视窗布局 + 概览仪表盘 / 全屏终端 / 就地代码编辑器)
func (v *AppView) renderWorkspace(c *ui.Context, t *ui.Theme) {
	// 捕获全局快速布局快捷键 (Alt+1 单视窗, Alt+2 左右双并列, Alt+4 四宫格)
	if c.Shortcut(ui.Alt, ui.Key1) {
		v.SetLayoutMode(LayoutSingle)
	} else if c.Shortcut(ui.Alt, ui.Key2) {
		v.SetLayoutMode(LayoutDualSplit)
	} else if c.Shortcut(ui.Alt, ui.Key4) {
		v.SetLayoutMode(LayoutQuadGrid)
	}

	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// 顶部多标签栏 (下方开放无缝衔接工作区，依靠底色自然区分)
		tabToClose := -1
		tabBar := ui.Row(c).Key("workspace_tab_bar").Height(36).Background(ColSidebarBg).Padding(0, 8, 0, 8).AlignItems(ui.Center).Gap(4)

		// 检测标签页标题或数量是否改变（如重命名、动态终端标题更新等）
		// 在 Immediate Mode UI 下，LayoutBounds() 反映的是上一帧布局；若标题发生变动，须主动请求下一帧刷新
		titlesChanged := len(v.tabs) != len(v.prevTabTitles)
		if !titlesChanged {
			for i, tab := range v.tabs {
				if tab.Title != v.prevTabTitles[i] {
					titlesChanged = true
					break
				}
			}
		}
		if titlesChanged {
			v.prevTabTitles = make([]string, len(v.tabs))
			for i, tab := range v.tabs {
				v.prevTabTitles[i] = tab.Title
			}
			c.Invalidate()
		}

		var targetRect ui.Rect
		foundActive := false
		type tiledTabBg struct {
			rect  ui.Rect
			color ui.Color
		}
		var tiledBgs []tiledTabBg

		tabBar.Children(func() {
			// 侧边栏折叠时，在 Tab 栏最左侧展示展开侧边栏按钮与分隔符 (高度与各 Tab 严格水平居中对齐)
			if v.sidebarCollapsed {
				btnExpand := ui.ButtonBase(c).Tooltip("展开侧边栏 (Cmd+B)").Width(26).Height(26).Radius(t.Radius - 2).Center()
				expandIconCol := t.TextMuted
				if btnExpand.Hovered() {
					btnExpand.Background(ui.Hex("#DFE5EC"))
					expandIconCol = t.Text
				}
				btnExpand.Children(func() {
					ui.Icon(c, IconPanelLeftOpen).Width(13).Height(13).TextColor(expandIconCol)
				})
				if btnExpand.Clicked() {
					v.ToggleSidebar()
				}

				ui.Box(c).Width(1).Height(14).Background(ui.Hex("#CBD5E1")).Margin(0, 2, 0, 2)
			}

			for i := range v.tabs {
				index := i
				tab := v.tabs[index]

				// 多终端平铺感知状态计算：
				// isFocused: 当前拥有键盘输入焦点的 Tab
				// isTiled: 正在屏幕另一个平铺窗格中展示的终端 Tab
				isFocused := v.activeTabIndex == index
				posLabel, paneIdx := v.getTabPanePositionLabel(index)
				isTiled := (paneIdx >= 0 && !isFocused)

				// 提示信息构造
				tip := ""
				if tab.Type == TabTypeOverview {
					tip = "主机概览与控制台"
				} else if tab.Type == TabTypeSession && tab.Session != nil {
					statusStr := "已连接"
					if tab.Session.Connecting {
						statusStr = "连接中..."
					} else if !tab.Session.Connected {
						statusStr = "已断开"
					}
					tip = fmt.Sprintf("终端连接: %s\n地址: %s:%d\n状态: %s", tab.Session.Host.Name, tab.Session.Host.Address, tab.Session.Host.Port, statusStr)
					if posLabel != "" {
						tip += fmt.Sprintf("\n平铺窗格: %s", posLabel)
					}
				} else if tab.Type == TabTypeEditor && tab.Editor != nil && tab.Session != nil {
					modStr := "已保存"
					if tab.Editor.Buffer != nil && tab.Editor.Buffer.Modified() {
						modStr = "未保存"
					}
					tip = fmt.Sprintf("文件编辑器: %s\n所属主机: %s (%s)\n文件路径: %s\n修改状态: %s", tab.Editor.Name, tab.Session.Host.Name, tab.Session.Host.Address, tab.Editor.Path, modStr)
					if posLabel != "" {
						tip += fmt.Sprintf("\n平铺窗格: %s", posLabel)
					}
				}

				// Tab 卡片：保持卡片背景透明，平铺底色统一在底层 tabBar.Draw 中渲染，
				// 避免子元素不透明背景在滑动切换瞬间遮挡滑块圆角
				cardBg := ui.Transparent
				if isFocused && !v.tabPillReady {
					cardBg = t.Surface // 首次无动画位置缓存时静态纯白兜底
				}

				tabKey := fmt.Sprintf("ws_tab_%d_%d", index, tab.Type)
				tabCard := ui.Row(c).Key(tabKey).Height(28).AlignItems(ui.Center).Gap(t.Space(1.5)).
					Background(cardBg).
					Radius(t.Radius - 2).
					Padding(0, 10).
					Shrink(1).
					MaxWidth(220)

				if isFocused {
					bounds := tabCard.LayoutBounds()
					if bounds.W > 0 {
						targetRect = bounds
						foundActive = true
					}
				} else if isTiled {
					bounds := tabCard.LayoutBounds()
					if bounds.W > 0 {
						bgCol := ui.Hex("#F1F5F9") // 平铺态默认淡灰底色
						if tabCard.Hovered() {
							bgCol = ui.Hex("#DFE5EC") // 平铺态鼠标悬停时高亮反馈
						}
						tiledBgs = append(tiledBgs, tiledTabBg{rect: bounds, color: bgCol})
					}
				}

				if !isFocused && !isTiled && tabCard.Hovered() {
					tabCard.Background(ui.Hex("#DFE5EC"))
				}

				if tip != "" {
					tabCard.Tooltip(tip)
				}
				tabCard.ContextMenu(func(m *ui.Menu) {
					v.renderTabContextMenu(m, c, tab, index, &tabToClose)
				})
				if tabToClose != index && tabCard.Clicked() {
					v.switchActivePaneToTab(index)
					bounds := tabCard.LayoutBounds()
					if bounds.W > 0 {
						targetRect = bounds
						foundActive = true
					}
					c.Invalidate()
				}

				tabCard.Children(func() {
					v.renderTabItems(c, t, tab, index, isFocused, isTiled, posLabel, &tabToClose)
				})
			}

			// 新建标签快捷按钮 (+)
			btnNew := ui.Row(c).Width(26).Height(26).Radius(t.Radius - 2).Center()
			if btnNew.Hovered() {
				btnNew.Background(ui.Hex("#DFE5EC"))
			}
			btnNew.Children(func() {
				ui.Icon(c, IconPlus).Width(12).Height(12).TextColor(t.TextMuted)
			})
			btnNew.Tooltip("新建终端或主机连接")
			btnNew.Menu(func(m *ui.Menu) {
				if m.Item("快速连接").Chosen() {
					v.openQuickConnectModal()
				}
				if m.Item("添加 SSH 主机...").Chosen() {
					v.openCreateModal()
				}
				if m.Item("本地终端").Chosen() {
					v.openNewLocalTerminal()
				}
				if len(v.app.Hosts) > 0 {
					m.Separator()
					for _, h := range v.app.Hosts {
						host := h
						openCount := v.getHostSessionCount(host.ID)
						label := "连接 " + host.Name
						if openCount > 0 {
							label = fmt.Sprintf("新建 %s 终端 (%d 已打开)", host.Name, openCount)
						}
						if m.Item(label).Chosen() {
							v.openNewHostSession(host)
						}
					}
				}
			})

			ui.Spacer(c)

			// 快速布局一键切换控制胶囊组 (单视窗 / 左右双 Tab 并列 / 四宫格)
			v.renderLayoutSwitcher(c, t)
		})

		if foundActive {
			v.tabPillReady = true
			v.lastTabRect = targetRect
		}

		if len(tiledBgs) > 0 || (v.tabPillReady && v.lastTabRect.W > 0) {
			targetX := v.lastTabRect.X
			targetW := v.lastTabRect.W
			pillX := tabBar.AnimateWith("ws_tab_pill_x", targetX, 180*time.Millisecond, ui.EaseOut)
			pillW := tabBar.AnimateWith("ws_tab_pill_w", targetW, 180*time.Millisecond, ui.EaseOut)
			pillY := v.lastTabRect.Y
			pillH := v.lastTabRect.H

			tabBar.Draw(func(p *ui.Painter, r ui.Rect) {
				// 1. 先绘制平铺未聚焦 Tab 的淡灰卡片底色 (处于底层，支持悬停高亮反馈)
				for _, tb := range tiledBgs {
					p.Fill(tb.rect, tb.color, t.Radius-2)
				}

				// 2. 绘制当前激活 Tab 的滑动白底滑块 (覆盖在上层，圆角与投影保持完整，绝不被遮挡)
				if v.tabPillReady && v.lastTabRect.W > 0 {
					pillRect := ui.Rect{
						X: pillX,
						Y: pillY,
						W: pillW,
						H: pillH,
					}
					p.Shadow(pillRect, t.Radius-2, 0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08))
					p.Fill(pillRect, t.Surface, t.Radius-2)
				}
			})
		}

		if tabToClose >= 0 {
			v.closeTab(tabToClose)
		}

		// 标签视窗内容
		// 1. 概览页 (Tab 0) 绝对不参与分屏平铺，独占全屏展示概览控制台
		if v.activeTabIndex == 0 || len(v.tabs) <= 1 {
			v.renderInspectorDashboard(c, t)
		} else {
			// 2. 终端平铺视图 (支持单视窗全屏、左右双终端并列、四宫格 2x2 并列)
			switch v.layoutMode {
			case LayoutDualSplit:
				ui.Row(c).Grow(1).Shrink(1).MinHeight(0).AlignItems(ui.Stretch).Background(ColTerminalBg).Children(func() {
					v.renderTilePane(c, t, 0)
					ui.Box(c).Width(1).Background(ui.Hex("#334155"))
					v.renderTilePane(c, t, 1)
				})
			case LayoutQuadGrid:
				ui.Column(c).Grow(1).Shrink(1).MinHeight(0).AlignItems(ui.Stretch).Background(ColTerminalBg).Children(func() {
					ui.Row(c).Grow(1).Shrink(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
						v.renderTilePane(c, t, 0)
						ui.Box(c).Width(1).Background(ui.Hex("#334155"))
						v.renderTilePane(c, t, 1)
					})
					ui.Box(c).Height(1).Background(ui.Hex("#334155"))
					ui.Row(c).Grow(1).Shrink(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
						v.renderTilePane(c, t, 2)
						ui.Box(c).Width(1).Background(ui.Hex("#334155"))
						v.renderTilePane(c, t, 3)
					})
				})
			default: // LayoutSingle
				ui.Column(c).Grow(1).Shrink(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
					v.renderTabContent(c, t, v.activeTabIndex, func() {
						v.closeTab(v.activeTabIndex)
					}, nil)
				})
			}
		}
	})
}

// renderTabContextMenu 渲染标签页右键菜单
func (v *AppView) renderTabContextMenu(m *ui.Menu, c *ui.Context, tab WorkspaceTab, index int, tabToClose *int) {
	// 1. 会话连接控制：断开连接 / 重新连接
	if tab.Type == TabTypeSession && tab.Session != nil {
		if tab.Session.Connected || tab.Session.Connecting {
			if m.Item("断开连接").Chosen() {
				tab.Session.Disconnect()
			}
		} else {
			if m.Item("重新连接").Chosen() {
				tab.Session.Start()
			}
		}
		m.Separator()
	}

	// 2. 复制功能 (复制标签页 & 复制相关信息到剪贴板)
	if tab.Type != TabTypeOverview {
		if m.Item("复制标签页").Chosen() {
			v.duplicateTab(index)
		}

		if tab.Type == TabTypeSession && tab.Session != nil && tab.Session.Host != nil {
			h := tab.Session.Host
			if h.AuthType == model.AuthTypeLocal {
				if m.Item("复制主机地址").Chosen() {
					c.WriteClipboard("localhost")
				}
			} else {
				connStr := fmt.Sprintf("%s@%s:%d", h.Username, h.Address, h.Port)
				if h.Username == "" {
					connStr = fmt.Sprintf("%s:%d", h.Address, h.Port)
				}
				if m.Item("复制主机连接地址").Chosen() {
					c.WriteClipboard(connStr)
				}
				if m.Item("复制主机 IP/域名").Chosen() {
					c.WriteClipboard(h.Address)
				}
			}
			if h.IsEphemeral {
				if m.Item("保存为主机配置...").Chosen() {
					v.openCreateModal()
					v.formName = h.Address
					v.formAddress = h.Address
					v.formPort = strconv.Itoa(h.Port)
					v.formUsername = h.Username
					v.formPassword = h.Password
				}
			}
		} else if tab.Type == TabTypeEditor && tab.Editor != nil {
			if m.Item("复制文件完整路径").Chosen() {
				c.WriteClipboard(tab.Editor.Path)
			}
			if m.Item("复制文件名").Chosen() {
				c.WriteClipboard(tab.Editor.Name)
			}
		}
		m.Separator()
	}

	// 3. 重命名功能 (除概览外的所有标签页均支持重命名)
	if tab.Type != TabTypeOverview {
		if m.Item("重命名标签页...").Chosen() {
			v.openRenameModal(index)
		}
		m.Separator()
	}

	// 4. 关闭标签页操作 (Chrome 标准标签组)
	if index > 0 {
		if m.Item("关闭标签页").Chosen() {
			*tabToClose = index
		}
	}
	if len(v.tabs) > 1 {
		if m.Item("关闭其他标签页").Chosen() {
			v.closeOtherTabs(index)
		}
	}
	if index < len(v.tabs)-1 {
		if m.Item("关闭右侧标签页").Chosen() {
			v.closeTabsToRight(index)
		}
	}
}

// renderTabItems 渲染标签项内部图标、标题与操作按钮
func (v *AppView) renderTabItems(c *ui.Context, t *ui.Theme, tab WorkspaceTab, index int, isFocused, isTiled bool, posLabel string, tabToClose *int) {
	textColor := t.TextMuted
	if isFocused || isTiled {
		textColor = t.Text
	}

	switch tab.Type {
	case TabTypeOverview:
		// 概览仪表盘标签
		homeIconCol := t.TextMuted
		if isFocused {
			homeIconCol = t.Accent
		}
		ui.Icon(c, IconHome).Width(12).Height(12).TextColor(homeIconCol)
		ui.Text(c, "概览").FontSize(12).Bold().TextColor(textColor).SingleLine()

	case TabTypeSession:
		// 终端/主机连接标签
		if tab.Session != nil {
			hColor := tab.Session.HostColor()
			// 主机连接专属色彩标识点 (7x7)
			ui.Box(c).Width(7).Height(7).Radius(3.5).Background(hColor.Accent)
			// 终端连接图标
			termIconCol := textColor
			if isFocused || isTiled {
				termIconCol = hColor.Accent
			}
			ui.Icon(c, IconTerminal).Width(12).Height(12).TextColor(termIconCol)
			// 主机名
			ui.Text(c, tab.Title).FontSize(12).Bold().TextColor(textColor).SingleLine()
			// 连接中或已断开状态徽记
			if tab.Session.Connecting {
				ui.Text(c, "连接中...").FontSize(9.5).TextColor(t.Warning)
			} else if !tab.Session.Connected {
				ui.Text(c, "已断开").FontSize(9.5).TextColor(t.Danger)
			}
		} else {
			ui.Icon(c, IconTerminal).Width(12).Height(12).TextColor(textColor)
			ui.Text(c, tab.Title).FontSize(12).Bold().TextColor(textColor).SingleLine()
		}

	case TabTypeEditor:
		// 远程/本地文件编辑标签
		if tab.Session != nil {
			hColor := tab.Session.HostColor()
			// 所属主机专属色标点 (7x7 与会话标签对齐)
			ui.Box(c).Width(7).Height(7).Radius(3.5).Background(hColor.Accent)
			// 文件图标
			fileIconCol := ui.Hex("#0284C7")
			if !isFocused && !isTiled {
				fileIconCol = t.TextMuted
			}
			ui.Icon(c, IconFile).Width(12).Height(12).TextColor(fileIconCol)
			// 文件名
			ui.Text(c, tab.Title).FontSize(12).Bold().TextColor(textColor).SingleLine()
			// 未保存修改指示黄点
			if tab.Editor != nil && tab.Editor.Buffer != nil && tab.Editor.Buffer.Modified() {
				ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Warning)
			}
		} else {
			ui.Icon(c, IconFile).Width(12).Height(12).TextColor(textColor)
			ui.Text(c, tab.Title).FontSize(12).Bold().TextColor(textColor).SingleLine()
		}
	}

	// 多终端平铺布局下展示所属窗格标识（双分屏展示“左”/“右”，四宫格展示“1”~“4”，概览页绝不显示）
	if posLabel != "" {
		badgeBg := ui.Hex("#E2E8F0")
		badgeFg := ui.Hex("#475569")
		if isFocused {
			badgeBg = ColBadgeBlueBg
			badgeFg = t.Accent
		}
		ui.Row(c).Background(badgeBg).Padding(1, 5).Radius(3).Children(func() {
			ui.Text(c, posLabel).FontSize(9.5).Bold().TextColor(badgeFg)
		})
	}

	// 关闭按钮 (概览标签不可关闭)
	if index > 0 {
		closeBtn := ui.Row(c).Width(16).Height(16).Radius(3).Center()
		closeIconCol := t.TextMuted
		if closeBtn.Hovered() {
			closeBtn.Background(ui.Hex("#E2E8F0"))
			closeIconCol = t.Text
		}
		closeBtn.Children(func() {
			ui.Icon(c, IconClose).Width(12).Height(12).TextColor(closeIconCol)
		})
		if closeBtn.Clicked() {
			if tabToClose != nil {
				*tabToClose = index
			}
		}
	}
}

// SetLayoutMode 一键切换工作区多窗格平铺布局模式
func (v *AppView) SetLayoutMode(mode WorkspaceLayoutMode) {
	v.layoutMode = mode
	// 若用户当前处于概览页且已存在其他终端标签，切换平铺模式时自动切到终端页面，让用户直接看到终端平铺效果；
	// 若仅有概览页，则仅记录布局模式，不自动创建本地终端
	if v.activeTabIndex == 0 && mode != LayoutSingle && len(v.tabs) > 1 {
		v.activeTabIndex = 1
	}
	v.rebalancePanes()
}

// getPaneCount 获取当前布局模式下的窗格总数
func (v *AppView) getPaneCount() int {
	switch v.layoutMode {
	case LayoutDualSplit:
		return 2
	case LayoutQuadGrid:
		return 4
	default:
		return 1
	}
}

// getTabPaneNum 获取指定终端 Tab 当前在哪个窗格中展示 (返回 1~4，若未在任何窗格展示或为概览则返回 0)
func (v *AppView) getTabPaneNum(tabIndex int) int {
	if tabIndex <= 0 || v.layoutMode == LayoutSingle || v.activeTabIndex == 0 {
		return 0
	}
	paneCount := v.getPaneCount()
	for p := 0; p < paneCount; p++ {
		if v.paneTabIndices[p] == tabIndex {
			return p + 1
		}
	}
	return 0
}

// getTabPanePositionLabel 获取指定终端 Tab 在多窗格下的位置显示文本 ("左"/"右" 或 "1".."4") 与对应窗格索引
func (v *AppView) getTabPanePositionLabel(tabIndex int) (string, int) {
	if tabIndex <= 0 || v.layoutMode == LayoutSingle || v.activeTabIndex == 0 {
		return "", -1
	}
	paneCount := v.getPaneCount()
	for p := 0; p < paneCount; p++ {
		if v.paneTabIndices[p] == tabIndex {
			if v.layoutMode == LayoutDualSplit {
				if p == 0 {
					return "左", 0
				}
				return "右", 1
			}
			return strconv.Itoa(p + 1), p
		}
	}
	return "", -1
}

// setActivePane 将指定平铺窗格设为当前激活聚焦窗格，并同步激活其对应的 Tab 索引
func (v *AppView) setActivePane(paneIndex int) {
	paneCount := v.getPaneCount()
	if paneIndex < 0 || paneIndex >= paneCount {
		return
	}
	v.activePaneIndex = paneIndex
	tabIdx := v.paneTabIndices[paneIndex]
	if tabIdx >= 1 && tabIdx < len(v.tabs) {
		v.activeTabIndex = tabIdx
	}
}

// switchActivePaneToTab 处理用户在 Tab 栏点击某个 Tab 时的聚焦与切换逻辑
func (v *AppView) switchActivePaneToTab(tabIndex int) {
	if tabIndex < 0 || tabIndex >= len(v.tabs) {
		tabIndex = 0
	}
	v.activeTabIndex = tabIndex
	if tabIndex >= 0 && tabIndex < len(v.tabs) {
		if s := v.tabs[tabIndex].Session; s != nil && s.Host != nil {
			v.selectedHostID = s.Host.ID
		}
	}

	// 概览页 (index 0) 独立全屏展示，绝不绑定到平铺窗格中
	if tabIndex == 0 {
		return
	}

	if v.layoutMode == LayoutSingle {
		v.paneTabIndices[0] = tabIndex
		v.activePaneIndex = 0
		return
	}

	paneCount := v.getPaneCount()

	// 1. 若该 Tab 当前已经在某个窗格 p 中展示，点击它直接将输入焦点切换到该窗格！
	for p := 0; p < paneCount; p++ {
		if v.paneTabIndices[p] == tabIndex {
			v.activePaneIndex = p
			return
		}
	}

	// 2. 若该 Tab 未在任何窗格中平铺，载入到当前聚焦窗格展示
	currPane := v.activePaneIndex
	if currPane < 0 || currPane >= paneCount {
		currPane = 0
		v.activePaneIndex = 0
	}
	v.paneTabIndices[currPane] = tabIndex
}

// rebalancePanes 仅对终端标签 (index >= 1) 进行窗格分配与去重，概览页 (index 0) 绝不参与
func (v *AppView) rebalancePanes() {
	paneCount := v.getPaneCount()
	if paneCount <= 1 {
		return
	}

	// 收集所有可用的终端/文件 Tab 索引 (严格排除概览 index 0)
	var termIndices []int
	for i := 1; i < len(v.tabs); i++ {
		termIndices = append(termIndices, i)
	}

	if len(termIndices) == 0 {
		for p := 0; p < 4; p++ {
			v.paneTabIndices[p] = -1
		}
		return
	}

	if v.activePaneIndex < 0 || v.activePaneIndex >= paneCount {
		v.activePaneIndex = 0
	}

	used := make(map[int]bool)

	// 确保当前激活窗格显示一个合法的终端 Tab
	curTab := v.paneTabIndices[v.activePaneIndex]
	if curTab < 1 || curTab >= len(v.tabs) {
		if v.activeTabIndex >= 1 && v.activeTabIndex < len(v.tabs) {
			curTab = v.activeTabIndex
		} else {
			curTab = termIndices[0]
		}
	}
	v.paneTabIndices[v.activePaneIndex] = curTab
	v.activeTabIndex = curTab
	used[curTab] = true

	// 依次为其余窗格分配未使用的终端 Tab
	for p := 0; p < paneCount; p++ {
		if p == v.activePaneIndex {
			continue
		}
		pTab := v.paneTabIndices[p]
		if pTab >= 1 && pTab < len(v.tabs) && !used[pTab] {
			used[pTab] = true
			continue
		}

		// 寻找下一个未被使用的终端 Tab
		assigned := -1
		for _, idx := range termIndices {
			if !used[idx] {
				assigned = idx
				break
			}
		}
		v.paneTabIndices[p] = assigned
		if assigned >= 1 {
			used[assigned] = true
		}
	}

	// 收集所有窗格中分配到的有效终端 Tab，并按其在标签栏中的先后顺序 (升序) 重新排列，
	// 确保左/上窗格永远展示前面的标签，右/下窗格展示后面的标签，符合用户直觉
	var assignedTabs []int
	for p := 0; p < paneCount; p++ {
		idx := v.paneTabIndices[p]
		if idx >= 1 && idx < len(v.tabs) {
			assignedTabs = append(assignedTabs, idx)
		}
	}
	sort.Ints(assignedTabs)

	// 将升序排序后的终端 Tab 重新赋予窗格，多余的窗格重置为 -1
	for p := 0; p < paneCount; p++ {
		if p < len(assignedTabs) {
			v.paneTabIndices[p] = assignedTabs[p]
		} else {
			v.paneTabIndices[p] = -1
		}
	}

	// 将当前激活窗格焦点同步定位到当前激活 Tab (v.activeTabIndex) 所在的窗格
	newActivePane := 0
	for p := 0; p < paneCount; p++ {
		if v.paneTabIndices[p] == v.activeTabIndex {
			newActivePane = p
			break
		}
	}
	v.activePaneIndex = newActivePane
}

// sanitizePanes 在 Tab 增减或重组后校验窗格状态 (确保概览 index 0 绝不混入窗格)
func (v *AppView) sanitizePanes() {
	if len(v.tabs) == 0 {
		v.tabs = []WorkspaceTab{{Type: TabTypeOverview, Title: "概览"}}
		v.activeTabIndex = 0
	}
	if v.activeTabIndex < 0 || v.activeTabIndex >= len(v.tabs) {
		v.activeTabIndex = len(v.tabs) - 1
	}
	for p := 0; p < 4; p++ {
		// 窗格索引超出范围，或等于 0 (概览)，均置为 -1
		if v.paneTabIndices[p] >= len(v.tabs) || v.paneTabIndices[p] <= 0 {
			v.paneTabIndices[p] = -1
		}
	}
	v.rebalancePanes()
}

// onTabRemoved 当某个 Tab 被移除后，校正各个窗格对应的 Tab 索引
func (v *AppView) onTabRemoved(removedIndex int) {
	for p := 0; p < 4; p++ {
		if v.paneTabIndices[p] == removedIndex {
			v.paneTabIndices[p] = -1
		} else if v.paneTabIndices[p] > removedIndex {
			v.paneTabIndices[p]--
		}
	}
	v.sanitizePanes()
}

// renderTabContent 统一渲染指定 Tab 的工作视窗内容
func (v *AppView) renderTabContent(c *ui.Context, t *ui.Theme, tabIndex int, onClose func(), onFocus func()) {
	if tabIndex < 0 || tabIndex >= len(v.tabs) || tabIndex == 0 {
		v.renderInspectorDashboard(c, t)
		return
	}
	currentTab := v.tabs[tabIndex]
	switch currentTab.Type {
	case TabTypeSession:
		if currentTab.Session != nil {
			currentTab.Session.Render(c, t, onClose, func() {
				v.sidebarTab = 1
			}, onFocus)
		} else {
			v.renderInspectorDashboard(c, t)
		}
	case TabTypeEditor:
		if currentTab.Editor != nil {
			currentTab.Editor.Render(c, t, onFocus)
		} else {
			v.renderInspectorDashboard(c, t)
		}
	default:
		v.renderInspectorDashboard(c, t)
	}
}

// renderLayoutSwitcher 渲染多标签栏右侧的快速布局控制胶囊组
func (v *AppView) renderLayoutSwitcher(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(28).AlignItems(ui.Center).Gap(1).Padding(2).Background(ui.Hex("#E2E8F0")).Radius(5).Children(func() {
		// 单视窗全屏
		v.renderLayoutBtn(c, t, LayoutSingle, IconLayoutSingle, "单视窗 (Alt+1)")
		// 左右双 Tab 并列
		v.renderLayoutBtn(c, t, LayoutDualSplit, IconLayoutDual, "左右分屏 (Alt+2)")
		// 四宫格 2x2 并列
		v.renderLayoutBtn(c, t, LayoutQuadGrid, IconLayoutQuad, "四宫格分屏 (Alt+4)")
	})
}

func (v *AppView) renderLayoutBtn(c *ui.Context, t *ui.Theme, mode WorkspaceLayoutMode, icon *ui.SVG, tooltip string) {
	isActive := v.layoutMode == mode
	btn := ui.ButtonBase(c).Tooltip(tooltip).Width(24).Height(24).Radius(4).Center()
	iconColor := t.TextMuted
	if isActive {
		btn.Background(t.Surface)
		iconColor = t.Accent
	} else if btn.Hovered() {
		btn.Background(ui.Hex("#F1F5F9"))
		iconColor = t.Text
	}
	btn.Children(func() {
		ui.Icon(c, icon).Width(13).Height(13).TextColor(iconColor)
	})
	if btn.Clicked() {
		v.SetLayoutMode(mode)
	}
}

// renderTilePane 渲染平铺布局中的单个终端窗格 (无多余冗余顶栏，终端顶天立地渲染)
func (v *AppView) renderTilePane(c *ui.Context, t *ui.Theme, paneIndex int) {
	isActivePane := v.activePaneIndex == paneIndex
	tabIndex := v.paneTabIndices[paneIndex]

	paneBox := ui.Column(c).Grow(1).Shrink(1).MinWidth(0).MinHeight(0).AlignItems(ui.Stretch).Clip().Background(ColTerminalBg)
	if isActivePane {
		paneBox.Border(1, t.Accent)
	} else {
		paneBox.Border(1, ui.Transparent)
	}

	onPaneFocus := func() {
		v.setActivePane(paneIndex)
	}

	paneBox.Children(func() {
		if tabIndex >= 1 && tabIndex < len(v.tabs) {
			v.renderTabContent(c, t, tabIndex, func() {
				v.closeTab(tabIndex)
			}, onPaneFocus)
		} else {
			v.renderEmptyPane(c, t, paneIndex)
		}
	})

	if paneBox.Clicked() && !isActivePane {
		v.setActivePane(paneIndex)
	}
}

// renderEmptyPane 渲染未绑定终端的平铺空窗格卡片 (深色终端质感)
func (v *AppView) renderEmptyPane(c *ui.Context, t *ui.Theme, paneIndex int) {
	emptyBox := ui.Column(c).Grow(1).Shrink(1).MinHeight(0).Center().Gap(t.Space(3)).Padding(t.Space(4)).Background(ColTerminalBg)
	if emptyBox.Clicked() {
		v.setActivePane(paneIndex)
	}
	emptyBox.Children(func() {
		ui.Icon(c, IconTerminal).Width(32).Height(32).TextColor(ui.Hex("#475569"))
		ui.Text(c, fmt.Sprintf("分屏窗格 %d", paneIndex+1)).FontSize(13).Bold().TextColor(ui.Hex("#94A3B8"))
		ui.Row(c).Gap(t.Space(2)).Children(func() {
			if ui.Button(c, "+ 本地终端").Clicked() {
				v.activePaneIndex = paneIndex
				v.openNewLocalTerminal()
			}
			if len(v.app.Hosts) > 0 {
				btnConn := ui.PrimaryButton(c, "连接主机...")
				btnConn.Menu(func(m *ui.Menu) {
					for _, h := range v.app.Hosts {
						host := h
						if m.Item(host.Name).Chosen() {
							v.activePaneIndex = paneIndex
							v.openNewHostSession(host)
						}
					}
				})
			} else {
				if ui.PrimaryButton(c, "+ 添加 SSH 主机").Clicked() {
					v.openCreateModal()
				}
			}
		})
	})
}

// recordRecentHost 记录最近连接的主机 ID (去重，保留最新 6 条)
func (v *AppView) recordRecentHost(hostID string) {
	if hostID == "" {
		return
	}
	newList := []string{hostID}
	for _, id := range v.recentHostIDs {
		if id != hostID {
			newList = append(newList, id)
		}
	}
	if len(newList) > 6 {
		newList = newList[:6]
	}
	v.recentHostIDs = newList
}

// getRecentHosts 获取用于在启动台展示的最近主机列表 (若最近记录不足则自动补充已有主机，确保初始状态不空荡)
func (v *AppView) getRecentHosts() []*model.Host {
	var list []*model.Host
	seen := make(map[string]bool)

	for _, id := range v.recentHostIDs {
		for _, h := range v.app.Hosts {
			if h.ID == id && !seen[id] {
				list = append(list, h)
				seen[id] = true
				break
			}
		}
	}

	for _, h := range v.app.Hosts {
		if len(list) >= 4 {
			break
		}
		if !seen[h.ID] {
			list = append(list, h)
			seen[h.ID] = true
		}
	}
	return list
}

// 主机属性审查与操作面板 (方向 B：极简启动台 Launchpad，集成快捷启动、最近连接与效率秘籍)
func (v *AppView) renderInspectorDashboard(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Fill().Padding(t.Space(6), t.Space(8)).Children(func() {
		ui.Column(c).Gap(t.Space(5)).Children(func() {
			// 1. 顶部欢迎与标语
			ui.Column(c).Gap(t.Space(1)).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Icon(c, IconBrand).TextColor(t.Accent).Width(20).Height(20)
					ui.Text(c, "GoShell 启动台").FontSize(17).Bold().TextColor(t.Text)
				})
				ui.Text(c, "轻量、高效的现代终端与远程运维工作区，从下方快捷开始或直接连接").FontSize(12).TextColor(t.TextMuted)
			})

			// 2. 快捷启动操作卡片组 (3 列并排卡片)
			ui.Row(c).Gap(t.Space(3)).Children(func() {
				v.renderQuickActionCard(c, t, IconServer, "快速连接", "免保存配置，输入 IP 临时直连", func() {
					v.openQuickConnectModal()
				})
				v.renderQuickActionCard(c, t, IconPlus, "添加 SSH 主机", "保存主机别名、端口与安全密钥", func() {
					v.openCreateModal()
				})
				v.renderQuickActionCard(c, t, IconTerminal, "本地终端", "启动系统原生命令行 Shell 环境", func() {
					v.openLocalTerminal()
				})
			})

			// 分割横线
			ui.Box(c).Height(1).Background(t.Border)

			// 3. 最近连接 (Recent Connections)
			recentHosts := v.getRecentHosts()
			ui.Column(c).Gap(t.Space(2.5)).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
						ui.Icon(c, IconActivity).TextColor(t.Accent).Width(15).Height(15)
						ui.Text(c, "最近连接").FontSize(14).Bold().TextColor(t.Text)
					})
					ui.Spacer(c)
					ui.Text(c, "双击直达终端").FontSize(11).TextColor(t.TextMuted)
				})

				if len(recentHosts) == 0 {
					ui.Column(c).Padding(t.Space(6)).Center().Gap(t.Space(1.5)).Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Children(func() {
						ui.Icon(c, IconServer).TextColor(t.TextMuted).Width(28).Height(28)
						ui.Text(c, "暂无连接记录").FontSize(13).Bold().TextColor(t.TextMuted)
						ui.Text(c, "点击上方「添加 SSH 主机」或「本地终端」开启您的第一个会话").FontSize(11).TextColor(t.TextMuted)
					})
				} else {
					ui.Column(c).Gap(t.Space(2)).Children(func() {
						for _, h := range recentHosts {
							v.renderRecentHostRow(c, t, h)
						}
					})
				}
			})
		})
	})
}

// 辅助渲染单个快捷启动卡片 (在纯白底色下采用 t.SurfaceHover 达到与侧边栏感知一致的细腻悬浮质感)
func (v *AppView) renderQuickActionCard(c *ui.Context, t *ui.Theme, icon *ui.SVG, title, desc string, onClick func()) {
	card := ui.Column(c).Grow(1).Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Padding(t.Space(3.5), t.Space(4)).Gap(t.Space(1.5))
	if card.Hovered() {
		card.Background(t.SurfaceHover)
	}
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Icon(c, icon).TextColor(t.Accent).Width(16).Height(16)
			ui.Text(c, title).FontSize(13).Bold().TextColor(t.Text)
		})
		ui.Text(c, desc).FontSize(11).TextColor(t.TextMuted)
	})
	if card.Clicked() && onClick != nil {
		onClick()
	}
}

// 辅助渲染最近连接的主机行 (在纯白工作区底色下采用 t.SurfaceHover，感知效果与左侧边栏一致)
func (v *AppView) renderRecentHostRow(c *ui.Context, t *ui.Theme, host *model.Host) {
	isLocal := host.AuthType == model.AuthTypeLocal

	row := ui.Row(c).Key("recent_"+host.ID).Height(38).Background(ui.Transparent).Radius(t.Radius).Padding(0, t.Space(2.5)).AlignItems(ui.Center).Gap(t.Space(2.5))
	isHovered := row.Hovered()
	if isHovered {
		row.Background(t.SurfaceHover)
	}

	row.Children(func() {
		// 主机名称与标识药丸
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, host.Name).FontSize(12).Bold().TextColor(t.Text)
			if isLocal {
				ui.Row(c).Background(ui.Hex("#E0F2FE")).Padding(1, 4).Radius(3).Children(func() {
					ui.Text(c, "置顶").FontSize(9).Bold().TextColor(ui.Hex("#0369A1"))
				})
			}
		})

		// 连接网络参数
		if isLocal {
			ui.Text(c, "本地终端").FontSize(11).TextColor(ColSidebarText)
		} else {
			ui.Text(c, fmt.Sprintf("%s@%s:%d", host.Username, host.Address, host.Port)).FontSize(11).TextColor(ColSidebarText)
		}

		ui.Spacer(c)

		// 快捷操作按钮 (hover时才显示，统一使用无边框紧凑按钮，风格与左侧列表保持完全一致)
		if isHovered {
			ui.Row(c).Gap(t.Space(1)).Children(func() {
				if ui.Button(c, "连接").Height(22).Padding(0, 8).FontSize(11).Radius(t.Radius - 2).Border(0, ui.Transparent).Clicked() {
					v.openNewHostSession(host)
				}
				if ui.Button(c, "编辑").Height(22).Padding(0, 8).FontSize(11).Radius(t.Radius - 2).Border(0, ui.Transparent).Clicked() {
					v.openEditModal(host)
				}
			})
		}
	})

	if row.DoubleClicked() {
		v.openHostSession(host)
	}
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
	}

	ui.Row(c).Height(26).Background(ColSidebarBg).Padding(0, t.Space(3)).AlignItems(ui.Center).Border(1, ColSidebarBorder).Children(func() {
		// 左边：状态指示点 + 就绪状态
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
			ui.Text(c, statusText).TextColor(t.Success).FontSize(11).Bold()
		})

		ui.Spacer(c)

		// 右边：IP 地址与版本号 (点击版本可查看关于与开源授权声明)
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, ipText).FontSize(11).TextColor(t.TextMuted)
			ui.Box(c).Width(1).Height(12).Background(t.Border)
			btnVer := ui.Row(c).AlignItems(ui.Center).Padding(1, 4).Radius(t.Radius - 3).Tooltip("关于 GoShell")
			if btnVer.Hovered() {
				btnVer.Background(ui.Hex("#DFE5EC"))
			}
			btnVer.Children(func() {
				ui.Text(c, "GoShell v0.3.0").FontSize(11).TextColor(t.TextMuted)
			})
			if btnVer.Clicked() {
				v.aboutModalOpen = true
			}
		})
	})
}

// 新建/编辑主机弹窗 (复刻 docker-mate 的 Dialog 规范)
func (v *AppView) renderHostModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.modalOpen, func() {
		ui.Column(c).Width(480).Gap(t.Space(3.5)).Children(func() {
			title := "添加主机"
			titleIcon := IconPlus
			if v.formID != "" {
				title = "编辑主机配置"
				titleIcon = IconServer
			}
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Icon(c, titleIcon).Width(18).Height(18).TextColor(t.Accent)
				ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)
			})
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
					ui.Text(c, "本地终端").FontSize(11).Bold().TextColor(localFg)
				}).OnClick(func() {
					v.formAuthType = "local"
					if v.formName == "" {
						v.formName = "本地终端"
					}
				})
			})

			// 别名
			ui.Text(c, "主机别名").FontSize(11.5).Bold().TextColor(t.Text)
			ui.TextInput(c, &v.formName)

			if v.formAuthType != "local" {
				// 地址与端口
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					ui.Column(c).Grow(1).Children(func() {
						ui.Text(c, "主机地址").FontSize(11.5).Bold().TextColor(t.Text)
						ui.TextInput(c, &v.formAddress)
					})
					ui.Column(c).Width(100).Children(func() {
						ui.Text(c, "端口").FontSize(11.5).Bold().TextColor(t.Text)
						ui.TextInput(c, &v.formPort)
					})
				})

				// 用户名
				ui.Text(c, "用户名").FontSize(11.5).Bold().TextColor(t.Text)
				ui.TextInput(c, &v.formUsername)

				if v.formAuthType == "password" {
					ui.Text(c, "密码").FontSize(11.5).Bold().TextColor(t.Text)
					ui.TextInput(c, &v.formPassword)
				} else {
					ui.Text(c, "私钥路径").FontSize(11.5).Bold().TextColor(t.Text)
					ui.TextInput(c, &v.formKeyPath).Placeholder("~/.ssh/id_rsa")
				}
			} else {
				// 本地 Shell 类型分段单选
				ui.Text(c, "Shell 类型").FontSize(11.5).Bold().TextColor(t.Text)
				ui.Row(c).Height(30).Background(ui.Hex("#E2E8F0")).Radius(t.Radius).Padding(2, 2).Gap(2).AlignItems(ui.Center).Children(func() {
					options := []struct {
						id   string
						name string
					}{
						{"auto", "自动探测"},
						{"busybox", "BusyBox"},
						{"powershell", "PowerShell"},
						{"cmd", "CMD"},
					}
					for _, opt := range options {
						bg := ui.Hex("#E2E8F0")
						fg := ColSidebarText
						if v.formShellType == opt.id || (v.formShellType == "" && opt.id == "auto") {
							bg = t.Surface
							fg = t.Text
						}
						targetID := opt.id
						ui.Row(c).Grow(1).Height(26).Background(bg).Radius(t.Radius - 2).Center().Children(func() {
							ui.Text(c, opt.name).FontSize(11).Bold().TextColor(fg)
						}).OnClick(func() {
							v.formShellType = targetID
						})
					}
				})
				ui.Text(c, "提示: 自动模式优先使用内置 BusyBox 虚拟 Bash，若未检测到则降级原生控制台").FontSize(11).TextColor(t.TextMuted)
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

// openRenameModal 打开标签页重命名弹窗
func (v *AppView) openRenameModal(tabIndex int) {
	if tabIndex <= 0 || tabIndex >= len(v.tabs) {
		return
	}
	tab := v.tabs[tabIndex]
	defaultTitle := tab.Title
	if tab.Type == TabTypeSession && tab.Session != nil && tab.Session.Host != nil {
		defaultTitle = tab.Session.Host.Name
	} else if tab.Type == TabTypeEditor && tab.Editor != nil {
		defaultTitle = tab.Editor.Name
	}

	v.renameTabIndex = tabIndex
	v.renameInput = tab.Title
	v.renameDefault = defaultTitle
	v.renameModalOpen = true
}

// saveRenameTab 保存标签页新名称
func (v *AppView) saveRenameTab() {
	if v.renameTabIndex <= 0 || v.renameTabIndex >= len(v.tabs) {
		v.renameModalOpen = false
		return
	}
	newTitle := strings.TrimSpace(v.renameInput)
	if newTitle == "" || newTitle == v.renameDefault {
		// 恢复默认名称
		v.tabs[v.renameTabIndex].CustomTitle = false
		if v.tabs[v.renameTabIndex].Session != nil && v.tabs[v.renameTabIndex].Session.Host != nil {
			v.updateHostTabTitles(v.tabs[v.renameTabIndex].Session.Host.ID)
		} else {
			v.tabs[v.renameTabIndex].Title = v.renameDefault
		}
	} else {
		v.tabs[v.renameTabIndex].Title = newTitle
		v.tabs[v.renameTabIndex].CustomTitle = true
		if v.tabs[v.renameTabIndex].Session != nil {
			v.tabs[v.renameTabIndex].Session.Title = newTitle
		}
	}
	v.renameModalOpen = false
	if v.window != nil {
		v.window.Invalidate()
	}
}

// renderRenameModal 标签页重命名弹窗
func (v *AppView) renderRenameModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.renameModalOpen, func() {
		ui.Column(c).Width(420).Gap(t.Space(3.5)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Icon(c, IconBrand).Width(18).Height(18).TextColor(t.Accent)
				ui.Text(c, "重命名标签页").FontSize(15).Bold().TextColor(t.Text)
			})

			ui.Text(c, "输入自定义标签名称；留空或点击「恢复默认」可还原原始名称").FontSize(11.5).TextColor(t.TextMuted)

			ui.TextInput(c, &v.renameInput).Placeholder("输入标签名称...").OnSubmit(func() {
				v.saveRenameTab()
			})

			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				if ui.Button(c, "恢复默认").Clicked() {
					v.renameInput = v.renameDefault
					v.saveRenameTab()
				}

				ui.Spacer(c)

				if ui.Button(c, "取消").Clicked() {
					v.renameModalOpen = false
				}

				if ui.PrimaryButton(c, "确定").Clicked() {
					v.saveRenameTab()
				}
			})
		})
	})
}

// renderAboutModal 渲染关于 GoShell 弹窗 (精简、现代极简设计)
func (v *AppView) renderAboutModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.aboutModalOpen, func() {
		ui.Column(c).Width(360).Gap(t.Space(4)).Center().Children(func() {
			// 1. 软件标识区 (Logo + 名称 + 版本)
			ui.Column(c).Center().Gap(t.Space(1)).Children(func() {
				ui.Image(c, goShellLogo).Size(48, 48)
				ui.Text(c, "GoShell").FontSize(18).Bold().TextColor(t.Text)
				ui.Text(c, "版本 0.3.0").FontSize(12).TextColor(t.TextMuted)
			})

			// 2. 核心技术致谢 (轻量精简两行列表)
			ui.Column(c).Fill().Background(ui.Hex("#F8FAFC")).Border(1, ui.Hex("#E2E8F0")).Padding(t.Space(2.5), t.Space(3)).Radius(t.Radius).Gap(t.Space(2)).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "终端引擎").FontSize(12).TextColor(t.TextMuted)
					ui.Spacer(c)
					ui.Text(c, "Ghostty").FontSize(12).Bold().TextColor(t.Text)
					ui.Text(c, " (MIT)").FontSize(11).TextColor(t.TextMuted)
				})

				ui.Box(c).Height(1).Background(ui.Hex("#E2E8F0"))

				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "UI 框架").FontSize(12).TextColor(t.TextMuted)
					ui.Spacer(c)
					ui.Text(c, "MyGo").FontSize(12).Bold().TextColor(t.Text)
					ui.Text(c, " (MIT)").FontSize(11).TextColor(t.TextMuted)
				})
			})

			// 3. 简短版权说明
			ui.Text(c, "© 2026 GoShell · All rights reserved").FontSize(11).TextColor(t.TextMuted)

			// 4. 关闭按钮
			ui.Row(c).Center().Children(func() {
				if ui.PrimaryButton(c, "确定").Width(96).Clicked() {
					v.aboutModalOpen = false
				}
			})
		})
	})
}

// 打开快速连接弹窗
func (v *AppView) openQuickConnectModal() {
	v.quickConnectTarget = ""
	v.quickConnectHost = ""
	v.quickConnectPort = "22"
	v.quickConnectUser = "root"
	v.quickConnectPassword = ""
	v.quickConnectErrorMsg = ""
	v.quickConnectModalOpen = true
}

// 基于已有解析好的主机打开快速连接弹窗
func (v *AppView) openQuickConnectWithHost(h *model.Host) {
	v.quickConnectTarget = h.Address
	v.quickConnectHost = h.Address
	v.quickConnectPort = strconv.Itoa(h.Port)
	if h.Port <= 0 {
		v.quickConnectPort = "22"
	}
	v.quickConnectUser = h.Username
	if v.quickConnectUser == "" {
		v.quickConnectUser = "root"
	}
	v.quickConnectPassword = h.Password
	v.quickConnectErrorMsg = ""
	v.quickConnectModalOpen = true
}

// 根据用户在目标框输入的快捷命令 (如 root@192.168.1.1:22 或 ssh -p 2222 root@1.2.3.4) 自动同步拆解各字段
func (v *AppView) syncQuickConnectFieldsFromTarget() {
	raw := strings.TrimSpace(v.quickConnectTarget)
	if raw == "" {
		return
	}
	if h, ok := ParseSSHCommand(raw); ok {
		if h.Address != "" {
			v.quickConnectHost = h.Address
		}
		if h.Username != "" {
			v.quickConnectUser = h.Username
		}
		if h.Port > 0 {
			v.quickConnectPort = strconv.Itoa(h.Port)
		}
		if h.Password != "" {
			v.quickConnectPassword = h.Password
		}
		return
	}

	// 尝试作为直接的目标字符串解析：[user[:pwd]@]host[:port]
	target := raw
	if atIdx := strings.Index(target, "@"); atIdx != -1 {
		userInfo := target[:atIdx]
		target = target[atIdx+1:]
		if colonIdx := strings.Index(userInfo, ":"); colonIdx != -1 {
			v.quickConnectUser = userInfo[:colonIdx]
			v.quickConnectPassword = userInfo[colonIdx+1:]
		} else {
			v.quickConnectUser = userInfo
		}
	}
	if colonIdx := strings.LastIndex(target, ":"); colonIdx != -1 {
		hostPart := target[:colonIdx]
		portPart := target[colonIdx+1:]
		if p, err := strconv.Atoi(portPart); err == nil && p > 0 && p <= 65535 {
			target = hostPart
			v.quickConnectPort = strconv.Itoa(p)
		}
	}
	target = strings.Trim(target, "[]'\"")
	if target != "" {
		v.quickConnectHost = target
	}
}

// 提交快速连接建立临时 SSH 标签页
func (v *AppView) submitQuickConnect() {
	v.syncQuickConnectFieldsFromTarget()

	hostAddr := strings.TrimSpace(v.quickConnectHost)
	if hostAddr == "" {
		hostAddr = strings.TrimSpace(v.quickConnectTarget)
	}
	if hostAddr == "" {
		v.quickConnectErrorMsg = "请输入主机 IP 或域名"
		return
	}

	port := 22
	if p, err := strconv.Atoi(strings.TrimSpace(v.quickConnectPort)); err == nil && p > 0 && p <= 65535 {
		port = p
	}

	user := strings.TrimSpace(v.quickConnectUser)
	if user == "" {
		user = "root"
	}

	ephemeralHost := &model.Host{
		ID:          "ephemeral-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Name:        "临时: " + hostAddr,
		Address:     hostAddr,
		Port:        port,
		Username:    user,
		Password:    v.quickConnectPassword,
		AuthType:    model.AuthTypePassword,
		IsEphemeral: true,
	}

	v.quickConnectModalOpen = false
	v.openNewHostSession(ephemeralHost)
}

// renderQuickConnectModal 渲染快速连接弹窗 (简明无冗余文案)
func (v *AppView) renderQuickConnectModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.quickConnectModalOpen, func() {
		ui.Column(c).Width(400).Gap(t.Space(3)).Children(func() {
			// 1. 标题区
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Icon(c, IconServer).Width(18).Height(18).TextColor(t.Accent)
				ui.Text(c, "快速连接").FontSize(15).Bold().TextColor(t.Text)
			})

			ui.Text(c, "临时建立 SSH 连接，断开后自动销毁，不保存至主机列表。").FontSize(12).TextColor(t.TextMuted)

			// 2. 表单输入区
			ui.Column(c).Gap(t.Space(2.5)).Children(func() {
				// 快速目标输入框
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "连接目标").FontSize(12).TextColor(t.Text)
					inp := ui.TextInput(c, &v.quickConnectTarget).Placeholder("如 root@192.168.1.100 或直接粘贴 ssh 命令")
					if inp.Changed() {
						v.syncQuickConnectFieldsFromTarget()
					}
				})

				// 主机地址与端口分栏
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "主机地址").FontSize(12).TextColor(t.Text)
						ui.TextInput(c, &v.quickConnectHost).Placeholder("IP 或域名")
					})
					ui.Column(c).Width(80).Gap(4).Children(func() {
						ui.Text(c, "端口").FontSize(12).TextColor(t.Text)
						ui.TextInput(c, &v.quickConnectPort).Placeholder("22")
					})
				})

				// 用户名与密码分栏
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "用户名").FontSize(12).TextColor(t.Text)
						ui.TextInput(c, &v.quickConnectUser).Placeholder("root")
					})
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, "密码").FontSize(12).TextColor(t.Text)
						ui.TextInput(c, &v.quickConnectPassword).Password().Placeholder("可选")
					})
				})

				if v.quickConnectErrorMsg != "" {
					ui.Text(c, v.quickConnectErrorMsg).FontSize(12).TextColor(ui.Hex("#EF4444"))
				}
			})

			// 3. 底部操作按钮
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Spacer(c)

				if ui.Button(c, "取消").Clicked() {
					v.quickConnectModalOpen = false
				}

				if ui.PrimaryButton(c, "连接").Clicked() {
					v.submitQuickConnect()
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
	v.formShellType = "auto"
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
	v.formShellType = h.ShellType
	if v.formShellType == "" {
		v.formShellType = "auto"
	}
	v.formGroupID = h.GroupID
	v.formErrorMsg = ""
	v.modalOpen = true
}

// 保存表单
func (v *AppView) saveHostForm() {
	if strings.TrimSpace(v.formName) == "" {
		v.formErrorMsg = "请输入主机别名"
		return
	}

	authType := model.AuthType(v.formAuthType)
	address := strings.TrimSpace(v.formAddress)
	port := 22

	shellType := ""
	if authType == model.AuthTypeLocal {
		address = "localhost"
		port = 0
		if v.formShellType != "auto" {
			shellType = strings.TrimSpace(v.formShellType)
		}
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
		ShellType:   shellType,
		ColorTag:    "#0366D6",
	}

	if err := v.app.Store.SaveHost(h); err != nil {
		v.formErrorMsg = fmt.Sprintf("保存失败: %v", err)
		return
	}

	_ = v.app.ReloadData()
	v.modalOpen = false
}

// 快速开启或切换到本地终端
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
	}
	v.openHostSession(localHost)
}

// 快速新建一个本地终端标签
func (v *AppView) openNewLocalTerminal() {
	for _, h := range v.app.Hosts {
		if h.AuthType == model.AuthTypeLocal {
			v.openNewHostSession(h)
			return
		}
	}
	localHost := &model.Host{
		ID:          "local",
		Name:        "本地终端",
		Address:     "localhost",
		AuthType:    model.AuthTypeLocal,
	}
	v.openNewHostSession(localHost)
}

// getHostSessionCount 获取指定主机当前已打开的终端标签数量
func (v *AppView) getHostSessionCount(hostID string) int {
	if hostID == "" {
		return 0
	}
	count := 0
	for _, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil && t.Session.Host != nil && t.Session.Host.ID == hostID {
			count++
		}
	}
	return count
}

// updateHostTabTitles 当主机的标签增减时，动态更新标题编号 (如仅一个显示 "Ubuntu-Server"，多个时显示 "Ubuntu-Server (1)", "Ubuntu-Server (2)")
func (v *AppView) updateHostTabTitles(hostID string) {
	if hostID == "" {
		return
	}
	var sessionIndices []int
	for i, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil && t.Session.Host != nil && t.Session.Host.ID == hostID {
			sessionIndices = append(sessionIndices, i)
		}
	}
	if len(sessionIndices) <= 1 {
		for _, idx := range sessionIndices {
			if v.tabs[idx].Session != nil && v.tabs[idx].Session.Host != nil && !v.tabs[idx].CustomTitle {
				v.tabs[idx].Title = v.tabs[idx].Session.Host.Name
				v.tabs[idx].Session.Title = v.tabs[idx].Session.Host.Name
			}
		}
	} else {
		for seq, idx := range sessionIndices {
			if v.tabs[idx].Session != nil && v.tabs[idx].Session.Host != nil && !v.tabs[idx].CustomTitle {
				numbered := fmt.Sprintf("%s (%d)", v.tabs[idx].Session.Host.Name, seq+1)
				v.tabs[idx].Title = numbered
				v.tabs[idx].Session.Title = numbered
			}
		}
	}
}

// openHostSession 打开或切换到主机的终端会话（若已有打开会话则切换，若无则新建）
func (v *AppView) openHostSession(h *model.Host) {
	v.recordRecentHost(h.ID)
	for i, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil && t.Session.Host.ID == h.ID {
			v.switchActivePaneToTab(i)
			return
		}
	}
	v.openNewHostSession(h)
}

// openNewHostSession 强制为指定主机创建并打开一个新的终端标签页
func (v *AppView) openNewHostSession(h *model.Host) {
	session := NewTerminalSession(h)
	session.OnUpdate = func() {
		if v.window != nil {
			v.window.Invalidate()
		}
	}
	session.OnSSHIntercept = func(targetHost *model.Host) {
		if targetHost.Password != "" {
			v.openNewHostSession(targetHost)
		} else {
			v.openQuickConnectWithHost(targetHost)
		}
		if v.window != nil {
			v.window.Invalidate()
		}
	}

	// 查找该主机是否已有分配好的专属色彩；若有则复用该色彩，保持同主机颜色一致
	hostColorIdx := -1
	for _, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil && t.Session.Host.ID == h.ID {
			hostColorIdx = t.Session.ColorIndex
			break
		}
	}

	if hostColorIdx < 0 {
		// 分配在当前所有打开会话中未被使用的节点专属色彩（方案 1: 同色系色标法）
		usedColors := make(map[int]bool)
		for _, t := range v.tabs {
			if t.Type == TabTypeSession && t.Session != nil {
				usedColors[t.Session.ColorIndex] = true
			}
		}
		for i := 0; i < len(HostColorPalette); i++ {
			if !usedColors[i] {
				hostColorIdx = i
				break
			}
		}
		if hostColorIdx < 0 {
			hostColorIdx = 0
		}
	}
	session.ColorIndex = hostColorIdx
	session.ShowHidden = v.showHiddenFiles

	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeSession,
		Title:   h.Name,
		Session: session,
	})
	v.switchActivePaneToTab(len(v.tabs) - 1)
	v.updateHostTabTitles(h.ID)
	v.recordRecentHost(h.ID)
	session.Start()
}

// openFileInEditor 在工作区打开或切换到指定文件的就地编辑器 Tab（方案 3：空间邻近吸附插入）
func (v *AppView) openFileInEditor(s *TerminalSession, entry sftpService.FileEntry) {
	for i, t := range v.tabs {
		if t.Type == TabTypeEditor && t.Editor != nil && t.Editor.Session == s && t.Editor.Path == entry.Path {
			v.switchActivePaneToTab(i)
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
		if v.tabs[i].Session != nil && v.tabs[i].Session.Host.ID == s.Host.ID {
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
		for p := 0; p < 4; p++ {
			if v.paneTabIndices[p] >= targetIdx {
				v.paneTabIndices[p]++
			}
		}
		v.switchActivePaneToTab(targetIdx)
	} else {
		v.tabs = append(v.tabs, newTab)
		v.switchActivePaneToTab(len(v.tabs) - 1)
	}
}

// closeTab 关闭指定索引的 Tab (带有未保存文件与活跃任务拦截守卫)
func (v *AppView) closeTab(tabIndex int) {
	if tabIndex <= 0 || tabIndex >= len(v.tabs) {
		return
	}

	target := v.tabs[tabIndex]

	// 1. 守卫：未保存的代码编辑器
	if target.Type == TabTypeEditor && target.Editor != nil {
		if target.Editor.Buffer != nil && target.Editor.Buffer.Modified() {
			if !v.confirmDiscardEditorTab(target.Editor) {
				return
			}
		}
	}

	// 2. 守卫：存在活跃后台传输、未保存关联文件或连接中的远程终端
	if target.Type == TabTypeSession && target.Session != nil {
		if !v.confirmCloseSessionTab(target.Session) {
			return
		}
	}

	var closedHostID string
	if target.Type == TabTypeSession && target.Session != nil {
		if target.Session.Host != nil {
			closedHostID = target.Session.Host.ID
		}
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

	if closedHostID != "" {
		v.updateHostTabTitles(closedHostID)
	}

	if v.activeTabIndex >= len(v.tabs) {
		v.activeTabIndex = len(v.tabs) - 1
	}
	if v.activeTabIndex < 0 {
		v.activeTabIndex = 0
	}
	v.onTabRemoved(tabIndex)
}

// closeOtherTabs 关闭除了指定索引之外的所有可关闭 Tab
func (v *AppView) closeOtherTabs(tabIndex int) {
	if tabIndex <= 0 || tabIndex >= len(v.tabs) {
		v.CloseAll()
		return
	}
	target := v.tabs[tabIndex]
	for i, t := range v.tabs {
		if i == 0 || i == tabIndex {
			continue
		}
		if t.Type == TabTypeSession && t.Session != nil && (target.Session == nil || t.Session != target.Session) {
			t.Session.Close()
		}
	}
	v.tabs = []WorkspaceTab{v.tabs[0], target}
	v.activeTabIndex = 1
	if target.Session != nil && target.Session.Host != nil {
		v.updateHostTabTitles(target.Session.Host.ID)
	}
	v.sanitizePanes()
}

// closeTabsToRight 关闭指定索引右侧的所有 Tab
func (v *AppView) closeTabsToRight(tabIndex int) {
	if tabIndex < 0 || tabIndex >= len(v.tabs)-1 {
		return
	}
	affectedHosts := make(map[string]bool)
	for i := tabIndex + 1; i < len(v.tabs); i++ {
		t := v.tabs[i]
		if t.Type == TabTypeSession && t.Session != nil {
			if t.Session.Host != nil {
				affectedHosts[t.Session.Host.ID] = true
			}
			t.Session.Close()
		}
	}
	v.tabs = v.tabs[:tabIndex+1]
	for hid := range affectedHosts {
		v.updateHostTabTitles(hid)
	}
	if v.activeTabIndex > tabIndex {
		v.activeTabIndex = tabIndex
	}
	v.sanitizePanes()
}

// duplicateTab 复制指定索引的标签页（在当前标签页后方就近插入一个副本）
func (v *AppView) duplicateTab(index int) {
	if index <= 0 || index >= len(v.tabs) {
		return
	}
	tab := v.tabs[index]
	switch tab.Type {
	case TabTypeSession:
		if tab.Session != nil && tab.Session.Host != nil {
			v.openNewHostSession(tab.Session.Host)
			// openNewHostSession 会将新 Tab 追加至尾部并更新标题
			// 将新 Tab 移动到当前 Tab 之后 (index + 1)
			lastIdx := len(v.tabs) - 1
			if lastIdx > index+1 {
				newTab := v.tabs[lastIdx]
				newTabs := make([]WorkspaceTab, 0, len(v.tabs))
				newTabs = append(newTabs, v.tabs[:index+1]...)
				newTabs = append(newTabs, newTab)
				newTabs = append(newTabs, v.tabs[index+1:lastIdx]...)
				v.tabs = newTabs
				v.activeTabIndex = index + 1
				v.updateHostTabTitles(tab.Session.Host.ID)
			}
		}
	case TabTypeEditor:
		if tab.Editor != nil && tab.Session != nil {
			edFile := NewEditorFile(tab.Session, tab.Editor.Path, tab.Editor.Name, tab.Editor.Size)
			edFile.LoadAsync()
			newTab := WorkspaceTab{
				Type:    TabTypeEditor,
				Title:   tab.Editor.Name,
				Session: tab.Session,
				Editor:  edFile,
			}
			targetIdx := index + 1
			newTabs := make([]WorkspaceTab, 0, len(v.tabs)+1)
			newTabs = append(newTabs, v.tabs[:targetIdx]...)
			newTabs = append(newTabs, newTab)
			newTabs = append(newTabs, v.tabs[targetIdx:]...)
			v.tabs = newTabs
			v.activeTabIndex = targetIdx
		}
	}
	v.sanitizePanes()
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
	v.sanitizePanes()
}

// AttachWindow 关联系统原生窗口实例，用于对话框的挂载与生命周期管理
func (v *AppView) AttachWindow(w *mygo.Window) {
	v.window = w
}

// WorkSummary 统计当前软件中正在进行的后台文件传输任务与未保存文件
type WorkSummary struct {
	ActiveTransfers int      // 后台进行中的 SFTP 传输任务总数
	TransferSpeed   string   // 传输速率
	UnsavedFiles    []string // 未保存的编辑器文件
}

// HasActiveWork 判断是否存在需要守护的未完成文件传输或未保存编辑
func (ws WorkSummary) HasActiveWork() bool {
	return ws.ActiveTransfers > 0 || len(ws.UnsavedFiles) > 0
}

// BuildConfirmMessage 构建退出软件时的详细提示与清单
func (ws WorkSummary) BuildConfirmMessage() (message, detail string) {
	message = "GoShell 仍有未完成的传输任务或未保存的文件，确定要退出吗？"

	var items []string
	if ws.ActiveTransfers > 0 {
		speedInfo := ""
		if ws.TransferSpeed != "" {
			speedInfo = fmt.Sprintf(" (%s)", ws.TransferSpeed)
		}
		items = append(items, fmt.Sprintf("• 后台有 %d 个文件传输任务正在进行%s，退出将直接中断传输", ws.ActiveTransfers, speedInfo))
	}
	if len(ws.UnsavedFiles) > 0 {
		items = append(items, fmt.Sprintf("• 有 %d 个文件存在未保存的修改 (%s)，退出将丢失这些编辑内容", len(ws.UnsavedFiles), strings.Join(ws.UnsavedFiles, ", ")))
	}

	detail = strings.Join(items, "\n")
	return message, detail
}

// GetWorkSummary 汇总当前所有标签页的文件传输与未保存文件状态
func (v *AppView) GetWorkSummary() WorkSummary {
	var ws WorkSummary
	for _, t := range v.tabs {
		if t.Type == TabTypeSession && t.Session != nil {
			if t.Session.TransferMgr != nil {
				cnt, speed := t.Session.TransferMgr.GetActiveStats()
				if cnt > 0 {
					ws.ActiveTransfers += cnt
					ws.TransferSpeed = speed
				}
			}
		}
		if t.Type == TabTypeEditor && t.Editor != nil {
			if t.Editor.Buffer != nil && t.Editor.Buffer.Modified() {
				ws.UnsavedFiles = append(ws.UnsavedFiles, t.Editor.Name)
			}
		}
	}
	return ws
}

// promptConfirm 对话框确认辅助函数，支持无 App / Headless 运行环境容错
func (v *AppView) promptConfirm(opts mygo.MessageOptions) bool {
	if v.window == nil {
		return true
	}
	defer func() {
		_ = recover()
	}()

	opts.Parent = v.window
	res, err := mygo.Dialog.Message(opts)
	if err != nil {
		return true
	}
	return res.Button == 0
}

// ConfirmQuit 软件退出前安全确认守卫 (仅在有正在传输的文件或未保存编辑时拦截)
func (v *AppView) ConfirmQuit() bool {
	if v.quitConfirmed {
		return true
	}
	ws := v.GetWorkSummary()
	if !ws.HasActiveWork() {
		return true
	}

	msg, detail := ws.BuildConfirmMessage()
	ok := v.promptConfirm(mygo.MessageOptions{
		Type:          mygo.MessageWarning,
		Title:         "退出 GoShell",
		Message:       msg,
		Detail:        detail,
		Buttons:       []string{"确认退出", "取消"},
		DefaultButton: 0,
		CancelButton:  1,
	})

	if ok {
		v.quitConfirmed = true
		return true
	}
	return false
}

// ConfirmClose 主窗口关闭前安全确认守卫
func (v *AppView) ConfirmClose() bool {
	if v.quitConfirmed {
		return true
	}
	return v.ConfirmQuit()
}

// confirmDiscardEditorTab 关闭未保存文件标签页时的拦截确认
func (v *AppView) confirmDiscardEditorTab(ef *EditorFile) bool {
	return v.promptConfirm(mygo.MessageOptions{
		Type:          mygo.MessageWarning,
		Title:         "关闭文件",
		Message:       fmt.Sprintf("文件“%s”有未保存的修改", ef.Name),
		Detail:        "如果关闭该标签页，尚未保存的代码修改将会丢失。",
		Buttons:       []string{"放弃修改并关闭", "取消"},
		DefaultButton: 0,
		CancelButton:  1,
	})
}

// confirmCloseSessionTab 关闭会话标签页时的拦截确认 (仅检测文件传输和未保存的关联文件)
func (v *AppView) confirmCloseSessionTab(s *TerminalSession) bool {
	// A. 检查后台传输
	var activeTransfers int
	var transferSpeed string
	if s.TransferMgr != nil {
		activeTransfers, transferSpeed = s.TransferMgr.GetActiveStats()
	}
	if activeTransfers > 0 {
		speedInfo := ""
		if transferSpeed != "" {
			speedInfo = fmt.Sprintf(" (%s)", transferSpeed)
		}
		return v.promptConfirm(mygo.MessageOptions{
			Type:          mygo.MessageWarning,
			Title:         "关闭会话",
			Message:       fmt.Sprintf("会话“%s”有后台传输任务正在进行", s.Host.Name),
			Detail:        fmt.Sprintf("当前正在进行 %d 个文件传输任务%s，关闭会话将强制中断传输。确定要关闭吗？", activeTransfers, speedInfo),
			Buttons:       []string{"强制中断并关闭", "取消"},
			DefaultButton: 0,
			CancelButton:  1,
		})
	}

	// B. 检查属于该会话的未保存文件编辑器
	var unsavedFiles []string
	for _, t := range v.tabs {
		if t.Type == TabTypeEditor && t.Editor != nil && t.Editor.Session == s {
			if t.Editor.Buffer != nil && t.Editor.Buffer.Modified() {
				unsavedFiles = append(unsavedFiles, t.Editor.Name)
			}
		}
	}
	if len(unsavedFiles) > 0 {
		return v.promptConfirm(mygo.MessageOptions{
			Type:          mygo.MessageWarning,
			Title:         "关闭会话",
			Message:       fmt.Sprintf("会话“%s”有关联文件尚未保存", s.Host.Name),
			Detail:        fmt.Sprintf("文件 (%s) 存在未保存的修改，关闭会话将一并关闭并放弃这些修改。确定要关闭吗？", strings.Join(unsavedFiles, ", ")),
			Buttons:       []string{"放弃修改并关闭", "取消"},
			DefaultButton: 0,
			CancelButton:  1,
		})
	}

	return true
}
