package ui

import "unicode/utf8"

// normalizeCRLF 规范化行终止符：将孤立的 \r 或 \n 规范化为 Windows 标准的 \r\n，已有 \r\n 保持不变
func normalizeCRLF(b []byte) []byte {
	needNorm := false
	for i := 0; i < len(b); i++ {
		if b[i] == '\r' {
			if i+1 == len(b) || b[i+1] != '\n' {
				needNorm = true
				break
			}
			i++ // 跳过随后的 \n
		} else if b[i] == '\n' {
			needNorm = true
			break
		}
	}
	if !needNorm {
		return b
	}
	res := make([]byte, 0, len(b)+4)
	for i := 0; i < len(b); i++ {
		if b[i] == '\r' {
			res = append(res, '\r', '\n')
			if i+1 < len(b) && b[i+1] == '\n' {
				i++
			}
		} else if b[i] == '\n' {
			res = append(res, '\r', '\n')
		} else {
			res = append(res, b[i])
		}
	}
	return res
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
