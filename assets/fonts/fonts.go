// Package fonts provides embedded typography assets for GoShell.
package fonts

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"sync"

	"github.com/egoist/mygo/ui"
)

const (
	// FamilyName 是 Maple Mono NF 字体的字体族名称。
	FamilyName = "Maple Mono NF"

	// FallbackChain 定义终端与代码编辑器的字体回退优先级链。
	FallbackChain = "Maple Mono NF, SF Mono, Menlo, Cascadia Mono, monospace"
)

//go:embed MapleMono-NF-Regular.ttf.gz
var regularTTFGz []byte

//go:embed MapleMono-NF-Bold.ttf.gz
var boldTTFGz []byte

//go:embed MapleMono-NF-Italic.ttf.gz
var italicTTFGz []byte

var (
	registerOnce sync.Once
	registerErr  error
)

// decompressGzip 解压内存中的 gzip 字节数据
func decompressGzip(compressed []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// Register 将内嵌的 Maple Mono NF gzip 字体解压并注册到 UI 字体排版系统。
func Register() error {
	registerOnce.Do(func() {
		regTTF, err := decompressGzip(regularTTFGz)
		if err != nil {
			registerErr = fmt.Errorf("解压 Maple Mono NF Regular 字体失败: %w", err)
			return
		}
		if err := ui.RegisterFont(regTTF, ""); err != nil {
			registerErr = fmt.Errorf("failed to register Maple Mono NF Regular: %w", err)
			return
		}

		boldTTF, err := decompressGzip(boldTTFGz)
		if err != nil {
			registerErr = fmt.Errorf("解压 Maple Mono NF Bold 字体失败: %w", err)
			return
		}
		if err := ui.RegisterFont(boldTTF, ""); err != nil {
			registerErr = fmt.Errorf("failed to register Maple Mono NF Bold: %w", err)
			return
		}

		italicTTF, err := decompressGzip(italicTTFGz)
		if err != nil {
			registerErr = fmt.Errorf("解压 Maple Mono NF Italic 字体失败: %w", err)
			return
		}
		if err := ui.RegisterFont(italicTTF, ""); err != nil {
			registerErr = fmt.Errorf("failed to register Maple Mono NF Italic: %w", err)
			return
		}
	})
	return registerErr
}
