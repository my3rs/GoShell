package ui

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui/editor"

	sftpService "goshell/internal/core/sftp"

	"goshell/internal/core/model"
)

func TestEditor_LanguageDetection(t *testing.T) {
	testCases := []struct {
		filename string
		wantLang string
	}{
		{"main.go", "Go"},
		{"config.json", "JSON"},
		{"docker-compose.yml", "YAML"},
		{"deploy.yaml", "YAML"},
		{"setup.sh", "Shell"},
		{"script.py", "Python"},
		{"README.md", "Markdown"},
		{"schema.sql", "SQL"},
		{"Dockerfile", "Dockerfile"},
		{"Makefile", "Makefile"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			lang := editor.LanguageFor(tc.filename)
			if lang == nil {
				t.Fatalf("LanguageFor(%q) returned nil, expected %s", tc.filename, tc.wantLang)
			}
			if lang.Name != tc.wantLang {
				t.Errorf("LanguageFor(%q) = %q, want %q", tc.filename, lang.Name, tc.wantLang)
			}
		})
	}
}

func TestEditor_BufferOperations(t *testing.T) {
	code := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"
	buf := editor.NewBuffer(code)
	lang := editor.LanguageFor("main.go")
	buf.SetLanguage(lang)

	if buf.LineCount() != 6 {
		t.Errorf("LineCount = %d, want 6", buf.LineCount())
	}

	if buf.Modified() {
		t.Errorf("New buffer should not be modified yet")
	}

	buf.Set("package main\n\nfunc main() {\n\tprintln(\"updated\")\n}\n")
	if !buf.Modified() {
		t.Errorf("Buffer after edit should report Modified() == true")
	}

	buf.MarkSaved()
	if buf.Modified() {
		t.Errorf("Buffer after MarkSaved() should report Modified() == false")
	}
}

func TestEditor_BinaryDetection(t *testing.T) {
	textData := []byte("Hello, this is pure UTF-8 plain text file without null bytes.\nLine 2.\n")
	if bytes.IndexByte(textData, 0) != -1 {
		t.Errorf("Text data should not contain null bytes")
	}

	binaryData := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00, 0x00, 0x00}
	if bytes.IndexByte(binaryData, 0) == -1 {
		t.Errorf("Binary data should contain null bytes")
	}
}

func TestSession_HostColors(t *testing.T) {
	if len(HostColorPalette) < 6 {
		t.Errorf("HostColorPalette should have at least 6 colors, got %d", len(HostColorPalette))
	}

	s1 := &TerminalSession{ColorIndex: 0}
	s2 := &TerminalSession{ColorIndex: 1}

	if s1.HostColor().Accent == s2.HostColor().Accent {
		t.Errorf("Adjacent sessions should have different colors")
	}
}

func TestEditor_SaveShortcutByOS(t *testing.T) {
	testCases := []struct {
		goos        string
		wantKey     string
		wantBtnText string
	}{
		{"darwin", "Cmd+S", "保存 (Cmd+S)"},
		{"windows", "Ctrl+S", "保存 (Ctrl+S)"},
		{"linux", "Ctrl+S", "保存 (Ctrl+S)"},
	}

	for _, tc := range testCases {
		t.Run(tc.goos, func(t *testing.T) {
			if gotKey := SaveShortcutTextForOS(tc.goos); gotKey != tc.wantKey {
				t.Errorf("SaveShortcutTextForOS(%q) = %q, want %q", tc.goos, gotKey, tc.wantKey)
			}
			if gotBtn := SaveButtonLabelForOS(tc.goos); gotBtn != tc.wantBtnText {
				t.Errorf("SaveButtonLabelForOS(%q) = %q, want %q", tc.goos, gotBtn, tc.wantBtnText)
			}
		})
	}

	// 验证当前运行平台的默认函数返回
	currentShortcut := SaveShortcutText()
	currentBtnText := SaveButtonLabel()
	if currentShortcut == "" || currentBtnText == "" {
		t.Errorf("SaveShortcutText() or SaveButtonLabel() returned empty")
	}
}

