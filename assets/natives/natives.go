package natives

import (
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
			if err := os.WriteFile(tmpFile, embeddedBytes, 0o755); err != nil {
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

	return nil
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
