package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"goshell/internal/app"
	"goshell/internal/core/model"
)

func TestWorkSummary_HasActiveWork(t *testing.T) {
	// 空任务汇总
	emptyWS := WorkSummary{}
	if emptyWS.HasActiveWork() {
		t.Errorf("Empty WorkSummary should not have active work")
	}

	// 仅有活跃传输任务
	wsTransfer := WorkSummary{ActiveTransfers: 1, TransferSpeed: "500 KB/s"}
	if !wsTransfer.HasActiveWork() {
		t.Errorf("WorkSummary with ActiveTransfers should report HasActiveWork == true")
	}

	// 仅有未保存文件
	wsFiles := WorkSummary{UnsavedFiles: []string{"config.yaml"}}
	if !wsFiles.HasActiveWork() {
		t.Errorf("WorkSummary with UnsavedFiles should report HasActiveWork == true")
	}
}

func TestWorkSummary_BuildConfirmMessage(t *testing.T) {
	ws := WorkSummary{
		ActiveTransfers: 2,
		TransferSpeed:   "1.5 MB/s",
		UnsavedFiles:    []string{"docker-compose.yml"},
	}

	msg, detail := ws.BuildConfirmMessage()
	if !strings.Contains(msg, "仍有未完成的传输任务或未保存的文件") {
		t.Errorf("Expected message to mention ongoing transfers or unsaved files, got: %s", msg)
	}

	if !strings.Contains(detail, "2 个文件传输任务") || !strings.Contains(detail, "1.5 MB/s") {
		t.Errorf("Detail should mention active transfers and speed, got: %s", detail)
	}

	if !strings.Contains(detail, "docker-compose.yml") {
		t.Errorf("Detail should list unsaved files, got: %s", detail)
	}
}

func TestAppView_GetWorkSummary(t *testing.T) {
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{
				Type:  TabTypeSession,
				Title: "prod-server",
				Session: &TerminalSession{
					Host: &model.Host{
						Name:     "prod-server",
						AuthType: model.AuthTypePassword,
					},
					Connected: true, // 即使处于连接状态，只要没有传输任务，也不拦截
				},
			},
			{
				Type:  TabTypeEditor,
				Title: "script.sh",
				Editor: &EditorFile{
					Name:   "script.sh",
					Buffer: editor.NewBuffer("echo hello\n"),
				},
			},
		},
	}

	// 初始状态：无文件传输任务，编辑器文件未修改 -> HasActiveWork 应该为 false
	ws := v.GetWorkSummary()
	if ws.HasActiveWork() {
		t.Errorf("Expected HasActiveWork == false when only idle connected session exists")
	}
	if len(ws.UnsavedFiles) != 0 {
		t.Errorf("Expected 0 unsaved files, got: %d", len(ws.UnsavedFiles))
	}

	// 修改编辑器文件 -> 触发守护
	v.tabs[2].Editor.Buffer.Set("echo modified\n")
	wsModified := v.GetWorkSummary()
	if !wsModified.HasActiveWork() {
		t.Errorf("Expected HasActiveWork == true after modifying editor file")
	}
	if len(wsModified.UnsavedFiles) != 1 || wsModified.UnsavedFiles[0] != "script.sh" {
		t.Errorf("Expected unsaved file 'script.sh', got: %v", wsModified.UnsavedFiles)
	}

	// 当无活动任务与未保存文件时，ConfirmQuit 直接放行
	vIdle := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
		},
	}
	if !vIdle.ConfirmQuit() {
		t.Errorf("ConfirmQuit should return true for idle state")
	}

	// 当已确认退出时，直接返回 true
	vConfirmed := &AppView{
		quitConfirmed: true,
	}
	if !vConfirmed.ConfirmQuit() || !vConfirmed.ConfirmClose() {
		t.Errorf("ConfirmQuit and ConfirmClose should return true when quitConfirmed is true")
	}
}