func TestTerminal_OptionsConfig(t *testing.T) {
	opts := terminal.Options{
		Font: terminal.Font{
			NoBold: true,
		},
		CopyOnSelect: terminal.CopyOnSelectClipboard,
		MiddleClick:  terminal.MiddleClickPaste,
		Theme:        terminal.DarkTheme(),
		DarkTheme:    terminal.DarkTheme(),
	}

	if !opts.Font.NoBold {
		t.Errorf("Font.NoBold = false, want true")
	}
	if opts.CopyOnSelect != terminal.CopyOnSelectClipboard {
		t.Errorf("CopyOnSelect = %v, want CopyOnSelectClipboard", opts.CopyOnSelect)
	}
	if opts.MiddleClick != terminal.MiddleClickPaste {
		t.Errorf("MiddleClick = %v, want MiddleClickPaste", opts.MiddleClick)
	}
	if opts.Theme == nil || opts.Theme.Background != terminal.DarkTheme().Background {
		t.Errorf("Terminal theme background should be dark")
	}
	if ColTerminalBg != terminal.DarkTheme().Background {
		t.Errorf("ColTerminalBg = %v, want %v", ColTerminalBg, terminal.DarkTheme().Background)
	}
}

func TestSession_HiddenFilesFiltering(t *testing.T) {
	s := &TerminalSession{
		FileList: []sftpService.FileEntry{
			{Name: ".git", IsDir: true},
			{Name: ".gitignore", IsDir: false},
			{Name: ".env", IsDir: false},
			{Name: "src", IsDir: true},
			{Name: "README.md", IsDir: false},
		},
		ShowHidden: false,
	}

	// 1. 未启用显示隐藏文件时：仅显示常规文件，排除以 . 开头的文件
	vis1 := s.VisibleFiles()
	if len(vis1) != 2 {
		t.Fatalf("VisibleFiles(ShowHidden=false) returned %d entries, want 2", len(vis1))
	}
	if vis1[0].Name != "src" || vis1[1].Name != "README.md" {
		t.Errorf("Unexpected entries: %v", vis1)
	}

	// 2. 启用显示隐藏文件时：显示全部条目
	s.ShowHidden = true
	vis2 := s.VisibleFiles()
	if len(vis2) != 5 {
		t.Fatalf("VisibleFiles(ShowHidden=true) returned %d entries, want 5", len(vis2))
	}
}

func TestAppView_TabManagement(t *testing.T) {
	s1 := &TerminalSession{Title: "Host 1"}
	s2 := &TerminalSession{Title: "Host 2"}
	s3 := &TerminalSession{Title: "Host 3"}

	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: "Host 1", Session: s1},
			{Type: TabTypeEditor, Title: "file1.go", Session: s1},
			{Type: TabTypeSession, Title: "Host 2", Session: s2},
			{Type: TabTypeSession, Title: "Host 3", Session: s3},
		},
		activeTabIndex: 2,
	}

	// 1. 测试 closeTabsToRight(2)
	v.closeTabsToRight(2)
	if len(v.tabs) != 3 {
		t.Fatalf("closeTabsToRight(2) expected 3 tabs, got %d", len(v.tabs))
	}
	if v.tabs[0].Title != "概览" || v.tabs[1].Title != "Host 1" || v.tabs[2].Title != "file1.go" {
		t.Errorf("unexpected tabs after closeTabsToRight: %+v", v.tabs)
	}

	// 2. 重新填充标签并测试 closeOtherTabs(1)
	v.tabs = []WorkspaceTab{
		{Type: TabTypeOverview, Title: "概览"},
		{Type: TabTypeSession, Title: "Host 1", Session: s1},
		{Type: TabTypeEditor, Title: "file1.go", Session: s1},
	}
	v.activeTabIndex = 1
	v.closeOtherTabs(1)
	if len(v.tabs) != 2 {
		t.Fatalf("closeOtherTabs(1) expected 2 tabs (overview + target), got %d", len(v.tabs))
	}
	if v.tabs[0].Type != TabTypeOverview || v.tabs[1].Title != "Host 1" {
		t.Errorf("unexpected tabs after closeOtherTabs: %+v", v.tabs)
	}
	if v.activeTabIndex != 1 {
		t.Errorf("activeTabIndex expected 1, got %d", v.activeTabIndex)
	}
}

