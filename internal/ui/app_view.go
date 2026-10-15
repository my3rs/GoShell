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
		theme:        ModernLightTheme(),
		sidebarWidth: 260,
		tabs:         []string{"Overview"},
		formPort:     "22",
		formUsername: "root",
		formAuthType: "password",
	}
}

// View 每一帧渲染入口（GPU 原生自绘）
func (v *AppView) View(c *ui.Context) {
	c.SetTheme(v.theme)
	t := c.Theme()

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// 1. 顶部无缝控制栏
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

// 顶部现代化控制栏 (macOS / Linear 浅色风格，纯文字与徽标，无 emoji)
func (v *AppView) renderTopBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(46).Background(t.Surface).Padding(t.Space(2), t.Space(4)).Gap(t.Space(3)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
		// 应用品牌文本
		ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "GoShell").FontSize(15).Bold().TextColor(t.Text)
			ui.Row(c).Background(ui.Hex("#F1F5F9")).Border(1, t.Border).Padding(t.Space(0.5), t.Space(1.5)).Radius(4).Children(func() {
				ui.Text(c, "v0.1.0").FontSize(10).TextColor(t.TextMuted)
			})
		})

		ui.Text(c, "/").TextColor(ui.Hex("#CBD5E1")).FontSize(13)

		// 新建主机操作
		if ui.PrimaryButton(c, "New Host").Clicked() {
			v.openCreateModal()
		}

		// 搜索过滤框
		ui.Row(c).Background(ui.Hex("#F8FAFC")).Border(1, t.Border).Padding(t.Space(1), t.Space(2)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "Search:").FontSize(11).TextColor(t.TextMuted)
			ui.TextInput(c, &v.searchQuery).Width(160)
		})

		ui.Spacer(c)

		// 右侧统计与运行环境状态
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Background(ui.Hex("#F8FAFC")).Border(1, t.Border).Padding(t.Space(1), t.Space(2.5)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
				ui.Text(c, fmt.Sprintf("%d Hosts", len(v.app.Hosts))).FontSize(11).TextColor(t.Text)
			})

			ui.Row(c).Background(ui.Hex("#EFF6FF")).Border(1, ui.Hex("#BFDBFE")).Padding(t.Space(1), t.Space(2.5)).Radius(t.Radius).Children(func() {
				ui.Text(c, "Native GPU Engine").FontSize(11).TextColor(ui.Hex("#1D4ED8"))
			})
		})
	})
}

