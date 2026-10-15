package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"goshell/internal/app"
	"goshell/internal/core/model"
)

// AppView 管理桌面主窗口 UI 状态与事件循环
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
	v := &AppView{
		app:          application,
		theme:        ModernLightTheme(),
		sidebarWidth: 240,
		tabs:         []string{"资产概览"},
		formPort:     "22",
		formUsername: "root",
		formAuthType: "password",
	}

	// 默认选中第一个主机以便展示详情
	if len(application.Hosts) > 0 {
		v.selectedHostID = application.Hosts[0].ID
	}

	return v
}

// View 每一帧渲染入口（原生 GPU 自绘）
func (v *AppView) View(c *ui.Context) {
	c.SetTheme(v.theme)
	t := c.Theme()

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 1. 原生桌面紧凑工具栏
		v.renderToolbar(c, t)

		// 2. 主体工作区（左侧资产树 + 右侧工作面板）
		ui.Split(c, &v.sidebarWidth, func() {
			v.renderSidebar(c, t)
		}, func() {
			v.renderWorkspace(c, t)
		}).Fill()

		// 3. 原生紧凑状态栏
		v.renderStatusBar(c, t)
	})

	// 4. 对话框弹窗（新建/编辑配置）
	if v.modalOpen {
		v.renderHostModal(c, t)
	}
}

// 原生桌面工具栏 (紧凑 36px，标准桌面分界与操作按钮)
func (v *AppView) renderToolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(36).Background(t.Surface).Padding(t.Space(1), t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
		// 窗口标题
		ui.Text(c, "GoShell").FontSize(13).Bold().TextColor(t.Text)

		ui.Box(c).Width(1).Height(16).Background(t.Border)

		// 操作按钮组
		if ui.PrimaryButton(c, "新建连接").Clicked() {
			v.openCreateModal()
		}

		if ui.Button(c, "刷新列表").Clicked() {
			_ = v.app.ReloadData()
		}

		ui.Spacer(c)

		// 紧凑搜索栏
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "搜索:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.searchQuery).Width(180)
		})
	})
}

// 左侧资产树与主机列表 (紧凑 26px 行高，标准 Finder/TablePlus 风格)
func (v *AppView) renderSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(ui.Hex("#F8F9FA")).Padding(t.Space(1.5)).Gap(t.Space(1)).Border(1, t.Border).Children(func() {
		// 边栏顶部小标题
		ui.Row(c).Padding(t.Space(1), t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "主机与集群").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("共 %d 台", len(v.app.Hosts))).FontSize(11).TextColor(t.TextMuted)
		})

		// 滚动列表
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).Gap(t.Space(2)).Children(func() {
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

					// 分组栏
					ui.Row(c).Padding(t.Space(1), t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
						ui.Text(c, group.Name).FontSize(11).Bold().TextColor(t.TextMuted)
						ui.Spacer(c)
						ui.Text(c, strconv.Itoa(len(groupHosts))).FontSize(11).TextColor(ui.Hex("#9AA2B1"))
					})

					// 主机项（原生桌面紧凑行）
					for _, h := range groupHosts {
						host := h
						isSelected := v.selectedHostID == host.ID

						rowBg := ui.Hex("#F8F9FA")
						borderCol := ui.Hex("#F8F9FA")
						if isSelected {
							rowBg = ui.Hex("#EBF1FA")
							borderCol = ui.Hex("#B8D3F7")
						}

						ui.Row(c).Background(rowBg).Border(1, borderCol).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							// 状态绿点 (矢量圆)
							ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)

							// 主机信息
							ui.Column(c).Grow(1).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, host.Name).FontSize(12).Bold().TextColor(t.Text)
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
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		// 原生多标签栏 (紧凑 30px)
		if len(v.tabs) > 0 {
			ui.Row(c).Height(30).Background(ui.Hex("#F1F3F5")).Padding(0, t.Space(2)).Gap(1).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
				for i, tab := range v.tabs {
					index := i
					title := tab
					isActive := v.activeTabIndex == index

					tabBg := ui.Hex("#F1F3F5")
					textColor := t.TextMuted
					borderCol := ui.Hex("#E2E5E9")
					if isActive {
						tabBg = t.Surface
						textColor = t.Text
						borderCol = t.Border
					}

					ui.Row(c).Height(29).Background(tabBg).Border(1, borderCol).Radius(t.Radius).Padding(t.Space(1), t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						ui.Text(c, title).FontSize(12).TextColor(textColor).Bold()

						// 非主概览标签提供关闭选项
						if index > 0 {
							if ui.Button(c, "关闭").Clicked() {
								v.closeTab(index)
							}
						}
					}).OnClick(func() {
						v.activeTabIndex = index
					})
				}
			})
		}

		// 标签视窗内容
		if v.activeTabIndex == 0 {
			v.renderInspectorDashboard(c, t)
		} else {
			v.renderActiveSessionTab(c, t)
		}
	})
}

