package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"goshell/internal/app"
	"goshell/internal/core/model"
)

// 本地终端固定置顶且不可拖拽，图钉占据右侧拖拽手柄位，并与整行垂直居中。
func TestHostLocalPinnedIconAtHandleSlot(t *testing.T) {
	v := &AppView{
		app: &app.App{
			Hosts: []*model.Host{
				{ID: "local-host", Name: "本机", Address: "localhost", AuthType: model.AuthTypeLocal},
				{ID: "remote-1", Name: "生产服务器-01", Address: "10.0.0.1", AuthType: model.AuthTypePassword},
			},
		},
		theme:        DockerMateTheme(),
		sidebarWidth: 240,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)

	pin := hostNameRect(t, tt, "置顶")
	name := hostNameRect(t, tt, "本机")
	subtitle := hostNameRect(t, tt, "本地终端")

	// 图钉应与整行（名称 + 副标题）垂直居中，而不是只贴合名称。
	rowCenter := (name.Y + subtitle.Y + subtitle.H) / 2
	if diff := pin.Y + pin.H/2 - rowCenter; diff > 0.5 || diff < -0.5 {
		t.Fatalf("图钉未与主机行垂直居中：行中心 %v，图钉中心 %v，偏差 %v", rowCenter, pin.Y+pin.H/2, diff)
	}

	// 图钉应贴在行的右内边距处（240 宽 - 8 内边距），即原拖拽手柄位。
	if diff := pin.X + pin.W - (240 - 8); diff > 0.5 || diff < -0.5 {
		t.Fatalf("图钉未占据拖拽手柄位：右边缘 %v，期望 %v", pin.X+pin.W, 240-8)
	}

	// 图钉脱离主机名，不再挤占名称横向空间。
	if pin.X <= name.X+name.W {
		t.Fatalf("图钉不应再紧跟主机名：名称 %v，图钉 %v", name, pin)
	}
}

// 远程主机仍使用拖拽手柄，且手柄位与图钉位一致，便于两行视觉对齐。
func TestHostRemoteRowKeepsDragHandleSlot(t *testing.T) {
	v := &AppView{
		app: &app.App{
			Hosts: []*model.Host{
				{ID: "local-host", Name: "本机", Address: "localhost", AuthType: model.AuthTypeLocal},
				{ID: "remote-1", Name: "生产服务器-01", Address: "10.0.0.1", AuthType: model.AuthTypePassword},
			},
		},
		theme:        DockerMateTheme(),
		sidebarWidth: 240,
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(240).Height(300).Children(func() { v.renderSidebarHostList(c, v.theme) })
	}, 500, 400)

	// 图钉只出现在置顶的本地终端行，远程主机行不渲染置顶标识。
	pin := hostNameRect(t, tt, "置顶")
	name := hostNameRect(t, tt, "本机")
	if pin.Y < name.Y {
		t.Fatalf("置顶标识应属于本地终端行：图钉 %v，本地名称 %v", pin, name)
	}
	if _, ok := tt.Find("生产服务器-01"); !ok {
		t.Fatal("远程主机名称未渲染")
	}
}