func TestAppView_LayoutSwitching(t *testing.T) {
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Host-1", Session: &TerminalSession{}},
			{Type: TabTypeSession, Title: "Host-2", Session: &TerminalSession{}},
			{Type: TabTypeSession, Title: "Host-3", Session: &TerminalSession{}},
			{Type: TabTypeSession, Title: "Host-4", Session: &TerminalSession{}},
		},
		activeTabIndex:  1,
		layoutMode:      LayoutSingle,
		paneTabIndices:  [4]int{1, 2, 3, 4},
		activePaneIndex: 0,
	}

	// 1. 初始为单视窗布局
	if v.getPaneCount() != 1 {
		t.Fatalf("Expected 1 pane for LayoutSingle, got %d", v.getPaneCount())
	}
	if v.getTabPaneNum(1) != 0 { // LayoutSingle 下不展示 P 角标
		t.Errorf("Expected Tab 1 to return 0 in single mode, got %d", v.getTabPaneNum(1))
	}

	// 2. 一键切换到左右双 Tab 并列布局
	v.SetLayoutMode(LayoutDualSplit)
	if v.layoutMode != LayoutDualSplit {
		t.Fatalf("Expected layoutMode to be LayoutDualSplit")
	}
	if v.getPaneCount() != 2 {
		t.Fatalf("Expected 2 panes for LayoutDualSplit, got %d", v.getPaneCount())
	}
	// Pane 0 应该为当前激活的终端 Tab 1
	if v.paneTabIndices[0] != 1 {
		t.Errorf("Expected Pane 0 to be Tab 1, got %d", v.paneTabIndices[0])
	}
	// Pane 1 应该自动分配不同于 Pane 0 的有效终端 Tab，且绝不能是概览 (0)
	if v.paneTabIndices[1] == v.paneTabIndices[0] || v.paneTabIndices[1] <= 0 || v.paneTabIndices[1] >= len(v.tabs) {
		t.Errorf("Expected Pane 1 to be a distinct valid terminal tab (>0), got %d", v.paneTabIndices[1])
	}

	// 3. 一键切换到四宫格 (2x2) 布局
	v.SetLayoutMode(LayoutQuadGrid)
	if v.layoutMode != LayoutQuadGrid {
		t.Fatalf("Expected layoutMode to be LayoutQuadGrid")
	}
	if v.getPaneCount() != 4 {
		t.Fatalf("Expected 4 panes for LayoutQuadGrid, got %d", v.getPaneCount())
	}
	// 验证 4 个窗格的 Tab 索引均为非概览终端 (>=1)，互不相同且均在有效范围内
	assignedSet := make(map[int]bool)
	for p := 0; p < 4; p++ {
		idx := v.paneTabIndices[p]
		if idx <= 0 || idx >= len(v.tabs) {
			t.Errorf("Pane %d has invalid terminal tab index %d (must be >= 1)", p, idx)
		}
		if assignedSet[idx] {
			t.Errorf("Duplicate tab index %d assigned to pane %d", idx, p)
		}
		assignedSet[idx] = true
	}

	// 4. 一键切回单视窗
	v.SetLayoutMode(LayoutSingle)
	if v.layoutMode != LayoutSingle {
		t.Fatalf("Expected layoutMode to return to LayoutSingle")
	}
	if v.getPaneCount() != 1 {
		t.Errorf("Expected 1 pane after returning to LayoutSingle")
	}
}

func TestAppView_PaneTabSwitchingAndSwap(t *testing.T) {
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Host-1", Session: &TerminalSession{}},
			{Type: TabTypeSession, Title: "Host-2", Session: &TerminalSession{}},
			{Type: TabTypeSession, Title: "Host-3", Session: &TerminalSession{}},
		},
		activeTabIndex:  1,
		layoutMode:      LayoutDualSplit,
		activePaneIndex: 0,
		paneTabIndices:  [4]int{1, 2, -1, -1},
	}

	// 当前 Pane 0 显示 Tab 1 (标"左")，Pane 1 显示 Tab 2 (标"右")
	if v.paneTabIndices[0] != 1 || v.paneTabIndices[1] != 2 {
		t.Fatalf("Unexpected initial pane indices: %v", v.paneTabIndices)
	}
	lbl0, p0 := v.getTabPanePositionLabel(1)
	if lbl0 != "左" || p0 != 0 {
		t.Errorf("Expected Tab 1 label '左' and pane 0, got %q, %d", lbl0, p0)
	}
	lbl1, p1 := v.getTabPanePositionLabel(2)
	if lbl1 != "右" || p1 != 1 {
		t.Errorf("Expected Tab 2 label '右' and pane 1, got %q, %d", lbl1, p1)
	}

	// 1. 方案 A：点击已经在 Pane 1 平铺展示的 Tab 2
	// 预期行为：直接将输入焦点切换到 Pane 1，两个窗格的内容保持不变
	v.switchActivePaneToTab(2)
	if v.activePaneIndex != 1 {
		t.Errorf("Expected activePaneIndex to switch to 1, got %d", v.activePaneIndex)
	}
	if v.activeTabIndex != 2 {
		t.Errorf("Expected activeTabIndex to be 2, got %d", v.activeTabIndex)
	}
	if v.paneTabIndices[0] != 1 || v.paneTabIndices[1] != 2 {
		t.Errorf("Pane assignments should remain unchanged: %v", v.paneTabIndices)
	}

	// 2. 聚焦在 Pane 1 时，点击未被展示的后台 Tab 3
	// 预期行为：将未平铺的 Tab 3 载入到当前聚焦窗格 Pane 1 中
	v.switchActivePaneToTab(3)
	if v.paneTabIndices[1] != 3 {
		t.Errorf("Expected Pane 1 to display Tab 3, got %d", v.paneTabIndices[1])
	}
	if v.paneTabIndices[0] != 1 {
		t.Errorf("Pane 0 should remain Tab 1, got %d", v.paneTabIndices[0])
	}
	lbl3, p3 := v.getTabPanePositionLabel(3)
	if lbl3 != "右" || p3 != 1 {
		t.Errorf("Expected Tab 3 label '右', got %q", lbl3)
	}

	// 3. 尝试切换到概览 Tab 0：概览页独立全屏查看，绝不修改窗格内的终端绑定，且位置标签为空
	v.switchActivePaneToTab(0)
	if v.activeTabIndex != 0 {
		t.Errorf("activeTabIndex should be 0 (Overview)")
	}
	if v.paneTabIndices[0] == 0 || v.paneTabIndices[1] == 0 {
		t.Errorf("Overview Tab 0 must never be assigned to panes: %v", v.paneTabIndices)
	}
	lblOverview, _ := v.getTabPanePositionLabel(0)
	if lblOverview != "" {
		t.Errorf("Overview Tab 0 should never have a position label, got %q", lblOverview)
	}
}

