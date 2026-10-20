package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"goshell/internal/app"
	"goshell/internal/core/model"
	"goshell/internal/core/security"
	"goshell/internal/core/storage"
)

func newHostDragView(t *testing.T, count int) *AppView {
	t.Helper()
	cipher, err := security.NewCipherService("")
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(filepath.Join(t.TempDir(), "hosts.db"), cipher)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for i := 0; i <= count; i++ {
		h := &model.Host{ID: fmt.Sprintf("host-%d", i), Name: fmt.Sprintf("主机-%d", i), Address: "127.0.0.1", Port: 22, AuthType: model.AuthTypePassword, SortOrder: i}
		if i == 0 {
			h.AuthType = model.AuthTypeLocal
		}
		if err := store.SaveHost(h); err != nil {
			t.Fatal(err)
		}
	}
	a := &app.App{Store: store}
	if err := a.ReloadData(); err != nil {
		t.Fatal(err)
	}
	spring := ui.Snappy
	return &AppView{app: a, theme: DockerMateTheme(), sidebarWidth: 240, hostDropAnim: &spring}
}

func hostOrder(v *AppView) []string {
	var ids []string
	for _, h := range v.app.Hosts {
		ids = append(ids, h.ID)
	}
	return ids
}

func hostNameRect(t *testing.T, tt *ui.Tester, name string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(name)
	if !ok {
		t.Fatalf("未找到主机：%s", name)
	}
	return r
}

func TestHostDragPointerInteractions(t *testing.T) {
	for _, scenario := range []string{"单击", "轻微移动", "向下排序", "向上排序", "移出列表", "返回原行", "Esc取消", "本地目标", "搜索禁排"} {
		t.Run(scenario, func(t *testing.T) {
			v := newHostDragView(t, 3)
			if scenario == "搜索禁排" {
				v.searchQuery = "主机"
			}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
			}, 500, 400)
			before := hostOrder(v)
			source := hostNameRect(t, tt, "主机-1")
			target := hostNameRect(t, tt, "主机-3")
			if scenario == "向上排序" {
				source, target = target, source
			}
			x, y := source.X+10, source.Y+4
			tt.Press(x, y)
			switch scenario {
			case "单击":
				tt.Release(x, y)
			case "轻微移动":
				tt.Move(x+2, y+1)
				tt.Release(x+2, y+1)
			default:
				tt.Move(x+8, y)
				endX, endY := target.X+10, target.Y+22
				if scenario == "向上排序" {
					endY = target.Y + 2
				}
				tt.Move(endX, endY)
				switch scenario {
				case "移出列表":
					endX, endY = 400, 200
				case "返回原行":
					endX, endY = x, y
				case "Esc取消":
					tt.Key(0, ui.KeyEscape)
				case "本地目标":
					local := hostNameRect(t, tt, "主机-0")
					endX, endY = local.X+10, local.Y+4
				}
				tt.Move(endX, endY)
				tt.Release(endX, endY)
			}
			want := before
			if scenario == "向下排序" {
				want = []string{"host-0", "host-2", "host-3", "host-1"}
			}
			if scenario == "向上排序" {
				want = []string{"host-0", "host-3", "host-1", "host-2"}
			}
			if got := hostOrder(v); !slices.Equal(got, want) {
				t.Fatalf("排序结果 %v，期望 %v", got, want)
			}
			if err := v.app.ReloadData(); err != nil {
				t.Fatal(err)
			}
			if got := hostOrder(v); !slices.Equal(got, want) {
				t.Fatalf("重新加载后排序未持久化：%v，期望 %v", got, want)
			}
			// 结束后仍能正常显示悬浮操作，不留下拖动态。
			r := hostNameRect(t, tt, "主机-1")
			tt.Move(r.X+10, r.Y+4)
			if !tt.HasText("连接") {
				t.Fatal("操作结束后连接按钮未恢复")
			}
		})
	}
}

