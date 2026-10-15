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

	// 主题配置
	theme *ui.Theme

	// 布局尺寸
	sidebarWidth float32

	// 交互状态
	searchQuery    string
	activeTabIndex int
	tabs           []string
	selectedHostID string

	// 新建/编辑主机弹窗表单状态
	modalOpen    bool
	formID       string
	formName     string
	formAddress  string
	formPort     string
	formUsername string
	formAuthType string // "password" 或 "key"
	formPassword string
	formKeyPath  string
	formGroupID  string
	formErrorMsg string
}

// NewAppView 初始化主窗口 UI 状态
func NewAppView(application *app.App) *AppView {
	return &AppView{
		app:          application,
		theme:        ModernDarkTheme(),
		sidebarWidth: 260,
		tabs:         []string{"欢迎与概览"},
		formPort:     "22",
		formUsername: "root",
		formAuthType: "password",
	}
}

// View 每一帧渲染入口（GPU 原生自绘）
func (v *AppView) View(c *ui.Context) {
	// 应用专业极客深色主题
	c.SetTheme(v.theme)
	t := c.Theme()

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 1. 顶部现代化无缝控制栏
		v.renderTopBar(c, t)

		// 2. 主体工作区（水平分屏：左侧边栏 + 右侧工作区）
		ui.Split(c, &v.sidebarWidth, func() {
			v.renderSidebar(c, t)
		}, func() {
			v.renderWorkspace(c, t)
		}).Fill()

		// 3. 底部极简状态栏
		v.renderStatusBar(c, t)
	})

	// 4. 浮层弹窗（新建/编辑主机）
	if v.modalOpen {
		v.renderHostModal(c, t)
	}
}

// 顶部现代化控制栏 (类似 Linear / Zed 顶栏)
func (v *AppView) renderTopBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(44).Background(ui.Hex("#0D121F")).Padding(t.Space(2), t.Space(4)).Gap(t.Space(3)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
		// Logo 品牌标识
		ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "⚡").FontSize(16)
			ui.Text(c, "GoShell").FontSize(15).Bold().TextColor(ui.Hex("#38BDF8"))
		})

		ui.Text(c, "•").TextColor(ui.Hex("#334155")).FontSize(12)

		// 新建主机按钮
		if ui.PrimaryButton(c, "+ 新建主机").Clicked() {
			v.openCreateModal()
		}

		// 胶囊搜索框
		ui.Row(c).Background(ui.Hex("#172033")).Padding(t.Space(1), t.Space(2.5)).Radius(16).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "⌕").FontSize(13).TextColor(t.TextMuted)
			ui.TextInput(c, &v.searchQuery).Width(160)
		})

		ui.Spacer(c)

		// 顶部状态信息微标
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Background(ui.Hex("#131C2E")).Padding(t.Space(1), t.Space(2.5)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Text(c, "●").TextColor(t.Success).FontSize(9)
				ui.Text(c, fmt.Sprintf("%d 台主机", len(v.app.Hosts))).FontSize(11).TextColor(t.Text)
			})

			ui.Row(c).Background(ui.Hex("#131C2E")).Padding(t.Space(1), t.Space(2.5)).Radius(t.Radius).Children(func() {
				ui.Text(c, "Zero CGO · Metal GPU").FontSize(11).TextColor(ui.Hex("#38BDF8"))
			})
		})
	})
}