// 左侧边栏 (资产与分组树)
func (v *AppView) renderSidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Background(ui.Hex("#F8FAFC")).Padding(t.Space(3)).Gap(t.Space(2.5)).Border(1, t.Border).Children(func() {
		// 边栏标题与刷新
		ui.Row(c).Padding(t.Space(1), t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "HOSTS").FontSize(11).Bold().TextColor(ui.Hex("#94A3B8"))
			ui.Spacer(c)
			if ui.Button(c, "Refresh").Clicked() {
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

					// 分组标题
					ui.Row(c).Padding(t.Space(1), t.Space(2)).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
						ui.Text(c, strings.ToUpper(group.Name)).FontSize(11).Bold().TextColor(t.TextMuted)
						ui.Spacer(c)
						ui.Text(c, fmt.Sprintf("%d", len(groupHosts))).FontSize(11).TextColor(ui.Hex("#94A3B8"))
					})

					// 该组下的主机列表项
					for _, h := range groupHosts {
						host := h
						isSelected := v.selectedHostID == host.ID

						cardBg := t.Surface
						borderCol := t.Border
						if isSelected {
							cardBg = ui.Hex("#EFF6FF")
							borderCol = ui.Hex("#93C5FD")
						}

						ui.Row(c).Background(cardBg).Border(1, borderCol).Padding(t.Space(2.5), t.Space(3)).Radius(t.Radius).AlignItems(ui.Center).Gap(t.Space(2.5)).Children(func() {
							// 状态绿点 (纯矢量自绘圆点)
							ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)

							// 主机名与连接参数
							ui.Column(c).Grow(1).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, host.Name).FontSize(13).Bold().TextColor(t.Text)
								ui.Text(c, fmt.Sprintf("%s@%s:%d", host.Username, host.Address, host.Port)).FontSize(11).TextColor(t.TextMuted)
							})

							// 连接按钮
							if ui.Button(c, "Connect").Clicked() {
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
		// 现代化多标签栏 (Safari / Linear 浅色标签风格)
		if len(v.tabs) > 0 {
			ui.Row(c).Height(38).Background(ui.Hex("#F1F5F9")).Padding(t.Space(1), t.Space(3)).Gap(t.Space(1)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
				for i, tab := range v.tabs {
					index := i
					title := tab
					isActive := v.activeTabIndex == index

					tabBg := ui.Hex("#F1F5F9")
					textColor := t.TextMuted
					borderCol := ui.Hex("#F1F5F9")
					if isActive {
						tabBg = t.Surface
						textColor = t.Text
						borderCol = t.Border
					}

					ui.Row(c).Background(tabBg).Border(1, borderCol).Radius(t.Radius).Padding(t.Space(1.5), t.Space(3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						ui.Text(c, title).FontSize(12).TextColor(textColor).Bold()

						// 非首个标签允许关闭
						if index > 0 {
							if ui.Button(c, "x").Clicked() {
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

// 概览面板 (纯白极简卡片，高对比度，清晰排版)
func (v *AppView) renderWelcomeDashboard(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Fill().Padding(t.Space(7)).Children(func() {
		ui.Column(c).Gap(t.Space(6)).Children(func() {
			// 标题区
			ui.Column(c).Gap(t.Space(1.5)).Children(func() {
				ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "GoShell Console").FontSize(22).Bold().TextColor(t.Text)
					ui.Row(c).Background(ui.Hex("#EFF6FF")).Border(1, ui.Hex("#BFDBFE")).Padding(t.Space(0.5), t.Space(2)).Radius(4).Children(func() {
						ui.Text(c, "Ready").FontSize(11).TextColor(ui.Hex("#1D4ED8"))
					})
				})
				ui.Text(c, "Pure Go native desktop client for SSH terminal and remote file management.").FontSize(13).TextColor(t.TextMuted)
			})

			// 核心功能卡片
			ui.Row(c).Gap(t.Space(4)).Children(func() {
				// 卡片 1
				ui.Column(c).Background(t.Surface).Border(1, t.Border).Padding(t.Space(5)).Radius(t.Radius).Width(240).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "Instant Terminal").FontSize(14).Bold().TextColor(t.Text)
					ui.Text(c, "Hardware-accelerated terminal via libghostty-vt with tab management and split panes.").FontSize(12).TextColor(t.TextMuted)
					if ui.PrimaryButton(c, "Add New Host").Clicked() {
						v.openCreateModal()
					}
				})

				// 卡片 2
				ui.Column(c).Background(t.Surface).Border(1, t.Border).Padding(t.Space(5)).Radius(t.Radius).Width(240).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "Encrypted Vault").FontSize(14).Bold().TextColor(t.Text)
					ui.Text(c, "Host credentials and private keys are encrypted locally using AES-256-GCM in SQLite.").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "Encrypted on disk").FontSize(11).TextColor(t.Success)
				})

				// 卡片 3
				ui.Column(c).Background(t.Surface).Border(1, t.Border).Padding(t.Space(5)).Radius(t.Radius).Width(240).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "Remote SFTP & Stats").FontSize(14).Bold().TextColor(t.Text)
					ui.Text(c, "Integrated file explorer and server metrics chart will be loaded in Phase 2.").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "Upcoming in Phase 2").FontSize(11).TextColor(t.Warning)
				})
			})

			// 选中主机详情卡片
			if v.selectedHostID != "" {
				var host *model.Host
				for _, h := range v.app.Hosts {
					if h.ID == v.selectedHostID {
						host = h
						break
					}
				}

				if host != nil {
					ui.Column(c).Background(ui.Hex("#F8FAFC")).Border(1, ui.Hex("#CBD5E1")).Padding(t.Space(5)).Radius(t.Radius).Gap(t.Space(3.5)).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
								ui.Text(c, host.Name).FontSize(16).Bold().TextColor(t.Text)
							})
							ui.Spacer(c)
							if ui.Button(c, "Edit").Clicked() {
								v.openEditModal(host)
							}
							if ui.Button(c, "Delete").Clicked() {
								_ = v.app.Store.DeleteHost(host.ID)
								v.selectedHostID = ""
								_ = v.app.ReloadData()
							}
						})

						ui.Row(c).Gap(t.Space(6)).Children(func() {
							ui.Column(c).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, "Host Address").FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, fmt.Sprintf("%s:%d", host.Address, host.Port)).FontSize(13).Bold().TextColor(t.Text)
							})
							ui.Column(c).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, "Username").FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, host.Username).FontSize(13).Bold().TextColor(t.Text)
							})
							ui.Column(c).Gap(t.Space(0.5)).Children(func() {
								ui.Text(c, "Auth Method").FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, string(host.AuthType)).FontSize(13).Bold().TextColor(t.Text)
							})
						})

						if host.Description != "" {
							ui.Text(c, "Note: "+host.Description).FontSize(12).TextColor(t.TextMuted)
						}

						ui.Row(c).Gap(t.Space(3)).Children(func() {
							if ui.PrimaryButton(c, "Open SSH Session").Clicked() {
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

	ui.Column(c).Fill().Padding(t.Space(8)).Center().Gap(t.Space(3)).Children(func() {
		ui.Text(c, fmt.Sprintf("Session Ready: %s", tabTitle)).FontSize(18).Bold().TextColor(t.Text)
		ui.Text(c, "Phase 1: Foundation and local SQLite persistence completed.").FontSize(13).TextColor(t.Success)
		ui.Text(c, "Phase 2 will mount the native Ghostty VT terminal and SSH stream right here.").FontSize(13).TextColor(t.TextMuted)

		if ui.Button(c, "Close Tab").Clicked() {
			v.closeTab(v.activeTabIndex)
		}
	})
}

// 底部极简状态栏
func (v *AppView) renderStatusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Height(26).Background(ui.Hex("#F8FAFC")).Padding(t.Space(1), t.Space(4)).AlignItems(ui.Center).Border(1, t.Border).Children(func() {
		ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success)
		ui.Text(c, " Ready").TextColor(t.Success).FontSize(11)
		ui.Text(c, " | SQLite: "+v.app.DataDir+"/goshell.db").FontSize(11).TextColor(t.TextMuted)
		ui.Spacer(c)
		ui.Text(c, "GoShell v0.1.0").FontSize(11).TextColor(t.TextMuted)
	})
}

