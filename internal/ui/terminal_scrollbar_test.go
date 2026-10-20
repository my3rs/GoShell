package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// noopTermConn 实现 io.ReadWriteCloser，用于在测试中创建不依赖真实子进程的 terminal.Terminal，并记录写入的输入字节
type noopTermConn struct {
	closed chan struct{}
	once   sync.Once
	mu     sync.Mutex
	sent   []byte
}

func newNoopTermConn() *noopTermConn {
	return &noopTermConn{closed: make(chan struct{})}
}

func (c *noopTermConn) Read([]byte) (int, error) {
	<-c.closed
	return 0, os.ErrClosed
}

func (c *noopTermConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	c.sent = append(c.sent, b...)
	c.mu.Unlock()
	return len(b), nil
}

func (c *noopTermConn) Sent() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(c.sent)
}

func (c *noopTermConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

// newScrollableTestTerminalWithConn 创建一个包含足够历史输出、可触发滚动条的测试终端并返回底层连接
func newScrollableTestTerminalWithConn(t *testing.T, totalLines int) (*terminal.Terminal, *noopTermConn) {
	t.Helper()
	conn := newNoopTermConn()
	term, err := terminal.New(terminal.Options{
		Conn:       conn,
		Scrollback: 1000,
	})
	if err != nil {
		t.Fatalf("创建测试终端失败: %v", err)
	}
	t.Cleanup(func() {
		_ = term.Close()
	})

	var sb strings.Builder
	for i := 1; i <= totalLines; i++ {
		fmt.Fprintf(&sb, "line-%03d\r\n", i)
	}
	term.Feed([]byte(sb.String()))
	return term, conn
}

// newScrollableTestTerminal 创建一个包含足够历史输出、可触发滚动条的测试终端
func newScrollableTestTerminal(t *testing.T, totalLines int) *terminal.Terminal {
	t.Helper()
	term, _ := newScrollableTestTerminalWithConn(t, totalLines)
	return term
}

func TestCalcThumbGeometryAndTargetOffset(t *testing.T) {
	// 1. 无需滚动时返回满高
	y, h := calcThumbGeometry(400, 24, 0, 24)
	if y != 0 || h != 400 {
		t.Fatalf("不可滚动时期望 y=0, h=400, 实际 y=%v, h=%v", y, h)
	}

	// 2. 常规比例计算：total=100, len=25, viewH=400 -> thumbH=100, travel=300
	yTop, hTop := calcThumbGeometry(400, 100, 0, 25)
	if yTop != 0 || hTop != 100 {
		t.Fatalf("顶部期望 y=0, h=100, 实际 y=%v, h=%v", yTop, hTop)
	}
	yMid, _ := calcThumbGeometry(400, 100, 37, 25)
	if yMid <= 0 || yMid >= 300 {
		t.Fatalf("中部期望 0 < y < 300, 实际 y=%v", yMid)
	}
	yBot, _ := calcThumbGeometry(400, 100, 75, 25)
	if yBot != 300 {
		t.Fatalf("底部期望 y=300, 实际 y=%v", yBot)
	}

	// 3. 最小滑块高度夹紧：total=10000, len=24, viewH=400 -> 最小 24px
	_, hSmall := calcThumbGeometry(400, 10000, 0, 24)
	if hSmall != termScrollbarMinThumbH {
		t.Fatalf("极长历史期望最小滑块高度 %v, 实际 %v", termScrollbarMinThumbH, hSmall)
	}

	// 4. 目标 Offset 映射：拖拽到顶、中、底及越界夹紧
	if got := calcTargetOffset(400, 100, -50, 10, 100, 25); got != 0 {
		t.Fatalf("越界上方期望 offset=0, 实际 %d", got)
	}
	if got := calcTargetOffset(400, 100, 160, 10, 100, 25); got != 38 {
		t.Fatalf("中点期望 offset=38, 实际 %d", got)
	}
	if got := calcTargetOffset(400, 100, 500, 10, 100, 25); got != 75 {
		t.Fatalf("越界下方期望 offset=75, 实际 %d", got)
	}
}

func TestReadAndScrollTerminalScrollbar(t *testing.T) {
	term := newScrollableTestTerminal(t, 120)

	info := readTerminalScrollbar(term)
	if !info.scrollable() {
		t.Fatalf("期望终端处于可滚动状态, 实际 info=%+v", info)
	}
	maxOffset := int(info.Total - info.Len)
	if int(info.Offset) != maxOffset {
		t.Fatalf("初始状态下期望位于最底部 offset=%d, 实际 %d", maxOffset, info.Offset)
	}

	// 滚动到顶部
	if !scrollTerminalToOffset(term, 0) {
		t.Fatal("scrollTerminalToOffset(0) 失败")
	}
	if got := readTerminalScrollbar(term); got.Offset != 0 {
		t.Fatalf("滚动到顶部后期望 offset=0, 实际 %d", got.Offset)
	}

	// 滚动到指定中间行
	mid := maxOffset / 2
	if !scrollTerminalToOffset(term, mid) {
		t.Fatalf("scrollTerminalToOffset(%d) 失败", mid)
	}
	if got := readTerminalScrollbar(term); int(got.Offset) != mid {
		t.Fatalf("滚动到中间行后期望 offset=%d, 实际 %d", mid, got.Offset)
	}

	// 滚动回最底部
	if !scrollTerminalToOffset(term, maxOffset+50) {
		t.Fatal("scrollTerminalToOffset(bottom) 失败")
	}
	if got := readTerminalScrollbar(term); int(got.Offset) != maxOffset {
		t.Fatalf("滚动到底部后期望 offset=%d, 实际 %d", maxOffset, got.Offset)
	}

	// 切换到备用屏幕（AltScreen）时应禁用滚动条
	term.Feed([]byte("\x1b[?1049h"))
	altInfo := readTerminalScrollbar(term)
	if !altInfo.AltScreen || altInfo.scrollable() {
		t.Fatalf("AltScreen 下期望禁用滚动条, 实际 info=%+v", altInfo)
	}
}

func TestTerminalScrollbarPointerDragAndKeyboardInput(t *testing.T) {
	term, conn := newScrollableTestTerminalWithConn(t, 150)
	var st terminalScrollbarState
	focusedCount := 0

	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			elem := terminal.View(c, term).Grow(1)
			attachTerminalScrollbar(c, elem, term, &st, func() {
				focusedCount++
			})
		})
	}, 800, 600)

	// 先渲染两帧完成布局并同步终端尺寸与 viewH
	tester.Frame()
	tester.Frame()

	initSB := readTerminalScrollbar(term)
	if !initSB.scrollable() {
		t.Fatalf("期望测试终端可滚动, 实际 %+v", initSB)
	}
	maxOffset := int(initSB.Total - initSB.Len)

	// 1. 点击轨道顶部区域（X=795 在右侧 12px 热区内，Y=10 靠近顶部）应直接跳转到顶部附近
	tester.Press(795, 10)
	if !st.dragging {
		t.Fatal("在滚动条热区按下鼠标左键后，st.dragging 应为 true")
	}
	if focusedCount == 0 {
		t.Fatal("点击滚动条时应触发 onFocus 回调")
	}
	afterTrackClick := readTerminalScrollbar(term)
	if afterTrackClick.Offset != 0 {
		t.Fatalf("点击轨道顶部后期望 offset=0, 实际 %d", afterTrackClick.Offset)
	}

	// 2. 按住滑块向下拖拽到视口中部 (Y=300)
	tester.Move(795, 300)
	afterMidDrag := readTerminalScrollbar(term)
	if afterMidDrag.Offset == 0 || int(afterMidDrag.Offset) >= maxOffset {
		t.Fatalf("拖拽至中部后期望 0 < offset < %d, 实际 %d", maxOffset, afterMidDrag.Offset)
	}

	// 3. 继续向下拖拽到视口底部 (Y=595)
	tester.Move(795, 595)
	afterBotDrag := readTerminalScrollbar(term)
	if int(afterBotDrag.Offset) != maxOffset {
		t.Fatalf("拖拽至底部后期望 offset=%d, 实际 %d", maxOffset, afterBotDrag.Offset)
	}

	// 4. 释放鼠标左键后应结束拖拽状态，且终端本身保持焦点可直接输入
	tester.Release(795, 595)
	if st.dragging {
		t.Fatal("释放鼠标左键后 st.dragging 应重置为 false")
	}

	tester.Type("pwd")
	tester.Key(0, ui.KeyEnter)
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(conn.Sent(), "pwd") {
		if time.Now().After(deadline) {
			t.Fatalf("拖拽滚动条后终端未能直接接收键盘输入，实际已发送: %q", conn.Sent())
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 5. 在普通终端文本区（X=200, Y=200）按下鼠标不应触发滚动条拖拽，且终端可继续正常接收键盘输入
	tester.Press(200, 200)
	if st.dragging {
		t.Fatal("在终端文本区按下鼠标不应触发滚动条拖拽")
	}
	tester.Release(200, 200)

	tester.Type("ls -la")
	tester.Key(0, ui.KeyEnter)

	deadline = time.Now().Add(2 * time.Second)
	for !strings.Contains(conn.Sent(), "ls -la") {
		if time.Now().After(deadline) {
			t.Fatalf("终端未能接收到键盘输入，实际已发送: %q", conn.Sent())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