func TestAppView_LayoutOnTabRemoved(t *testing.T) {
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Host-1", Session: &TerminalSession{}},
			{Type: TabTypeSession, Title: "Host-2", Session: &TerminalSession{}},
		},
		activeTabIndex:  2,
		layoutMode:      LayoutDualSplit,
		activePaneIndex: 1,
		paneTabIndices:  [4]int{1, 2, -1, -1},
	}

	// 移除 Tab 1 (Host-1)，此时原 Tab 2 变为索引 1
	v.tabs = []WorkspaceTab{v.tabs[0], v.tabs[2]}
	v.onTabRemoved(1)

	// 窗格中的索引必须全部小于 len(v.tabs)，且绝对不能为 0
	for p := 0; p < 2; p++ {
		idx := v.paneTabIndices[p]
		if idx == 0 {
			t.Errorf("Pane %d assigned overview tab 0!", p)
		}
		if idx >= len(v.tabs) {
			t.Errorf("Pane %d has out of range tab index %d (tabs len: %d)", p, idx, len(v.tabs))
		}
	}
}

func TestAppView_EmptyPaneHandling(t *testing.T) {
	// 仅有 1 个概览 Tab + 1 个终端 Tab 的情况下切换四宫格
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Host-1", Session: &TerminalSession{}},
		},
		activeTabIndex:  1,
		layoutMode:      LayoutSingle,
		paneTabIndices:  [4]int{1, -1, -1, -1},
		activePaneIndex: 0,
	}

	v.SetLayoutMode(LayoutQuadGrid)
	if v.layoutMode != LayoutQuadGrid {
		t.Fatalf("Expected LayoutQuadGrid")
	}

	// Pane 0 绑定 Tab 1，其余窗格由于无更多可用终端，应标记为 -1 (空窗格卡片)，绝对不能分配 Tab 0
	if v.paneTabIndices[0] != 1 {
		t.Errorf("Expected Pane 0 to be Tab 1, got %d", v.paneTabIndices[0])
	}
	for p := 1; p < 4; p++ {
		if v.paneTabIndices[p] != -1 {
			t.Errorf("Expected Pane %d to be -1 (empty pane), got %d", p, v.paneTabIndices[p])
		}
	}
}