// 左侧资产树与主机列表
func (v *AppView) renderSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(ui.Hex("#0B0F19")).Padding(t.Space(2.5)).Gap(t.Space(2)).Border(1, t.Border).Children(func() {
		// 边栏标题与操作
		ui.Row(c).Padding(t.Space(1), t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "ASSETS").FontSize(11).Bold().TextColor(ui.Hex("#64748B"))
			ui.Spacer(c)
			if ui.Button(c, "刷新").Clicked() {
				_ = v.app.ReloadData()
			}
		})

		// 滚动主机列表
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(t.Space(3)).Children(func() {
				query := strings.ToLower(strings.TrimSpace(v.searchQuery))

				for _, group := range v.app.Groups {
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

					// 分组头部（折叠胶囊感）
					ui.Row(c).Padding(t.Space(1), t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
						ui.Text(c, "▾").FontSize(10).TextColor(ui.Hex("#64748B"))
						ui.Text(c, group.Name).FontSize(12).Bold().TextColor(ui.Hex("#94A3B8"))
						ui.Spacer(c)
						ui.Text(c, fmt.Sprintf("%d", len(groupHosts))).FontSize(11).TextColor(ui.Hex("#475569"))
					})

					// 该组下的主机卡片
					for _, h := range groupHosts {
						host := h
						isSelected := v.selectedHostID == host.ID

						cardBg := ui.Hex("#111827")
						borderColor := ui.Hex("#1E293B")
						if isSelected {
							cardBg = ui.Hex("#1E293B")
							borderColor = ui.Hex("#38BDF8")
						}

						ui.Row(c).Background(cardBg).Border(1, borderColor).Padding(t.Space(2.5), t.Space(3)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2.5)).Children(func() {
							// 状态指示圆点
							ui.Text(c, "●").TextColor(t.Success).FontSize(8)

							// 主机名与连接串
							ui.Column(c).Grow(1).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, host.Name).FontSize(13).Bold().TextColor(t.Text)
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
	ui.Column(c).Fill().Background(ui.Hex("#090D16")).Children(func() {
		// 现代化标签栏
		if len(v.tabs) > 0 {
			ui.Row(c).Height(38).Background(ui.Hex("#0D121F")).Padding(t.Space(1), t.Space(3)).Gap(t.Space(1)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
				for i, tab := range v.tabs {
					index := i
					title := tab
					isActive := v.activeTabIndex == index

					tabBg := ui.Hex("#0D121F")
					textColor := ui.Hex("#94A3B8")
					borderCol := ui.Hex("#0D121F")
					if isActive {
						tabBg = ui.Hex("#162032")
						textColor = ui.Hex("#F8FAFC")
						borderCol = ui.Hex("#38BDF8")
					}

					ui.Row(c).Background(tabBg).Border(1, borderCol).Radius(t.Radius).Padding(t.Space(1.5), t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						if index == 0 {
							ui.Text(c, "✦").FontSize(11).TextColor(ui.Hex("#38BDF8"))
						} else {
							ui.Text(c, ">_").FontSize(11).TextColor(t.Success)
						}
						ui.Text(c, title).FontSize(12).TextColor(textColor).Bold()

						// 允许关闭非主页 Tab
						if index > 0 {
							if ui.Button(c, "×").Clicked() {
								v.closeTab(index)
							}
						}
					}).OnClick(func() {
						v.activeTabIndex = index
					})
				}
			})
		}

		// 标签页内容
		if v.activeTabIndex == 0 {
			v.renderWelcomeDashboard(c, t)
		} else {
			v.renderActiveSessionTab(c, t)
		}
	})
}

