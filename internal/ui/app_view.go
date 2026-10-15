package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"goshell/internal/app"
	"goshell/internal/core/model"
)

// AppView 管理主窗口的 UI 状态与渲染循环
type AppView struct {
	app *app.App

	// 布局尺寸
	sidebarWidth float32

	// 交互状态
	searchQuery    string
	activeTabIndex int
	tabs           []string
	selectedHostID string

	// 新建/编辑主机弹窗表单状态
	modalOpen      bool
	formID         string
	formName       string
	formAddress    string
	formPort       string
	formUsername   string
	formAuthType   string // "password" 或 "key"
	formPassword   string
	formKeyPath    string
	formGroupID    string
	formErrorMsg   string
}

// NewAppView 初始化主窗口 UI 状态
func NewAppView(application *app.App) *AppView {
	return &AppView{
		app:          application,
		sidebarWidth: 260,
		tabs:         []string{"欢迎与资产概览"},
		formPort:     "22",
		formUsername: "root",
		formAuthType: "password",
	}
}

// View 每一帧渲染入口（在 GPU 上直接自绘）
func (v *AppView) View(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		// 1. 顶部工具控制栏
		v.renderTopBar(c, t)

		// 2. 主体工作区（水平分屏：左侧边栏 + 右侧工作区）
		ui.Split(c, &v.sidebarWidth, func() {
			v.renderSidebar(c, t)
		}, func() {
			v.renderWorkspace(c, t)
		}).Fill()

		// 3. 底部状态栏
		v.renderStatusBar(c, t)
	})

	// 4. 浮层弹窗（新建/编辑主机）
	if v.modalOpen {
		v.renderHostModal(c, t)
	}
}

// 顶部工具栏
func (v *AppView) renderTopBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(46).Background(t.Surface).Padding(t.Space(2), t.Space(4)).Gap(t.Space(3)).AlignItems(ui.Center).Children(func() {
		// Logo
		ui.Text(c, "⚡ GoShell").FontSize(16).Bold().TextColor(t.Accent)

		ui.Text(c, "|").TextColor(t.Border)

		// 新建连接按钮
		if ui.PrimaryButton(c, "+ 新建主机").Clicked() {
			v.openCreateModal()
		}

		// 搜索过滤框
		ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "🔍").FontSize(12)
			ui.TextInput(c, &v.searchQuery).Width(180)
		})

		ui.Spacer(c)

		// 资产与会话快速统计微标
		ui.Badge(c, fmt.Sprintf("主机总数: %d", len(v.app.Hosts)))
		ui.Badge(c, "免 cgo · GPU 原生自绘")
	})
}

// 左侧资产树与主机列表
func (v *AppView) renderSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(t.Background).Padding(t.Space(3)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "服务器资产").FontSize(13).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "刷新").Clicked() {
				_ = v.app.ReloadData()
			}
		})

		ui.Divider(c)

		// 滚动主机列表
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(t.Space(2)).Children(func() {
				query := strings.ToLower(strings.TrimSpace(v.searchQuery))

				for _, group := range v.app.Groups {
					// 过滤该组下的主机
					var groupHosts []*model.Host
					for _, h := range v.app.Hosts {
						if h.GroupID == group.ID {
							if query == "" || strings.Contains(strings.ToLower(h.Name), query) || strings.Contains(strings.ToLower(h.Address), query) {
								groupHosts = append(groupHosts, h)
							}
						}
					}

					if len(groupHosts) == 0 && query != "" {
						continue
					}

					// 分组标题
					ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "📁").FontSize(12)
						ui.Text(c, group.Name).FontSize(12).Bold().TextColor(t.Text)
						ui.Text(c, fmt.Sprintf("(%d)", len(groupHosts))).FontSize(11).TextColor(t.TextMuted)
					})

					// 该组下的主机列表项
					for _, h := range groupHosts {
						host := h // 闭包变量捕获
						isSelected := v.selectedHostID == host.ID

						cardBg := t.Surface
						if isSelected {
							cardBg = t.SurfaceHover
						}

						ui.Row(c).Background(cardBg).Padding(t.Space(2), t.Space(3)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							// 状态指示圆点
							ui.Text(c, "●").TextColor(t.Success).FontSize(10)

							// 主机名称与地址
							ui.Column(c).Grow(1).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, host.Name).FontSize(13).Bold()
								ui.Text(c, fmt.Sprintf("%s@%s:%d", host.Username, host.Address, host.Port)).FontSize(11).TextColor(t.TextMuted)
							})

							// 连接按钮
							if ui.Button(c, "连接").Clicked() {
								v.openHostTab(host)
							}
						}).OnClick(func() {
							v.selectedHostID = host.ID
						})
					}
				}
			})
		})
	})
}