func TestSession_ShouldShowTransferQueue(t *testing.T) {
	// 本地终端：不显示传输队列
	localSession := &TerminalSession{
		Host: &model.Host{
			Name:     "本地终端",
			AuthType: model.AuthTypeLocal,
		},
	}
	if localSession.ShouldShowTransferQueue() {
		t.Errorf("localSession.ShouldShowTransferQueue() = true, want false")
	}

	// 远程 SSH 主机：显示传输队列
	remoteSession := &TerminalSession{
		Host: &model.Host{
			Name:     "生产服务器",
			Address:  "192.168.1.100",
			Port:     22,
			AuthType: model.AuthTypePassword,
		},
	}
	if !remoteSession.ShouldShowTransferQueue() {
		t.Errorf("remoteSession.ShouldShowTransferQueue() = false, want true")
	}

	// 空 Host 防御
	nilHostSession := &TerminalSession{Host: nil}
	if nilHostSession.ShouldShowTransferQueue() {
		t.Errorf("nilHostSession.ShouldShowTransferQueue() = true, want false")
	}
}

func TestAppView_MultipleTabsForSameHost(t *testing.T) {
	h := &model.Host{ID: "host-1", Name: "Ubuntu-Prod"}

	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
		},
		activeTabIndex: 0,
	}

	// 1. 打开第一个会话
	s1 := &TerminalSession{ID: "sess-1", Host: h, Title: h.Name, ColorIndex: 0}
	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeSession,
		Title:   h.Name,
		Session: s1,
	})
	v.updateHostTabTitles(h.ID)
	if v.tabs[1].Title != "Ubuntu-Prod" {
		t.Errorf("Expected single tab title to be 'Ubuntu-Prod', got %q", v.tabs[1].Title)
	}
	if v.getHostSessionCount(h.ID) != 1 {
		t.Errorf("Expected session count 1, got %d", v.getHostSessionCount(h.ID))
	}

	// 2. 打开第二个会话 (同一主机)
	s2 := &TerminalSession{ID: "sess-2", Host: h, Title: h.Name, ColorIndex: 0}
	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeSession,
		Title:   h.Name,
		Session: s2,
	})
	v.updateHostTabTitles(h.ID)
	if v.getHostSessionCount(h.ID) != 2 {
		t.Errorf("Expected session count 2, got %d", v.getHostSessionCount(h.ID))
	}
	if v.tabs[1].Title != "Ubuntu-Prod (1)" || v.tabs[2].Title != "Ubuntu-Prod (2)" {
		t.Errorf("Expected titles 'Ubuntu-Prod (1)' and 'Ubuntu-Prod (2)', got %q and %q", v.tabs[1].Title, v.tabs[2].Title)
	}

	// 3. 打开第三个会话 (同一主机)
	s3 := &TerminalSession{ID: "sess-3", Host: h, Title: h.Name, ColorIndex: 0}
	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeSession,
		Title:   h.Name,
		Session: s3,
	})
	v.updateHostTabTitles(h.ID)
	if v.tabs[3].Title != "Ubuntu-Prod (3)" {
		t.Errorf("Expected 3rd tab title 'Ubuntu-Prod (3)', got %q", v.tabs[3].Title)
	}

	// 4. 关闭中间的会话 (索引 2)
	v.tabs = append(v.tabs[:2], v.tabs[3:]...)
	v.updateHostTabTitles(h.ID)
	if len(v.tabs) != 3 {
		t.Fatalf("Expected 3 tabs after closing, got %d", len(v.tabs))
	}
	if v.tabs[1].Title != "Ubuntu-Prod (1)" || v.tabs[2].Title != "Ubuntu-Prod (2)" {
		t.Errorf("Expected re-numbered titles 'Ubuntu-Prod (1)' and 'Ubuntu-Prod (2)', got %q and %q", v.tabs[1].Title, v.tabs[2].Title)
	}

	// 5. 再次关闭一个会话，仅剩一个时自动恢复无序号名称
	v.tabs = append(v.tabs[:2], v.tabs[3:]...)
	v.updateHostTabTitles(h.ID)
	if v.tabs[1].Title != "Ubuntu-Prod" {
		t.Errorf("Expected single remaining tab title to revert to 'Ubuntu-Prod', got %q", v.tabs[1].Title)
	}
}

