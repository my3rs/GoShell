package natives

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// Ensure pre-seeds the Ghostty VT terminal engine library so that GoShell
// runs completely offline without any internet connection.
func Ensure() error {
	if LibraryName == "" || LibrarySHA256 == "" {
		return nil
	}

	// 1. 如果已通过环境变量指定，且文件存在，则无需处理
	if envPath := os.Getenv("MYGO_GHOSTTY_VT"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return nil
		}
	}

	// 2. 检查可执行文件同级目录下是否存在有效动态库
	if exe, err := os.Executable(); err == nil {
		exeLib := filepath.Join(filepath.Dir(exe), LibraryName)
		if _, err := os.Stat(exeLib); err == nil {
			return nil
		}
	}

	// 3. 检查用户缓存目录 (<cache>/mygo/natives/<sha256>/<name>)
	var candidateDirs []string
	if cacheDir, err := os.UserCacheDir(); err == nil {
		candidateDirs = append(candidateDirs, filepath.Join(cacheDir, "mygo", "natives", LibrarySHA256))
	}
	candidateDirs = append(candidateDirs, filepath.Join(os.TempDir(), "mygo", "natives", LibrarySHA256))

	for _, dir := range candidateDirs {
		targetPath := filepath.Join(dir, LibraryName)
		if verifyFile(targetPath, LibrarySHA256) {
			_ = os.Setenv("MYGO_GHOSTTY_VT", targetPath)
			return nil
		}
	}

	// 4. 若环境中均不存在，自动从单文件二进制内嵌字节释放到本地并设置环境变量
	if len(embeddedBytes) > 0 {
		for _, dir := range candidateDirs {
			targetPath := filepath.Join(dir, LibraryName)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				continue
			}
			tmpFile := fmt.Sprintf("%s.tmp-%d", targetPath, os.Getpid())
			if err := writeDecompressedBytes(tmpFile, embeddedBytes, 0o755); err != nil {
				continue
			}
			_ = os.Rename(tmpFile, targetPath)
			if verifyFile(targetPath, LibrarySHA256) {
				_ = os.Setenv("MYGO_GHOSTTY_VT", targetPath)
				log.Printf("natives: successfully extracted embedded terminal engine to %s", targetPath)
				return nil
			}
		}
	}

	// 预热就绪 Busybox 虚拟终端程序 (单文件绿色便携版自动释放)
	_, _ = EnsureBusybox()

	return nil
}

// EnsureBusybox 确保 Windows 系统上有可用的 Busybox 虚拟终端可执行程序。
// 1. 若同级目录下存在 busybox.exe，优先返回同级路径；
// 2. 检查缓存目录 (%LOCALAPPDATA%/goshell/natives/<sha256>/busybox.exe)；
// 3. 若缓存不存在且内嵌了 busybox，则自动释放到本地缓存；
// 4. 返回就绪的 busybox.exe 绝对路径。非 Windows 或未内嵌时返回空字符串。
func EnsureBusybox() (string, error) {
	if BusyboxName == "" || BusyboxSHA256 == "" {
		return "", nil
	}

	// 1. 检查环境变量 GOSHELL_BUSYBOX
	if envPath := os.Getenv("GOSHELL_BUSYBOX"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	// 2. 检查可执行文件同级目录
	if exe, err := os.Executable(); err == nil {
		exeBin := filepath.Join(filepath.Dir(exe), BusyboxName)
		if stat, err := os.Stat(exeBin); err == nil && !stat.IsDir() {
			return exeBin, nil
		}
	}

	// 3. 检查用户缓存目录与临时目录
	var candidateDirs []string
	if cacheDir, err := os.UserCacheDir(); err == nil {
		candidateDirs = append(candidateDirs, filepath.Join(cacheDir, "goshell", "natives", BusyboxSHA256))
	}
	candidateDirs = append(candidateDirs, filepath.Join(os.TempDir(), "goshell", "natives", BusyboxSHA256))

	for _, dir := range candidateDirs {
		targetPath := filepath.Join(dir, BusyboxName)
		if verifyFile(targetPath, BusyboxSHA256) {
			return targetPath, nil
		}
	}

	// 4. 若环境中均不存在，自动从单文件二进制内嵌字节释放到本地缓存
	if len(embeddedBusyboxBytes) > 0 {
		for _, dir := range candidateDirs {
			targetPath := filepath.Join(dir, BusyboxName)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				continue
			}
			tmpFile := fmt.Sprintf("%s.tmp-%d", targetPath, os.Getpid())
			if err := writeDecompressedBytes(tmpFile, embeddedBusyboxBytes, 0o755); err != nil {
				continue
			}
			_ = os.Rename(tmpFile, targetPath)
			if verifyFile(targetPath, BusyboxSHA256) {
				log.Printf("natives: successfully extracted embedded busybox to %s", targetPath)
				return targetPath, nil
			}
		}
	}

	return "", fmt.Errorf("busybox not found or failed to extract")
}

// writeDecompressedBytes 将数据流透明写入目标文件：若是 gzip 压缩数据则自动解压写出，否则原样写入
func writeDecompressedBytes(targetPath string, data []byte, perm os.FileMode) error {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		// 非 gzip 数据，直接常规写入（向前兼容）
		return os.WriteFile(targetPath, data, perm)
	}
	defer gzReader.Close()

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, gzReader)
	return err
}

func verifyFile(path string, expectedSHA string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == expectedSHA
}
