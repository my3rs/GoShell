//go:build !windows

package ui

import "io"

// shouldFallbackPipe 检查当前系统是否需要管道降级（非 Windows 系统原生支持 PTY，不需要降级）
func shouldFallbackPipe() bool {
	return false
}

// startFallbackPipeTerminal 非 Windows 平台占位
func startFallbackPipeTerminal(dir string) (io.ReadWriteCloser, error) {
	return nil, nil
}