func TestSession_Disconnect(t *testing.T) {
	s := &TerminalSession{
		Connected:  true,
		Connecting: false,
	}

	s.Disconnect()

	if s.Connected {
		t.Errorf("Expected Connected == false after Disconnect()")
	}
	if s.Connecting {
		t.Errorf("Expected Connecting == false after Disconnect()")
	}
	if s.ErrorMsg != "连接已主动断开" {
		t.Errorf("Expected ErrorMsg '连接已主动断开', got %q", s.ErrorMsg)
	}
}

func TestAppView_RenameTab(t *testing.T) {
	h := &model.Host{ID: "host-1", Name: "Ubuntu-Prod"}
	s := &TerminalSession{ID: "sess-1", Host: h, Title: h.Name}

	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
			{Type: TabTypeSession, Title: h.Name, Session: s},
		},
		activeTabIndex: 1,
	}

	// 1. 打开重命名弹窗
	v.openRenameModal(1)
	if !v.renameModalOpen {
		t.Fatalf("Expected renameModalOpen == true")
	}
	if v.renameInput != "Ubuntu-Prod" || v.renameDefault != "Ubuntu-Prod" {
		t.Errorf("Unexpected renameInput %q or renameDefault %q", v.renameInput, v.renameDefault)
	}

	// 2. 保存新名称
	v.renameInput = "生产 Nginx 节点"
	v.saveRenameTab()

	if v.renameModalOpen {
		t.Errorf("Expected renameModalOpen == false after save")
	}
	if v.tabs[1].Title != "生产 Nginx 节点" {
		t.Errorf("Expected tab title '生产 Nginx 节点', got %q", v.tabs[1].Title)
	}
	if !v.tabs[1].CustomTitle {
		t.Errorf("Expected CustomTitle == true")
	}
	if s.Title != "生产 Nginx 节点" {
		t.Errorf("Expected session Title '生产 Nginx 节点', got %q", s.Title)
	}

	// 3. 验证多 Tab 自动编号不会覆盖用户自定义标题
	s2 := &TerminalSession{ID: "sess-2", Host: h, Title: h.Name}
	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeSession,
		Title:   h.Name,
		Session: s2,
	})
	v.updateHostTabTitles(h.ID)

	if v.tabs[1].Title != "生产 Nginx 节点" {
		t.Errorf("Expected custom tab title to remain '生产 Nginx 节点', got %q", v.tabs[1].Title)
	}
	if v.tabs[2].Title != "Ubuntu-Prod (2)" {
		t.Errorf("Expected second tab to be numbered 'Ubuntu-Prod (2)', got %q", v.tabs[2].Title)
	}

	// 4. 重命名为空时恢复默认
	v.openRenameModal(1)
	v.renameInput = ""
	v.saveRenameTab()

	if v.tabs[1].CustomTitle {
		t.Errorf("Expected CustomTitle == false after resetting name")
	}
	if v.tabs[1].Title != "Ubuntu-Prod (1)" {
		t.Errorf("Expected tab title to revert to 'Ubuntu-Prod (1)', got %q", v.tabs[1].Title)
	}
}