func TestHostDragViewChangesCancel(t *testing.T) {
	for _, change := range []string{"隐藏列表", "搜索过滤"} {
		t.Run(change, func(t *testing.T) {
			v := newHostDragView(t, 3)
			visible := true
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Width(240).Height(300).Children(func() {
					if visible {
						v.renderSidebarHostList(c, v.theme)
					}
				})
			}, 500, 400)
			before := hostOrder(v)
			source := hostNameRect(t, tt, "主机-1")
			target := hostNameRect(t, tt, "主机-3")
			tt.Press(source.X+10, source.Y+4)
			tt.Move(target.X+10, target.Y+22)
			if change == "隐藏列表" {
				v.ToggleSidebar()
				visible = false
			} else {
				v.searchQuery = "不存在"
			}
			tt.Frame()
			tt.Release(target.X+10, target.Y+22)
			if change == "隐藏列表" {
				v.ToggleSidebar()
			}
			visible, v.searchQuery = true, ""
			tt.Frame()
			if got := hostOrder(v); !slices.Equal(got, before) {
				t.Fatalf("视图变化后仍提交排序：%v", got)
			}
			r := hostNameRect(t, tt, "主机-1")
			tt.Move(r.X+10, r.Y+4)
			if !tt.HasText("连接") {
				t.Fatal("视图恢复后残留拖动态")
			}
		})
	}
}

func TestHostDragCanceledBeforeViewReturns(t *testing.T) {
	for _, change := range []string{"折叠", "切换文件", "搜索"} {
		t.Run(change, func(t *testing.T) {
			v := newHostDragView(t, 3)
			tt := ui.NewTester(func(c *ui.Context) { v.View(c) }, 1000, 600)
			before := hostOrder(v)
			source := hostNameRect(t, tt, "主机-1")
			target := hostNameRect(t, tt, "主机-3")
			tt.Press(source.X+10, source.Y+4)
			tt.Move(target.X+10, target.Y+22)
			switch change {
			case "折叠":
				v.ToggleSidebar()
			case "切换文件":
				v.sidebarTab = 1
			case "搜索":
				v.searchQuery = "不存在"
			}
			tt.Frame()
			switch change {
			case "折叠":
				v.ToggleSidebar()
			case "切换文件":
				v.sidebarTab = 0
			case "搜索":
				v.searchQuery = ""
			}
			tt.Frame()
			target = hostNameRect(t, tt, "主机-3")
			tt.Move(target.X+10, target.Y+22)
			tt.Release(target.X+10, target.Y+22)
			if got := hostOrder(v); !slices.Equal(got, before) {
				t.Fatalf("恢复视图后接受了旧拖动：%v", got)
			}
		})
	}
}

func TestHostDragKeepsRowLayout(t *testing.T) {
	v := newHostDragView(t, 3)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)
	source := hostNameRect(t, tt, "主机-1")
	target := hostNameRect(t, tt, "主机-3")
	tt.Move(target.X+10, target.Y+4)
	if hovered := hostNameRect(t, tt, "主机-3"); hovered != target {
		t.Fatalf("悬浮操作改变主机文字布局：%v -> %v", target, hovered)
	}
	tt.Press(source.X+10, source.Y+4)
	tt.Move(source.X+20, source.Y+4)
	for _, y := range []float32{target.Y + 2, target.Y + 22, target.Y + 2} {
		tt.Move(target.X+10, y)
		if got := hostNameRect(t, tt, "主机-3"); got != target {
			t.Fatalf("插入线改变行布局：%v -> %v", target, got)
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Release(target.X+10, target.Y+2)
}

func TestHostDropAnimatesRows(t *testing.T) {
	v := newHostDragView(t, 3)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)
	source := hostNameRect(t, tt, "主机-1")
	target := hostNameRect(t, tt, "主机-3")
	tt.Press(source.X+10, source.Y+4)
	tt.Move(target.X+10, target.Y+22)
	tt.Release(target.X+10, target.Y+22)

	// 松手后的第一帧：被拖主机应停留在原地附近平滑起步，不应瞬移到目标位置 150+。
	if got := hostNameRect(t, tt, "主机-1"); got.Y > source.Y+2 || got.Y < source.Y-2 {
		t.Fatalf("松手第一帧主机-1 已瞬移：%v -> %v", source.Y, got.Y)
	}
	// 随后若干帧推进时间：位移补间持续进行，并最终落位到目标行下方。
	moved := false
	for i := 0; i < 40; i++ {
		tt.Advance(20 * time.Millisecond)
		if y := hostNameRect(t, tt, "主机-1").Y; y > source.Y+2 {
			moved = true
		}
	}
	if !moved {
		t.Fatal("松手后没有补间动画，行位置直接跳变")
	}
	if got := hostNameRect(t, tt, "主机-1"); got.Y > target.Y+2 || got.Y < target.Y-2 {
		t.Fatalf("动画结束后主机-1 未落在目标行：%v，期望 %v", got.Y, target.Y)
	}
	if got := hostNameRect(t, tt, "主机-2"); got.Y > source.Y+2 || got.Y < source.Y-2 {
		t.Fatalf("动画结束后主机-2 未补位：%v，期望 %v", got.Y, source.Y)
	}
}