func TestAppView_ActivePaneSessionForFiles(t *testing.T) {
	s1 := &TerminalSession{Host: &model.Host{ID: "h1", Name: "Host-1", Address: "10.0.0.1"}}
	s2 := &TerminalSession{Host: &model.Host{ID: "h2", Name: "Host-2", Address: "10.0.0.2"}}
	s3 := &TerminalSession{Host: &model.Host{ID: "h3", Name: "Host-3", Address: "10.0.0.3"}}

	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Host-1", Session: s1},
			{Type: TabTypeSession, Title: "Host-2", Session: s2},
			{Type: TabTypeSession, Title: "Host-3", Session: s3},
		},
		activeTabIndex:  1,
		layoutMode:      LayoutDualSplit,
		activePaneIndex: 0,
		paneTabIndices:  [4]int{1, 2, -1, -1},
	}

	// 1. 左右双列模式下，当前 active 窗格为 Pane 0 (Tab 1, Host-1)
	// 预期：左侧文件列表获取到的 activeSession 必须是处于 active 格的 Host-1
	sess := v.getActiveSession()
	if sess != s1 {
		t.Fatalf("Expected active session to be s1 (Host-1), got %v", sess)
	}

	// 2. 点击/聚焦切换到 Pane 1 (Tab 2, Host-2)
	// 预期：左侧文件列表获取到的 activeSession 必须自动切换为处于 active 格的 Host-2
	v.setActivePane(1)
	if v.activePaneIndex != 1 {
		t.Errorf("Expected activePaneIndex to be 1, got %d", v.activePaneIndex)
	}
	if v.activeTabIndex != 2 {
		t.Errorf("Expected activeTabIndex to be 2, got %d", v.activeTabIndex)
	}
	sess = v.getActiveSession()
	if sess != s2 {
		t.Fatalf("Expected active session to be s2 (Host-2) when Pane 1 is active, got %v", sess)
	}

	// 3. 切换至四宫格布局，并切换 active 格到 Pane 2 (Tab 3, Host-3)
	// 预期：左侧文件列表自动切换为 Host-3
	v.SetLayoutMode(LayoutQuadGrid)
	v.paneTabIndices = [4]int{1, 2, 3, -1}
	v.setActivePane(2)
	sess = v.getActiveSession()
	if sess != s3 {
		t.Fatalf("Expected active session to be s3 (Host-3) when Pane 2 is active, got %v", sess)
	}

	// 4. 当用户切换回概览页 Tab 0 (全屏概览)
	// 概览页自身无主机，应保持正常无崩溃
	v.switchActivePaneToTab(0)
	if v.activeTabIndex != 0 {
		t.Errorf("Expected activeTabIndex to be 0")
	}
}

func TestAppView_ToggleSidebar(t *testing.T) {
	v := &AppView{
		sidebarWidth:     240,
		sidebarCollapsed: false,
	}

	// 1. 默认展开状态
	if v.sidebarCollapsed {
		t.Errorf("Expected sidebar to be expanded initially")
	}

	// 2. 触发折叠
	v.ToggleSidebar()
	if !v.sidebarCollapsed {
		t.Errorf("Expected sidebar to be collapsed after ToggleSidebar")
	}

	// 3. 再次触发展开，恢复宽度
	v.sidebarWidth = 100 // 模拟异常或极小宽度
	v.ToggleSidebar()
	if v.sidebarCollapsed {
		t.Errorf("Expected sidebar to be expanded after second ToggleSidebar")
	}
	if v.sidebarWidth < 180 {
		t.Errorf("Expected sidebarWidth to be restored to at least 180, got %f", v.sidebarWidth)
	}
}

func TestAppView_AboutModal(t *testing.T) {
	v := &AppView{
		aboutModalOpen: false,
	}

	if v.aboutModalOpen {
		t.Errorf("aboutModalOpen should be false initially")
	}

	v.aboutModalOpen = true
	if !v.aboutModalOpen {
		t.Errorf("aboutModalOpen should be true after triggering")
	}

	v.aboutModalOpen = false
	if v.aboutModalOpen {
		t.Errorf("aboutModalOpen should be false after closing")
	}
}

func TestAppView_SwitchActivePaneToTab(t *testing.T) {
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Server-1"},
			{Type: TabTypeSession, Title: "Server-2"},
		},
		activeTabIndex: 0,
		layoutMode:     LayoutSingle,
	}

	// 切换到 Tab 1
	v.switchActivePaneToTab(1)
	if v.activeTabIndex != 1 {
		t.Errorf("Expected activeTabIndex to be 1, got %d", v.activeTabIndex)
	}
	if v.paneTabIndices[0] != 1 {
		t.Errorf("Expected paneTabIndices[0] to be 1, got %d", v.paneTabIndices[0])
	}

	// 切换到 Tab 2
	v.switchActivePaneToTab(2)
	if v.activeTabIndex != 2 {
		t.Errorf("Expected activeTabIndex to be 2, got %d", v.activeTabIndex)
	}
	if v.paneTabIndices[0] != 2 {
		t.Errorf("Expected paneTabIndices[0] to be 2, got %d", v.paneTabIndices[0])
	}

	// 切换回概览页 Tab 0
	v.switchActivePaneToTab(0)
	if v.activeTabIndex != 0 {
		t.Errorf("Expected activeTabIndex to be 0, got %d", v.activeTabIndex)
	}
}