func TestTab_DuplicateTab(t *testing.T) {
	v := &AppView{
		tabs: []WorkspaceTab{
			{Type: TabTypeOverview, Title: "概览"},
		},
	}

	// 1. 测试越界或 Overview 复制保护
	v.duplicateTab(0)
	if len(v.tabs) != 1 {
		t.Fatalf("Overview tab should not be duplicated, got %d tabs", len(v.tabs))
	}
	v.duplicateTab(-1)
	v.duplicateTab(99)
	if len(v.tabs) != 1 {
		t.Fatalf("Invalid indices should not create tabs")
	}

	// 2. 测试复制 Session Tab (验证空间邻近插入及标题序号重排序)
	h1 := &model.Host{ID: "host-1", Name: "HostA", Address: "10.0.0.1", Port: 22, Username: "root"}
	h2 := &model.Host{ID: "host-2", Name: "HostB", Address: "10.0.0.2", Port: 22, Username: "admin"}

	s1 := &TerminalSession{ID: "sess-1", Host: h1, Title: h1.Name}
	s2 := &TerminalSession{ID: "sess-2", Host: h2, Title: h2.Name}

	v.tabs = append(v.tabs,
		WorkspaceTab{Type: TabTypeSession, Title: h1.Name, Session: s1},
		WorkspaceTab{Type: TabTypeSession, Title: h2.Name, Session: s2},
	)
	// 当前 Tabs: [0: Overview, 1: HostA, 2: HostB]

	// 复制索引 1 (HostA)
	v.duplicateTab(1)
	// 期望新标签插入至索引 2 (HostA 与 HostB 之间)，总标签数变为 4
	if len(v.tabs) != 4 {
		t.Fatalf("Expected 4 tabs after duplicate, got %d", len(v.tabs))
	}
	if v.activeTabIndex != 2 {
		t.Errorf("Expected activeTabIndex == 2, got %d", v.activeTabIndex)
	}
	if v.tabs[1].Session.Host.ID != "host-1" || v.tabs[2].Session.Host.ID != "host-1" {
		t.Errorf("Expected tabs 1 and 2 to be host-1")
	}
	if v.tabs[3].Session.Host.ID != "host-2" {
		t.Errorf("Expected tab 3 to remain host-2")
	}
	// 验证动态序号更新
	if v.tabs[1].Title != "HostA (1)" || v.tabs[2].Title != "HostA (2)" {
		t.Errorf("Expected numbered titles 'HostA (1)' and 'HostA (2)', got %q and %q", v.tabs[1].Title, v.tabs[2].Title)
	}

	// 3. 测试复制 Editor Tab
	ed := &EditorFile{Session: s1, Path: "/etc/nginx/nginx.conf", Name: "nginx.conf", Size: 1024}
	v.tabs = append(v.tabs, WorkspaceTab{
		Type:    TabTypeEditor,
		Title:   "nginx.conf",
		Session: s1,
		Editor:  ed,
	})
	// 当前 Tabs: [0: Overview, 1: HostA(1), 2: HostA(2), 3: HostB, 4: nginx.conf]
	editorIdx := 4
	v.duplicateTab(editorIdx)

	if len(v.tabs) != 6 {
		t.Fatalf("Expected 6 tabs after duplicating editor tab, got %d", len(v.tabs))
	}
	if v.activeTabIndex != 5 {
		t.Errorf("Expected activeTabIndex == 5, got %d", v.activeTabIndex)
	}
	if v.tabs[5].Type != TabTypeEditor || v.tabs[5].Editor == nil {
		t.Fatalf("Expected duplicated tab to be editor tab")
	}
	if v.tabs[5].Editor.Path != "/etc/nginx/nginx.conf" || v.tabs[5].Title != "nginx.conf" {
		t.Errorf("Expected duplicated editor tab path and title to match original")
	}
}

func TestTab_ClipboardCopyStrings(t *testing.T) {
	hSSH := &model.Host{
		ID: "h-ssh", Name: "Web-1", Address: "192.168.1.100", Port: 2222, Username: "ubuntu",
	}
	connStr := fmt.Sprintf("%s@%s:%d", hSSH.Username, hSSH.Address, hSSH.Port)
	if connStr != "ubuntu@192.168.1.100:2222" {
		t.Errorf("Unexpected connStr: %s", connStr)
	}

	hNoUser := &model.Host{
		ID: "h-ssh-nouser", Name: "Web-2", Address: "10.0.0.5", Port: 22,
	}
	connStrNoUser := fmt.Sprintf("%s:%d", hNoUser.Address, hNoUser.Port)
	if connStrNoUser != "10.0.0.5:22" {
		t.Errorf("Unexpected connStrNoUser: %s", connStrNoUser)
	}
}

func TestSession_LocalStartIsAsync(t *testing.T) {
	updateCalled := make(chan struct{}, 10)
	session := NewTerminalSession(&model.Host{
		ID:       "local",
		Name:     "本地终端",
		Address:  "localhost",
		AuthType: model.AuthTypeLocal,
	})
	session.OnUpdate = func() {
		select {
		case updateCalled <- struct{}{}:
		default:
		}
	}

	session.Start()

	// 验证 Start 立即返回且处于连接中或已连接状态
	session.mu.Lock()
	connecting := session.Connecting
	connected := session.Connected
	session.mu.Unlock()

	if !connecting && !connected {
		t.Errorf("expected session to be Connecting or Connected immediately after Start()")
	}

	// 验证触发了 UI 刷新通知
	select {
	case <-updateCalled:
		// 收到通知
	case <-time.After(2 * time.Second):
		t.Errorf("expected notifyUpdate to be called on session.Start()")
	}

	session.Close()
}



