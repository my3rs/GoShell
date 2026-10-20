package ui

import (
	"math"
	"reflect"
	"sync"
	"unsafe"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

const (
	// termScrollbarHitWidth 定义终端右侧滚动条的鼠标命中热区宽度（DIP）
	termScrollbarHitWidth float32 = 12
	// termScrollbarMinThumbH 定义滚动条滑块的最小可视高度（DIP），与 mygo 终端绘制保持一致
	termScrollbarMinThumbH float32 = 24
)

// terminalScrollbarState 维护单个终端视图的滚动条鼠标拖拽与悬停状态
type terminalScrollbarState struct {
	dragging   bool
	grabOffset float32
	hovering   bool
	viewH      float32
}

// termScrollbarInfo 描述终端缓冲区总行数、当前视口起始行偏移与视口行数
type termScrollbarInfo struct {
	Total     uint64
	Offset    uint64
	Len       uint64
	AltScreen bool
	Valid     bool
}

// scrollable 判断当前终端是否存在可滚动的历史内容（非全屏备用屏且总行数大于视口行数）
func (info termScrollbarInfo) scrollable() bool {
	return info.Valid && !info.AltScreen && info.Total > info.Len && info.Len > 0
}

// vtTerminalScroller 映射底层 vt.Terminal 导出的滚动与状态查询方法
type vtTerminalScroller interface {
	AltScreen() bool
	ScrollToTop()
	ScrollToBottom()
	ScrollBy(rows int)
	ScrollToRow(row int)
}

// calcThumbGeometry 计算滚动条滑块在视口高度 viewH 内的纵向坐标与高度
func calcThumbGeometry(viewH float32, total, offset, length uint64) (thumbY, thumbH float32) {
	if viewH <= 0 || total <= length || length == 0 {
		return 0, viewH
	}
	rawH := viewH * float32(length) / float32(total)
	thumbH = min(max(rawH, termScrollbarMinThumbH), viewH)
	maxOffset := total - length
	if offset > maxOffset {
		offset = maxOffset
	}
	travel := max(viewH-thumbH, 0)
	thumbY = travel * float32(offset) / float32(maxOffset)
	return thumbY, thumbH
}

// calcTargetOffset 根据鼠标当前纵向位置与抓取偏移量计算目标首行 Offset
func calcTargetOffset(viewH, thumbH, pointerY, grabOffset float32, total, length uint64) int {
	if total <= length {
		return 0
	}
	maxOffset := int(total - length)
	travel := viewH - thumbH
	if travel <= 0 {
		return 0
	}
	ratio := float64(pointerY-grabOffset) / float64(travel)
	ratio = max(0, min(1, ratio))
	return int(math.Round(ratio * float64(maxOffset)))
}

// resolveTerminalInternals 提取 terminal.Terminal 内部的互斥锁与底层 vt.Terminal 实例
func resolveTerminalInternals(term *terminal.Terminal) (*sync.Mutex, reflect.Value, vtTerminalScroller, bool) {
	if term == nil {
		return nil, reflect.Value{}, nil, false
	}
	tv := reflect.ValueOf(term)
	if tv.Kind() != reflect.Pointer || tv.IsNil() {
		return nil, reflect.Value{}, nil, false
	}
	te := tv.Elem()
	muField := te.FieldByName("mu")
	termField := te.FieldByName("term")
	if !muField.IsValid() || !muField.CanAddr() || !termField.IsValid() || !termField.CanAddr() {
		return nil, reflect.Value{}, nil, false
	}
	mu := (*sync.Mutex)(unsafe.Pointer(muField.UnsafeAddr()))
	vtVal := reflect.NewAt(termField.Type(), unsafe.Pointer(termField.UnsafeAddr())).Elem()
	if !vtVal.IsValid() || vtVal.IsNil() {
		return mu, reflect.Value{}, nil, false
	}
	scroller, ok := vtVal.Interface().(vtTerminalScroller)
	if !ok {
		return mu, reflect.Value{}, nil, false
	}
	return mu, vtVal, scroller, true
}

// readTerminalScrollbar 线程安全地读取终端当前的滚动条度量信息
func readTerminalScrollbar(term *terminal.Terminal) termScrollbarInfo {
	mu, _, _, ok := resolveTerminalInternals(term)
	if !ok && mu == nil {
		return termScrollbarInfo{}
	}
	mu.Lock()
	defer mu.Unlock()

	_, vtVal, scroller, ok := resolveTerminalInternals(term)
	if !ok {
		return termScrollbarInfo{}
	}
	if scroller.AltScreen() {
		return termScrollbarInfo{AltScreen: true, Valid: true}
	}
	sbMethod := vtVal.MethodByName("Scrollbar")
	if !sbMethod.IsValid() {
		return termScrollbarInfo{}
	}
	out := sbMethod.Call(nil)
	if len(out) != 1 {
		return termScrollbarInfo{}
	}
	sbVal := out[0]
	totalField := sbVal.FieldByName("Total")
	offsetField := sbVal.FieldByName("Offset")
	lenField := sbVal.FieldByName("Len")
	if !totalField.IsValid() || !offsetField.IsValid() || !lenField.IsValid() {
		return termScrollbarInfo{}
	}
	return termScrollbarInfo{
		Total:  totalField.Uint(),
		Offset: offsetField.Uint(),
		Len:    lenField.Uint(),
		Valid:  true,
	}
}

// scrollTerminalToOffset 线程安全地将终端视口滚动至指定首行偏移
func scrollTerminalToOffset(term *terminal.Terminal, targetOffset int) bool {
	mu, _, _, ok := resolveTerminalInternals(term)
	if !ok && mu == nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()

	_, vtVal, scroller, ok := resolveTerminalInternals(term)
	if !ok || scroller.AltScreen() {
		return false
	}
	sbMethod := vtVal.MethodByName("Scrollbar")
	if !sbMethod.IsValid() {
		return false
	}
	out := sbMethod.Call(nil)
	if len(out) != 1 {
		return false
	}
	sbVal := out[0]
	total := sbVal.FieldByName("Total").Uint()
	length := sbVal.FieldByName("Len").Uint()
	if total <= length {
		return false
	}
	maxOffset := int(total - length)
	switch {
	case targetOffset <= 0:
		scroller.ScrollToTop()
	case targetOffset >= maxOffset:
		scroller.ScrollToBottom()
	default:
		scroller.ScrollToRow(targetOffset)
	}
	return true
}

// scrollTerminalBy 线程安全地按相对行数滚动终端视口（供滚动条区域内的滚轮事件使用）
func scrollTerminalBy(term *terminal.Terminal, deltaRows int) bool {
	if deltaRows == 0 {
		return false
	}
	mu, _, _, ok := resolveTerminalInternals(term)
	if !ok && mu == nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()

	_, _, scroller, ok := resolveTerminalInternals(term)
	if !ok || scroller.AltScreen() {
		return false
	}
	scroller.ScrollBy(deltaRows)
	return true
}

// attachTerminalScrollbar 在 terminal.View 右侧边缘挂载独立的绝对定位滚动条子元素，
// 完全保留终端自身的键盘输入、IME 与文本区鼠标事件处理逻辑
func attachTerminalScrollbar(c *ui.Context, termElem ui.Element, term *terminal.Terminal, st *terminalScrollbarState, onFocus func()) {
	if term == nil || st == nil || termElem.ID() == 0 {
		return
	}
	bounds := termElem.LayoutBounds()
	if bounds.H > 0 {
		st.viewH = bounds.H
	}

	sb := readTerminalScrollbar(term)
	if !sb.scrollable() {
		st.dragging = false
		st.hovering = false
		return
	}

	termElem.Children(func() {
		bar := ui.Box(c).Key("term-scrollbar").
			Absolute().Right(0).Top(0).Bottom(0).Width(termScrollbarHitWidth).
			Cursor(ui.CursorDefault)

		st.hovering = bar.Hovered()

		bar.HandleInput(func(ev ui.InputEvent) bool {
			return handleTerminalScrollbarInput(c, term, st, ev, onFocus)
		})

		bar.Draw(func(p *ui.Painter, r ui.Rect) {
			st.viewH = r.H
			curSB := readTerminalScrollbar(term)
			if !curSB.scrollable() {
				return
			}
			drawTerminalScrollbarOverlay(p, r, curSB, st.hovering || st.dragging, st.dragging)
		})
	})
}

// handleTerminalScrollbarInput 处理终端右侧滚动条子元素的鼠标按下、拖拽移动、释放与滚轮事件
func handleTerminalScrollbarInput(c *ui.Context, term *terminal.Terminal, st *terminalScrollbarState, ev ui.InputEvent, onFocus func()) bool {
	sb := readTerminalScrollbar(term)
	if !sb.scrollable() || st.viewH <= 0 {
		st.dragging = false
		return false
	}

	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		if onFocus != nil {
			onFocus()
		}
		thumbY, thumbH := calcThumbGeometry(st.viewH, sb.Total, sb.Offset, sb.Len)
		st.dragging = true
		st.hovering = true
		if ev.Y >= thumbY && ev.Y <= thumbY+thumbH {
			st.grabOffset = ev.Y - thumbY
		} else {
			st.grabOffset = thumbH / 2
			target := calcTargetOffset(st.viewH, thumbH, ev.Y, st.grabOffset, sb.Total, sb.Len)
			scrollTerminalToOffset(term, target)
		}
		c.Invalidate()
		return true

	case ui.InputPointerMove:
		if st.dragging {
			_, thumbH := calcThumbGeometry(st.viewH, sb.Total, sb.Offset, sb.Len)
			target := calcTargetOffset(st.viewH, thumbH, ev.Y, st.grabOffset, sb.Total, sb.Len)
			scrollTerminalToOffset(term, target)
			c.Invalidate()
			return true
		}
		return false

	case ui.InputPointerUp:
		if st.dragging && ev.Button == 0 {
			st.dragging = false
			c.Invalidate()
			return true
		}

	case ui.InputScroll:
		rows := int(math.Round(float64(ev.DY) / 12.0))
		if rows == 0 && ev.DY != 0 {
			if ev.DY > 0 {
				rows = 1
			} else {
				rows = -1
			}
		}
		if scrollTerminalBy(term, rows) {
			c.Invalidate()
			return true
		}
	}
	return false
}