// 右侧工作区
func (v *AppView) renderWorkspace(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 标签栏
		if len(v.tabs) > 0 {
			ui.Tabs(c, &v.activeTabIndex, v.tabs...)
		}

		ui.Divider(c)

		// 标签页内容区域
		if v.activeTabIndex == 0 {
			v.renderWelcomeDashboard(c, t)
		} else {
			v.renderActiveSessionTab(c, t)
		}
	})
}

// 欢迎与概览面板
func (v *AppView) renderWelcomeDashboard(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Fill().Padding(t.Space(6)).Children(func() {
		ui.Column(c).Gap(t.Space(5)).Children(func() {
			ui.Text(c, "欢迎使用 GoShell 远程终端与资产管理器").FontSize(22).Bold().TextColor(t.Text)
			ui.Text(c, "基于 MyGo 纯 Go 框架与 GPU 原生自绘引擎打造，秒级启动，无 Electron 臃肿感。").FontSize(13).TextColor(t.TextMuted)

			ui.Divider(c)

			// 快捷功能卡片
			ui.Row(c).Gap(t.Space(4)).Children(func() {
				// 卡片 1
				ui.Column(c).Background(t.Surface).Padding(t.Space(4)).Radius(t.Radius).Width(240).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "🚀 快速连接").FontSize(15).Bold()
					ui.Text(c, "点击左侧保存的主机或添加新服务器，立即建立双向 SSH 与 SFTP 会话。").FontSize(12).TextColor(t.TextMuted)
					if ui.PrimaryButton(c, "+ 添加新服务器").Clicked() {
						v.openCreateModal()
					}
				})

				// 卡片 2
				ui.Column(c).Background(t.Surface).Padding(t.Space(4)).Radius(t.Radius).Width(240).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "🔒 凭据落盘保护").FontSize(15).Bold()
					ui.Text(c, "主机密码与私钥均使用 AES-256-GCM 工业级加密存储于本地纯 Go SQLite。").FontSize(12).TextColor(t.TextMuted)
					ui.Badge(c, "已加密保护")
				})

				// 卡片 3
				ui.Column(c).Background(t.Surface).Padding(t.Space(4)).Radius(t.Radius).Width(240).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "📈 实时性能看板").FontSize(15).Bold()
					ui.Text(c, "连接服务器后将自动无侵入采样 CPU、内存、网络波形，媲美 FinalShell。").FontSize(12).TextColor(t.TextMuted)
					ui.Badge(c, "下一阶段即将接入")
				})
			})

			// 当前选中主机详细卡片
			if v.selectedHostID != "" {
				var host *model.Host
				for _, h := range v.app.Hosts {
					if h.ID == v.selectedHostID {
						host = h
						break
					}
				}

				if host != nil {
					ui.Column(c).Background(t.Surface).Padding(t.Space(4)).Radius(t.Radius).Gap(t.Space(3)).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("选中的主机: %s", host.Name)).FontSize(16).Bold()
							ui.Spacer(c)
							if ui.Button(c, "✏ 编辑").Clicked() {
								v.openEditModal(host)
							}
							if ui.Button(c, "🗑 删除").Clicked() {
								_ = v.app.Store.DeleteHost(host.ID)
								v.selectedHostID = ""
								_ = v.app.ReloadData()
							}
						})

						ui.Row(c).Gap(t.Space(6)).Children(func() {
							ui.Text(c, fmt.Sprintf("IP/域名: %s", host.Address)).FontSize(13)
							ui.Text(c, fmt.Sprintf("端口: %d", host.Port)).FontSize(13)
							ui.Text(c, fmt.Sprintf("用户: %s", host.Username)).FontSize(13)
							ui.Text(c, fmt.Sprintf("认证方式: %s", host.AuthType)).FontSize(13)
						})

						if host.Description != "" {
							ui.Text(c, fmt.Sprintf("备注: %s", host.Description)).FontSize(12).TextColor(t.TextMuted)
						}

						ui.Row(c).Gap(t.Space(2)).Children(func() {
							if ui.PrimaryButton(c, "▶ 开启 SSH 终端与文件管理").Clicked() {
								v.openHostTab(host)
							}
						})
					})
				}
			}
		})
	})
}

// 会话标签页展示区
func (v *AppView) renderActiveSessionTab(c *ui.Context, t *ui.Theme) {
	tabTitle := v.tabs[v.activeTabIndex]

	ui.Column(c).Fill().Padding(t.Space(6)).Center().Gap(t.Space(4)).Children(func() {
		ui.Text(c, "🖥️").FontSize(48)
		ui.Text(c, fmt.Sprintf("会话已就绪: %s", tabTitle)).FontSize(20).Bold()
		ui.Text(c, "Phase 1（基础设施与数据持久化层）已验证完成！").FontSize(14).TextColor(t.Success)
		ui.Text(c, "Phase 2 将在此区域直接挂载原生 Ghostty VT 终端与 SSH 流。").FontSize(13).TextColor(t.TextMuted)

		if ui.Button(c, "关闭当前标签页").Clicked() {
			v.closeCurrentTab()
		}
	})
}