func TestHostDragScrollsAtEdge(t *testing.T) {
	v := newHostDragView(t, 30)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)
	source := hostNameRect(t, tt, "主机-1")
	tt.Press(source.X+10, source.Y+4)
	tt.Move(source.X+20, source.Y+4)
	tt.Move(100, 294)
	for i := 0; i < 10; i++ {
		tt.Frame()
	}
	last := hostNameRect(t, tt, "主机-30")
	if last.Y >= 300 {
		t.Fatalf("拖动到底部未自动滚动到列表末尾：%v", last)
	}
	tt.Move(100, 40)
	for i := 0; i < 10; i++ {
		tt.Frame()
	}
	first := hostNameRect(t, tt, "主机-1")
	if first.Y < 40 || first.Y >= 300 {
		t.Fatalf("拖动到顶部未自动滚回：%v", first)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Release(100, 40)
	if v.app.Hosts[1].ID != "host-1" {
		t.Fatal("取消自动滚动中的拖动后顺序改变")
	}
}

func TestHostRowContextMenuStillWorks(t *testing.T) {
	v := newHostDragView(t, 3)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)
	if err := tt.RightClick("主机-2"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tt.Menu(), "编辑") || !slices.Contains(tt.Menu(), "上移") {
		t.Fatal("主机右键菜单未正常显示")
	}
	if err := tt.ChooseMenuItem("上移"); err != nil {
		t.Fatal(err)
	}
	want := []string{"host-0", "host-2", "host-1", "host-3"}
	if got := hostOrder(v); !slices.Equal(got, want) {
		t.Fatalf("菜单排序结果：%v，期望 %v", got, want)
	}
}

func TestHostDragReportsSaveFailure(t *testing.T) {
	v := newHostDragView(t, 3)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)
	before := hostOrder(v)
	if err := v.app.Store.Close(); err != nil {
		t.Fatal(err)
	}
	source := hostNameRect(t, tt, "主机-1")
	target := hostNameRect(t, tt, "主机-3")
	tt.Press(source.X+10, source.Y+4)
	tt.Move(target.X+10, target.Y+22)
	tt.Release(target.X+10, target.Y+22)
	if !tt.HasText("失败") {
		t.Fatal("排序保存失败后未显示错误提示")
	}
	if got := hostOrder(v); !slices.Equal(got, before) {
		t.Fatalf("失败后显示了未保存的顺序：%v", got)
	}
}

func TestHostReorderSkipsUnchangedOrder(t *testing.T) {
	v := newHostDragView(t, 3)
	// 关闭数据库后，原地排序仍应成功，证明没有无意义的持久化写入。
	if err := v.app.Store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, move := range []struct {
		source, target string
		position       int
	}{
		{"host-1", "host-1", 1},
		{"host-1", "host-2", -1},
		{"host-2", "host-1", 1},
	} {
		if err := v.reorderHosts(move.source, move.target, move.position); err != nil {
			t.Fatalf("原地排序不应写数据库：%v", err)
		}
	}
}

func TestHostFormPreservesOrder(t *testing.T) {
	v := newHostDragView(t, 3)
	if err := v.reorderHosts("host-1", "host-3", 1); err != nil {
		t.Fatal(err)
	}
	want := hostOrder(v)
	v.openEditModal(v.app.Hosts[3])
	v.formName = "重命名后的主机"
	v.saveHostForm()
	if v.formErrorMsg != "" {
		t.Fatal(v.formErrorMsg)
	}
	if got := hostOrder(v); !slices.Equal(got, want) {
		t.Fatalf("编辑破坏顺序：%v，期望 %v", got, want)
	}
	v.formID = ""
	v.formName = "新主机"
	v.saveHostForm()
	if v.formErrorMsg != "" {
		t.Fatal(v.formErrorMsg)
	}
	got := hostOrder(v)
	if len(got) != 5 || !slices.Equal(got[:4], want) {
		t.Fatalf("新增主机未追加到末尾：%v", got)
	}
}
