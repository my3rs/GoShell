package ui

import "unicode/utf8"

// crlfNormalizer 规范化行终止符（等同于 POSIX 伪终端 PTY 的 ONLCR 模式）：
// 将输出流中所有未跟在 \r 后面的裸 \n 规范化为 \r\n，已有的 \r\n 保持不变，同时保留单独 \r 作为行首覆盖。
// 结构体保持前一数据块的末尾状态，彻底避免跨数据块切片时重复插入换行或斜向阶梯楼梯效应。
type crlfNormalizer struct {
	lastByte byte
}

func (n *crlfNormalizer) Normalize(src []byte) []byte {
	if len(src) == 0 {
		return src
	}
	needNorm := false
	prev := n.lastByte
	for i := 0; i < len(src); i++ {
		b := src[i]
		if b == '\n' && prev != '\r' {
			needNorm = true
			break
		}
		prev = b
	}
	if !needNorm {
		n.lastByte = src[len(src)-1]
		return src
	}

	dst := make([]byte, 0, len(src)+len(src)/8+8)
	for i := 0; i < len(src); i++ {
		b := src[i]
		if b == '\n' {
			if n.lastByte != '\r' {
				dst = append(dst, '\r')
			}
			dst = append(dst, '\n')
		} else {
			dst = append(dst, b)
		}
		n.lastByte = b
	}
	return dst
}

// normalizeCRLF 规范化行终止符（无状态单次调用）：将孤立未配对的 \n 规范化为 \r\n，已有 \r\n 保持不变
func normalizeCRLF(b []byte) []byte {
	var n crlfNormalizer
	return n.Normalize(b)
}

// isUTF8 检查字节切片是否为合法的 UTF-8。若末尾仅包含不完整的前缀，incomplete 返回末尾未完成字节数
func isUTF8(b []byte) (valid bool, incomplete int) {
	i := 0
	for i < len(b) {
		if b[i] < 0x80 {
			i++
			continue
		}
		if utf8.FullRune(b[i:]) {
			r, size := utf8.DecodeRune(b[i:])
			if r == utf8.RuneError && size == 1 {
				return false, 0
			}
			i += size
		} else {
			return true, len(b) - i
		}
	}
	return true, 0
}

// gbkIncompleteTail 检查末尾是否含有未配对的 GBK 双字节字符前导字节
func gbkIncompleteTail(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	if b[len(b)-1] < 0x81 {
		return 0
	}
	count := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] >= 0x81 {
			count++
		} else {
			break
		}
	}
	if count%2 == 1 {
		return 1
	}
	return 0
}

// runeWidth 计算 Unicode 字符在等宽终端中的显示宽度（1 格或 2 格）
func runeWidth(r rune) int {
	if r < 0x80 {
		return 1
	}
	// 常见 CJK 汉字、全角标点符号、日韩字符等在终端中占 2 列宽
	if (r >= 0x1100 && r <= 0x115F) || // Hangul Jamo
		(r >= 0x2E80 && r <= 0xA4CF) || // CJK Radicals, Unified Ideographs, Yi
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0xFE10 && r <= 0xFE19) || // Vertical forms
		(r >= 0xFE30 && r <= 0xFE6F) || // CJK Compatibility Forms
		(r >= 0xFF00 && r <= 0xFF60) || // Fullwidth Forms
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFD) ||
		(r >= 0x30000 && r <= 0x3FFFD) {
		return 2
	}
	return 1
}

// skipEscapeSequence 检测并跳过 ANSI 转义序列（如方向键 \x1b[A 等），返回跳过后的新索引
func skipEscapeSequence(runes []rune, i int) int {
	if i >= len(runes) || runes[i] != 0x1b {
		return i
	}
	if i+1 >= len(runes) {
		return i + 1
	}
	next := runes[i+1]
	if next == '[' { // CSI 序列: \x1b[ ... [0x40-0x7E]
		j := i + 2
		for j < len(runes) {
			b := runes[j]
			if b >= 0x40 && b <= 0x7E {
				return j + 1
			}
			j++
		}
		return len(runes)
	} else if next == 'O' { // SS3 序列
		if i+2 < len(runes) {
			return i + 3
		}
		return len(runes)
	}
	return i + 2
}