// 底部状态栏
func (v *AppView) renderStatusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(26).Background(t.Surface).Padding(t.Space(1), t.Space(3)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "● 就绪").TextColor(t.Success).FontSize(11)
		ui.Text(c, " | 本地数据库: "+v.app.DataDir+"/goshell.db").FontSize(11).TextColor(t.TextMuted)
		ui.Spacer(c)
		ui.Text(c, "GoShell v0.1.0-alpha (Phase 1 Ready)").FontSize(11).TextColor(t.TextMuted)
	})
}

// 新建/编辑主机弹窗
func (v *AppView) renderHostModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.modalOpen, func() {
		ui.Column(c).Width(420).Background(t.Background).Padding(t.Space(5)).Radius(t.Radius).Gap(t.Space(3)).Children(func() {
			title := "添加新主机"
			if v.formID != "" {
				title = "编辑主机配置"
			}
			ui.Text(c, title).FontSize(16).Bold()

			if v.formErrorMsg != "" {
				ui.Text(c, v.formErrorMsg).FontSize(12).TextColor(t.Danger)
			}

			// 字段：名称
			ui.Text(c, "显示别名:").FontSize(12)
			ui.TextInput(c, &v.formName)

			// 字段：主机与端口
			ui.Row(c).Gap(t.Space(2)).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "主机 IP / 域名:").FontSize(12)
					ui.TextInput(c, &v.formAddress)
				})
				ui.Column(c).Width(80).Children(func() {
					ui.Text(c, "端口:").FontSize(12)
					ui.TextInput(c, &v.formPort)
				})
			})

			// 字段：用户名
			ui.Text(c, "登录用户名:").FontSize(12)
			ui.TextInput(c, &v.formUsername)

			// 认证方式选择
			ui.Row(c).Gap(t.Space(4)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "认证方式:").FontSize(12)
				if ui.Button(c, "密码认证").Clicked() {
					v.formAuthType = "password"
				}
				if ui.Button(c, "私钥认证").Clicked() {
					v.formAuthType = "key"
				}
				ui.Badge(c, "当前: "+v.formAuthType)
			})

			if v.formAuthType == "password" {
				ui.Text(c, "密码:").FontSize(12)
				ui.TextInput(c, &v.formPassword)
			} else {
				ui.Text(c, "私钥路径 (如 ~/.ssh/id_rsa):").FontSize(12)
				ui.TextInput(c, &v.formKeyPath)
			}

			ui.Divider(c)

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
	if strings.TrimSpace(v.formAddress) == "" {
		v.formErrorMsg = "请输入主机 IP 或域名"
		return
	}
	port, err := strconv.Atoi(strings.TrimSpace(v.formPort))
	if err != nil || port <= 0 {
		v.formErrorMsg = "请输入有效端口号 (1-65535)"
		return
	}

	h := &model.Host{
		ID:          v.formID,
		GroupID:     v.formGroupID,
		Name:        strings.TrimSpace(v.formName),
		Address:     strings.TrimSpace(v.formAddress),
		Port:        port,
		Username:    strings.TrimSpace(v.formUsername),
		AuthType:    model.AuthType(v.formAuthType),
		Password:    v.formPassword,
		KeyPath:     strings.TrimSpace(v.formKeyPath),
		Description: "用户保存的连接",
		ColorTag:    "#10B981",
	}

	if err := v.app.Store.SaveHost(h); err != nil {
		v.formErrorMsg = fmt.Sprintf("保存失败: %v", err)
		return
	}

	_ = v.app.ReloadData()
	v.selectedHostID = h.ID
	v.modalOpen = false
}

// 在标签栏中打开主机会话
func (v *AppView) openHostTab(h *model.Host) {
	title := h.Name
	for i, tab := range v.tabs {
		if tab == title {
			v.activeTabIndex = i
			return
		}
	}

	v.tabs = append(v.tabs, title)
	v.activeTabIndex = len(v.tabs) - 1
}

// 关闭当前标签页
func (v *AppView) closeCurrentTab() {
	if v.activeTabIndex == 0 {
		return // 不关闭欢迎概览页
	}
	v.tabs = append(v.tabs, v.tabs[:v.activeTabIndex]...)
	v.tabs = append(v.tabs, v.tabs[v.activeTabIndex+1:]...)
	v.activeTabIndex = max(0, v.activeTabIndex-1)
}