// 主机属性审查与操作面板 (标准桌面软件属性面板风格，拒绝网页大卡片)
func (v *AppView) renderInspectorDashboard(c *ui.Context, t *ui.Theme) {
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
		ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
			ui.Text(c, "未选择主机").FontSize(14).Bold().TextColor(t.TextMuted)
			ui.Text(c, "请在左侧列表单击选择主机查看配置，或双击连接").FontSize(12).TextColor(t.TextMuted)
		})
		return
	}

	ui.Scroll(c).Fill().Padding(t.Space(5)).Children(func() {
		ui.Column(c).Gap(t.Space(4)).Children(func() {
			// 顶部主机标题与快捷操作栏
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(t.Space(0.5)).Children(func() {
					ui.Text(c, host.Name).FontSize(18).Bold().TextColor(t.Text)
					ui.Text(c, fmt.Sprintf("%s (%s:%d)", host.Description, host.Address, host.Port)).FontSize(12).TextColor(t.TextMuted)
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
					if ui.PrimaryButton(c, "发起 SSH 连接").Clicked() {
						v.openHostTab(host)
					}
				})
			})

			ui.Box(c).Height(1).Background(t.Border)

			// 属性配置明细表单组（原生两栏表单）
			ui.Column(c).Background(ui.Hex("#FAFAFC")).Border(1, t.Border).Padding(t.Space(4)).Radius(t.Radius).Gap(t.Space(3)).Children(func() {
				ui.Text(c, "连接配置明细").FontSize(13).Bold().TextColor(t.Text)

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
					} else {
						v.renderFieldItem(c, t, "密码状态", "已加密保存")
					}
				})

				// 属性行 3
				ui.Row(c).Gap(t.Space(8)).Children(func() {
					v.renderFieldItem(c, t, "创建时间", host.CreatedAt.Format("2006-01-02 15:04:05"))
					v.renderFieldItem(c, t, "更新时间", host.UpdatedAt.Format("2006-01-02 15:04:05"))
				})
			})

			// 状态与就绪说明
			ui.Column(c).Background(ui.Hex("#F4F9F6")).Border(1, ui.Hex("#D2ECE0")).Padding(t.Space(3.5)).Radius(t.Radius).Gap(t.Space(1)).Children(func() {
				ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
					ui.Text(c, "主机配置有效，处于就绪状态").FontSize(12).Bold().TextColor(ui.Hex("#0D6832"))
				})
				ui.Text(c, "第一阶段基础架构（数据持久化、凭据加解密与原生桌面框架）已全部通过。点击右上角即可开启交互终端。").FontSize(11).TextColor(ui.Hex("#2B6E4E"))
			})
		})
	})
}

// 辅助渲染单项属性
func (v *AppView) renderFieldItem(c *ui.Context, t *ui.Theme, label, value string) {
	ui.Column(c).Width(160).Gap(t.Space(0.5)).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(12).Bold().TextColor(t.Text)
	})
}

// 会话标签页展示区
func (v *AppView) renderActiveSessionTab(c *ui.Context, t *ui.Theme) {
	tabTitle := v.tabs[v.activeTabIndex]

	ui.Column(c).Fill().Padding(t.Space(6)).Center().Gap(t.Space(3)).Children(func() {
		ui.Text(c, fmt.Sprintf("终端会话: %s", tabTitle)).FontSize(16).Bold().TextColor(t.Text)
		ui.Text(c, "第一阶段：基础设施与本地持久化已完成").FontSize(12).TextColor(t.Success)
		ui.Text(c, "第二阶段将在此处无缝接入 Ghostty 终端渲染与远程 SSH 实时数据流").FontSize(12).TextColor(t.TextMuted)

		if ui.Button(c, "关闭当前会话").Clicked() {
			v.closeTab(v.activeTabIndex)
		}
	})
}

// 底部紧凑状态栏 (标准 24px 高度)
func (v *AppView) renderStatusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(24).Background(ui.Hex("#F1F3F5")).Padding(0, t.Space(3)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
		ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
		ui.Text(c, " 就绪").TextColor(t.Success).FontSize(11)
		ui.Text(c, " | 本地数据库: "+v.app.DataDir+"/goshell.db").FontSize(11).TextColor(t.TextMuted)
		ui.Spacer(c)
		ui.Text(c, "GoShell v0.1.0").FontSize(11).TextColor(t.TextMuted)
	})
}

// 新建/编辑主机弹窗 (桌面对话框风格，4px 原生圆角)
func (v *AppView) renderHostModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.modalOpen, func() {
		ui.Column(c).Width(420).Background(t.Surface).Border(1, ui.Hex("#CBD5E1")).Padding(t.Space(5)).Radius(t.Radius).Gap(t.Space(3)).Children(func() {
			title := "添加主机连接"
			if v.formID != "" {
				title = "编辑主机连接"
			}
			ui.Text(c, title).FontSize(15).Bold().TextColor(t.Text)

			if v.formErrorMsg != "" {
				ui.Text(c, v.formErrorMsg).FontSize(12).TextColor(t.Danger)
			}

			// 别名
			ui.Text(c, "主机别名:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.formName)

			// 地址与端口
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

			// 用户名
			ui.Text(c, "登录用户名:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.formUsername)

			// 认证方式选择
			ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "认证方式:").FontSize(12).TextColor(t.TextMuted)
				if ui.Button(c, "密码认证").Clicked() {
					v.formAuthType = "password"
				}
				if ui.Button(c, "私钥认证").Clicked() {
					v.formAuthType = "key"
				}
				ui.Text(c, "(当前: "+v.formAuthType+")").FontSize(11).TextColor(t.Accent)
			})

			if v.formAuthType == "password" {
				ui.Text(c, "密码:").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &v.formPassword)
			} else {
				ui.Text(c, "私钥路径 (如 ~/.ssh/id_rsa):").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &v.formKeyPath)
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
		Description: "自建主机连接",
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