// 欢迎与资产概览面板（深色现代化卡片风格）
func (v *AppView) renderWelcomeDashboard(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Fill().Padding(t.Space(7)).Children(func() {
		ui.Column(c).Gap(t.Space(6)).Children(func() {
			// 标题与理念
			ui.Column(c).Gap(t.Space(1.5)).Children(func() {
				ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "GoShell 远程终端控制台").FontSize(24).Bold().TextColor(t.Text)
					ui.Row(c).Background(ui.Hex("#0369A1")).Padding(t.Space(0.5), t.Space(2)).Radius(12).Children(func() {
						ui.Text(c, "v0.1.0-alpha").FontSize(11).TextColor(ui.Hex("#FFFFFF"))
					})
				})
				ui.Text(c, "纯 Go 编写、免 cgo、GPU 原生自绘。告别 Electron 臃肿与 Java 迟缓，秒开秒连。").FontSize(13).TextColor(t.TextMuted)
			})

			// 核心指标与功能卡片
			ui.Row(c).Gap(t.Space(4)).Children(func() {
				// 卡片 1: 极速会话
				ui.Column(c).Background(ui.Hex("#111827")).Border(1, ui.Hex("#1F293D")).Padding(t.Space(5)).Radius(t.Radius).Width(250).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "🚀 极速原生会话").FontSize(14).Bold().TextColor(ui.Hex("#38BDF8"))
					ui.Text(c, "内置 Ghostty 终端渲染引擎与多标签分屏，即刻开启低延迟交互。").FontSize(12).TextColor(t.TextMuted)
					if ui.PrimaryButton(c, "+ 快速添加服务器").Clicked() {
						v.openCreateModal()
					}
				})

				// 卡片 2: 凭据保险库
				ui.Column(c).Background(ui.Hex("#111827")).Border(1, ui.Hex("#1F293D")).Padding(t.Space(5)).Radius(t.Radius).Width(250).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "🔒 AES-256 安全保险库").FontSize(14).Bold().TextColor(ui.Hex("#10B981"))
					ui.Text(c, "主机密码与密钥经过强散列加密落地于本地纯 Go SQLite。").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "✓ 凭据已加密").FontSize(11).TextColor(t.Success)
				})

				// 卡片 3: SFTP 与性能监控
				ui.Column(c).Background(ui.Hex("#111827")).Border(1, ui.Hex("#1F293D")).Padding(t.Space(5)).Radius(t.Radius).Width(250).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "📈 FinalShell 看板").FontSize(14).Bold().TextColor(ui.Hex("#F59E0B"))
					ui.Text(c, "即将在此提供实时 CPU、内存、网络波形与 SFTP 拖拽传输。").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "即将进入 Phase 2").FontSize(11).TextColor(t.Warning)
				})
			})

			// 选中的主机详情卡片
			if v.selectedHostID != "" {
				var host *model.Host
				for _, h := range v.app.Hosts {
					if h.ID == v.selectedHostID {
						host = h
						break
					}
				}

				if host != nil {
					ui.Column(c).Background(ui.Hex("#131B2E")).Border(1, ui.Hex("#2563EB")).Padding(t.Space(5)).Radius(t.Radius).Gap(t.Space(3.5)).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "●").TextColor(t.Success).FontSize(10)
								ui.Text(c, host.Name).FontSize(16).Bold().TextColor(t.Text)
							})
							ui.Spacer(c)
							if ui.Button(c, "✏ 编辑配置").Clicked() {
								v.openEditModal(host)
							}
							if ui.Button(c, "🗑 删除").Clicked() {
								_ = v.app.Store.DeleteHost(host.ID)
								v.selectedHostID = ""
								_ = v.app.ReloadData()
							}
						})

						ui.Row(c).Gap(t.Space(6)).Children(func() {
							ui.Column(c).Gap(t.Space(1)).Children(func() {
								ui.Text(c, "主机地址").FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, fmt.Sprintf("%s:%d", host.Address, host.Port)).FontSize(13).Bold()
							})
							ui.Column(c).Gap(t.Space(1)).Children(func() {
								ui.Text(c, "登录用户").FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, host.Username).FontSize(13).Bold()
							})
							ui.Column(c).Gap(t.Space(1)).Children(func() {
								ui.Text(c, "认证方式").FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, string(host.AuthType)).FontSize(13).Bold()
							})
						})

						if host.Description != "" {
							ui.Text(c, "备注: "+host.Description).FontSize(12).TextColor(t.TextMuted)
						}

						ui.Row(c).Gap(t.Space(3)).Children(func() {
							if ui.PrimaryButton(c, "▶ 立即连接 SSH 终端").Clicked() {
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

	ui.Column(c).Fill().Padding(t.Space(8)).Center().Gap(t.Space(4)).Children(func() {
		ui.Text(c, "⚡").FontSize(44)
		ui.Text(c, fmt.Sprintf("会话已就绪: %s", tabTitle)).FontSize(20).Bold().TextColor(t.Text)
		ui.Text(c, "Phase 1: 基础设施、纯 Go 本地持久化与现代深色 UI 骨架已就绪").FontSize(13).TextColor(t.Success)
		ui.Text(c, "在下一阶段，此处将直接接入 Ghostty VT 终端并渲染 SSH 数据流。").FontSize(13).TextColor(t.TextMuted)

		if ui.Button(c, "关闭当前标签页").Clicked() {
			v.closeTab(v.activeTabIndex)
		}
	})
}

// 底部极简状态栏
func (v *AppView) renderStatusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(26).Background(ui.Hex("#0B0F19")).Padding(t.Space(1), t.Space(4)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
		ui.Text(c, "● 就绪").TextColor(t.Success).FontSize(11)
		ui.Text(c, " | 本地 SQLite: "+v.app.DataDir+"/goshell.db").FontSize(11).TextColor(ui.Hex("#64748B"))
		ui.Spacer(c)
		ui.Text(c, "GoShell v0.1.0 (Phase 1 Ready)").FontSize(11).TextColor(ui.Hex("#64748B"))
	})
}

