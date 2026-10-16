package sftp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// FileEntry 远程文件或目录的元数据
type FileEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	SizeHuman string    `json:"size_human"`
	Mode      string    `json:"mode"`
	IsDir     bool      `json:"is_dir"`
	ModTime   time.Time `json:"mod_time"`
}

// Client 封装了基于 pkg/sftp 的远程文件管理服务
type Client struct {
	rawClient  *ssh.Client
	sftpClient *sftp.Client
	mu         sync.Mutex
	closed     bool
}

// NewClient 使用已建立的 SSH 物理连接创建独立 SFTP 会话管道
func NewClient(sshClient *ssh.Client) (*Client, error) {
	if sshClient == nil {
		return nil, errors.New("SSH 物理连接不能为空")
	}

	sc, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("创建 SFTP 客户端子通道失败: %w", err)
	}

	return &Client{
		rawClient:  sshClient,
		sftpClient: sc,
	}, nil
}

// GetWorkingDirectory 获取远程默认工作目录（通常为远程用户的主目录）
func (c *Client) GetWorkingDirectory() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return "", errors.New("SFTP 连接已关闭")
	}

	cwd, err := c.sftpClient.Getwd()
	if err != nil {
		return "/", nil
	}
	if cwd == "" {
		return "/", nil
	}
	return cwd, nil
}

// ListDirectory 读取远程路径下的文件列表，并按文件夹优先、字母升序排列
func (c *Client) ListDirectory(remotePath string) ([]FileEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("SFTP 连接已关闭")
	}

	if remotePath == "" {
		remotePath = "/"
	}
	remotePath = path.Clean(remotePath)

	entries, err := c.sftpClient.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("读取远程目录失败 (%s): %w", remotePath, err)
	}

	var result []FileEntry
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		fullPath := path.Join(remotePath, name)
		isDir := entry.IsDir()

		// 若条目为符号链接，检查其实际指向的目标是否为目录
		if !isDir && (entry.Mode()&os.ModeSymlink != 0) {
			if targetInfo, err := c.sftpClient.Stat(fullPath); err == nil {
				isDir = targetInfo.IsDir()
			}
		}

		result = append(result, FileEntry{
			Name:      name,
			Path:      fullPath,
			Size:      entry.Size(),
			SizeHuman: FormatSize(entry.Size(), isDir),
			Mode:      entry.Mode().String(),
			IsDir:     isDir,
			ModTime:   entry.ModTime(),
		})
	}

	// 排序：目录在前，同类型按名称不区分大小写升序
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// OpenFile 打开远程文件用于读取
func (c *Client) OpenFile(remotePath string) (io.ReadCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.Open(remotePath)
}

// CreateFile 创建远程文件用于写入（覆盖已有文件）
func (c *Client) CreateFile(remotePath string) (io.WriteCloser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.Create(remotePath)
}

// ReadFile 读取指定远程文件的全部字节数据（限制最大读取大小，防止超大文件耗尽内存）
func (c *Client) ReadFile(remotePath string, maxBytes int64) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("SFTP 连接已关闭")
	}

	f, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("打开远程文件失败: %w", err)
	}
	defer f.Close()

	r := io.LimitReader(f, maxBytes+1)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取远程文件内容失败: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("文件大小超过编辑上限 (最大允许 %d MB)", maxBytes/(1024*1024))
	}
	return data, nil
}

// WriteFile 将字节数据全量覆盖写入指定远程文件
func (c *Client) WriteFile(remotePath string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("SFTP 连接已关闭")
	}

	f, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("写入远程文件失败: %w", err)
	}
	return nil
}

// Remove 删除远程文件
func (c *Client) Remove(remotePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.Remove(remotePath)
}

// RemoveDirectory 删除远程空目录
func (c *Client) RemoveDirectory(remotePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.RemoveDirectory(remotePath)
}

// Mkdir 创建远程目录
func (c *Client) Mkdir(remotePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.Mkdir(remotePath)
}

// Rename 重命名或移动远程文件/目录
func (c *Client) Rename(oldPath, newPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.Rename(oldPath, newPath)
}

// Stat 获取远程文件信息
func (c *Client) Stat(remotePath string) (os.FileInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("SFTP 连接已关闭")
	}
	return c.sftpClient.Stat(remotePath)
}

// Close 关闭 SFTP 客户端通道
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.sftpClient != nil {
		return c.sftpClient.Close()
	}
	return nil
}

// FormatSize 将字节格式化为人类可读的字符串（如 1.2 KB, 45.6 MB）
func FormatSize(size int64, isDir bool) string {
	if isDir {
		return "--"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