func TestAppView_TabAnimationState(t *testing.T) {
	v := &AppView{
		app: &app.App{
			Hosts: []*model.Host{
				{ID: "host-1", Name: "Server 1", Address: "1.1.1.1", Port: 22},
			},
		},
		theme:          DockerMateTheme(),
		sidebarTab:     0,
		activeTabIndex: 0,
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Server 1"},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		v.View(c)
	}, 800, 600)

	// 第 1 帧渲染
	tt.Frame()

	// 第 2 帧渲染，布局边界已稳定，tabPillReady 应当为 true，且已正确捕获激活 Tab 尺寸
	tt.Frame()
	if !v.tabPillReady {
		t.Errorf("Expected tabPillReady to be true after initial frames render")
	}
	if v.lastTabRect.W <= 0 {
		t.Errorf("Expected lastTabRect.W > 0, got %f", v.lastTabRect.W)
	}

	// 验证侧边栏分段 Tab 切换
	v.sidebarTab = 1
	tt.Frame()
	if v.sidebarTab != 1 {
		t.Errorf("Expected sidebarTab to be 1, got %d", v.sidebarTab)
	}

	// 验证工作区 Tab 切换
	v.switchActivePaneToTab(1)
	tt.Frame()
	if v.activeTabIndex != 1 {
		t.Errorf("Expected activeTabIndex to be 1, got %d", v.activeTabIndex)
	}
	if v.lastTabRect.W <= 0 {
		t.Errorf("Expected lastTabRect.W > 0 after tab switch, got %f", v.lastTabRect.W)
	}
}

func TestAppView_QuickConnect(t *testing.T) {
	v := &AppView{
		app: &app.App{
			Hosts: []*model.Host{},
		},
		theme:          DockerMateTheme(),
		tabs:           []WorkspaceTab{{Type: TabTypeOverview, Title: "概览"}},
		paneTabIndices: [4]int{0, 1, 2, 3},
	}

	// 1. 测试打开快速连接弹窗
	v.openQuickConnectModal()
	if !v.quickConnectModalOpen {
		t.Errorf("Expected quickConnectModalOpen = true")
	}
	if v.quickConnectPort != "22" || v.quickConnectUser != "root" {
		t.Errorf("Default port/user mismatch: port=%s, user=%s", v.quickConnectPort, v.quickConnectUser)
	}

	// 2. 测试输入解析
	v.quickConnectTarget = "ssh -p 2222 admin@192.168.1.50"
	v.syncQuickConnectFieldsFromTarget()
	if v.quickConnectHost != "192.168.1.50" {
		t.Errorf("quickConnectHost = %q, want 192.168.1.50", v.quickConnectHost)
	}
	if v.quickConnectPort != "2222" {
		t.Errorf("quickConnectPort = %q, want 2222", v.quickConnectPort)
	}
	if v.quickConnectUser != "admin" {
		t.Errorf("quickConnectUser = %q, want admin", v.quickConnectUser)
	}

	// 3. 测试提交快速连接
	v.quickConnectPassword = "mypassword"
	v.submitQuickConnect()

	if v.quickConnectModalOpen {
		t.Errorf("Expected quickConnectModalOpen = false after submit")
	}
	if len(v.tabs) != 2 {
		t.Fatalf("Expected 2 tabs after quick connect, got %d", len(v.tabs))
	}

	tab := v.tabs[1]
	if tab.Type != TabTypeSession || tab.Session == nil {
		t.Fatalf("Expected tab to be a session tab")
	}
	if !tab.Session.Host.IsEphemeral {
		t.Errorf("Expected Host.IsEphemeral = true")
	}
	if tab.Session.Host.Address != "192.168.1.50" {
		t.Errorf("Host.Address = %q, want 192.168.1.50", tab.Session.Host.Address)
	}
	if tab.Session.Host.Port != 2222 {
		t.Errorf("Host.Port = %d, want 2222", tab.Session.Host.Port)
	}
	if tab.Session.Host.Password != "mypassword" {
		t.Errorf("Host.Password = %q, want mypassword", tab.Session.Host.Password)
	}

	// 4. 测试基于已有 Host 打开快速连接弹窗
	preparsed := &model.Host{
		Address:  "example.com",
		Port:     8022,
		Username: "deploy",
	}
	v.openQuickConnectWithHost(preparsed)
	if !v.quickConnectModalOpen {
		t.Errorf("Expected quickConnectModalOpen = true")
	}
	if v.quickConnectHost != "example.com" || v.quickConnectPort != "8022" || v.quickConnectUser != "deploy" {
		t.Errorf("openQuickConnectWithHost mismatch: host=%s, port=%s, user=%s", v.quickConnectHost, v.quickConnectPort, v.quickConnectUser)
	}
}