// 新建/编辑主机弹窗 (纯白卡片式，清晰现代)
func (v *AppView) renderHostModal(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &v.modalOpen, func() {
		ui.Column(c).Width(440).Background(t.Surface).Border(1, ui.Hex("#CBD5E1")).Padding(t.Space(6)).Radius(t.Radius).Gap(t.Space(3.5)).Children(func() {
			title := "Add New Host"
			if v.formID != "" {
				title = "Edit Host Configuration"
			}
			ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)

			if v.formErrorMsg != "" {
				ui.Text(c, v.formErrorMsg).FontSize(12).TextColor(t.Danger)
			}

			// 字段：名称
			ui.Text(c, "Alias / Name:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.formName)

			// 字段：主机与端口
			ui.Row(c).Gap(t.Space(2)).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "Address / IP:").FontSize(12).TextColor(t.TextMuted)
					ui.TextInput(c, &v.formAddress)
				})
				ui.Column(c).Width(90).Children(func() {
					ui.Text(c, "Port:").FontSize(12).TextColor(t.TextMuted)
					ui.TextInput(c, &v.formPort)
				})
			})

			// 字段：用户名
			ui.Text(c, "Username:").FontSize(12).TextColor(t.TextMuted)
			ui.TextInput(c, &v.formUsername)

			// 认证方式选择
			ui.Row(c).Gap(t.Space(3)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Auth Method:").FontSize(12).TextColor(t.TextMuted)
				if ui.Button(c, "Password").Clicked() {
					v.formAuthType = "password"
				}
				if ui.Button(c, "Private Key").Clicked() {
					v.formAuthType = "key"
				}
				ui.Row(c).Background(ui.Hex("#EFF6FF")).Border(1, ui.Hex("#BFDBFE")).Padding(t.Space(0.5), t.Space(2)).Radius(4).Children(func() {
					ui.Text(c, "Active: "+v.formAuthType).FontSize(11).TextColor(ui.Hex("#1D4ED8"))
				})
			})

			if v.formAuthType == "password" {
				ui.Text(c, "Password:").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &v.formPassword)
			} else {
				ui.Text(c, "Key Path (e.g. ~/.ssh/id_rsa):").FontSize(12).TextColor(t.TextMuted)
				ui.TextInput(c, &v.formKeyPath)
			}

			// 底部操作按钮
			ui.Row(c).Gap(t.Space(2.5)).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, "Cancel").Clicked() {
					v.modalOpen = false
				}
				if ui.PrimaryButton(c, "Save Host").Clicked() {
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
		v.formErrorMsg = "Please enter a host alias"
		return
	}
	if strings.TrimSpace(v.formAddress) == "" {
		v.formErrorMsg = "Please enter an IP address or hostname"
		return
	}
	port, err := strconv.Atoi(strings.TrimSpace(v.formPort))
	if err != nil || port <= 0 {
		v.formErrorMsg = "Please enter a valid port number (1-65535)"
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
		Description: "User host entry",
		ColorTag:    "#10B981",
	}

	if err := v.app.Store.SaveHost(h); err != nil {
		v.formErrorMsg = fmt.Sprintf("Save failed: %v", err)
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