// 新建/编辑主机弹窗
func (v *AppView) renderHostModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.modalOpen, func() {
		ui.Column(c).Width(440).Background(ui.Hex("#111827")).Border(1, ui.Hex("#334155")).Padding(t.Space(6)).Radius(12).Gap(t.Space(3.5)).Children(func() {
			title := "添加新主机"
			if v.formID != "" {
				title = "编辑主机配置"
			}
			ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)

			if v.formErrorMsg != "" {
				ui.Text(c, v.formErrorMsg).FontSize(12).TextColor(t.Danger)
			}

			// 字段：名称
			ui.Text(c, "显示别名:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.formName)

			// 字段：主机与端口
			ui.Row(c).Gap(t.Space(2)).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "主机 IP / 域名:").FontSize(12).TextColor(t.TextMuted)
					ui.TextInput(c, &v.formAddress)
				})
				ui.Column(c).Width(90).Children(func() {
					ui.Text(c, "端口:").FontSize(12).TextColor(t.TextMuted)
					ui.TextInput(c, &v.formPort)
				})
			})

			// 字段：用户名
			ui.Text(c, "登录用户名:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.formUsername)

			// 认证方式选择
			ui.Row(c).Gap(t.Space(3)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "认证方式:").FontSize(12).TextColor(t.TextMuted)
				if ui.Button(c, "密码认证").Clicked() {
					v.formAuthType = "password"
				}
				if ui.Button(c, "私钥认证").Clicked() {
					v.formAuthType = "key"
				}
				ui.Row(c).Background(ui.Hex("#1E293B")).Padding(t.Space(0.5), t.Space(2)).Radius(8).Children(func() {
					ui.Text(c, "当前: "+v.formAuthType).FontSize(11).TextColor(ui.Hex("#38BDF8"))
				})
			})

			if v.formAuthType == "password" {
				ui.Text(c, "密码:").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &v.formPassword)
			} else {
				ui.Text(c, "私钥路径 (如 ~/.ssh/id_rsa):").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &v.formKeyPath)
			}

			// 底部操作按钮
			ui.Row(c).Gap(t.Space(2.5)).Children(func() {
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

// 关闭指定 Tab
func (v *AppView) closeTab(index int) {
	if index <= 0 || index >= len(v.tabs) {
		return
	}
	v.tabs = append(v.tabs[:index], v.tabs[index+1:]...)
	if v.activeTabIndex >= len(v.tabs) {
		v.activeTabIndex = len(v.tabs) - 1
	}
}