// drawTerminalScrollbarOverlay 在终端右侧绘制滚动条轨道与滑块（支持底部常驻提示与悬停/拖拽加宽高亮）
func drawTerminalScrollbarOverlay(p *ui.Painter, r ui.Rect, sb termScrollbarInfo, activeOrHover, dragging bool) {
	thumbY, thumbH := calcThumbGeometry(r.H, sb.Total, sb.Offset, sb.Len)
	drawH := max(thumbH-4, 4)

	if activeOrHover {
		trackRect := ui.Rect{
			X: r.X + r.W - termScrollbarHitWidth,
			Y: r.Y,
			W: termScrollbarHitWidth,
			H: r.H,
		}
		p.Fill(trackRect, ui.RGBA(255, 255, 255, 0.06), 0)

		alpha := float32(0.65)
		if dragging {
			alpha = 0.85
		}
		thumbRect := ui.Rect{
			X: r.X + r.W - 8,
			Y: r.Y + thumbY + 2,
			W: 6,
			H: drawH,
		}
		p.Fill(thumbRect, ui.RGBA(244, 244, 245, alpha), 3)
		return
	}

	// 当视口位于最底部但存在历史滚动行时，补齐 4px 半透明滑块，使滚动条始终可见可抓
	if sb.Offset+sb.Len >= sb.Total {
		thumbRect := ui.Rect{
			X: r.X + r.W - 7,
			Y: r.Y + thumbY + 2,
			W: 4,
			H: drawH,
		}
		p.Fill(thumbRect, ui.RGBA(244, 244, 245, 0.35), 2)
	}
}
