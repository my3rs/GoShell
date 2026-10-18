package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"goshell/internal/core/model"
)

// Client 封装了远程主机的 SSH 客户端连接
type Client struct {
	host       *model.Host
	sshClient  *ssh.Client
	stopKeep   chan struct{}
	closeOnce  sync.Once
	mu         sync.Mutex
	isClosed   bool
}

// TerminalConn 包装了终端交互流，实现了 io.ReadWriteCloser 与 Resize(cols, rows int) 接口
// 完美适配 github.com/egoist/mygo/plugins/terminal 的 Options.Conn
type TerminalConn struct {
	session *ssh.Session
	stdin   io.WriteCloser
	reader  io.Reader
	closer  io.Closer

	mu     sync.Mutex
	closed bool
}

// Read 读取远程终端输出
func (t *TerminalConn) Read(p []byte) (n int, err error) {
	return t.reader.Read(p)
}

// Write 向远程终端写入用户按键输入
func (t *TerminalConn) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.stdin == nil {
		return 0, io.EOF
	}
	return t.stdin.Write(p)
}

// Close 关闭终端会话信道
func (t *TerminalConn) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true

	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.session != nil {
		_ = t.session.Close()
	}
	return nil
}

// Resize 窗口变化响应，当终端控件尺寸变化时由 MyGo 自动触发
func (t *TerminalConn) Resize(cols, rows int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.session == nil {
		return nil
	}
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	// crypto/ssh.Session.WindowChange 签名为 (h, w int)，即 (rows, cols)
	return t.session.WindowChange(rows, cols)
}

// Dial 建立到远程主机的 SSH 连接
func Dial(host *model.Host) (*Client, error) {
	if host.Address == "" {
		return nil, errors.New("主机地址不能为空")
	}

	port := host.Port
	if port <= 0 {
		port = 22
	}

	username := host.Username
	if username == "" {
		username = "root"
	}

	authMethods, err := buildAuthMethods(host)
	if err != nil {
		return nil, fmt.Errorf("解析认证凭据失败: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 桌面运维客户端体验优化
		Timeout:         8 * time.Second,
	}

	addr := net.JoinHostPort(host.Address, fmt.Sprintf("%d", port))
	dialer := &net.Dialer{
		Timeout:   config.Timeout,
		KeepAlive: 30 * time.Second,
	}
	tcpConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("无法连接到 %s: %w", addr, err)
	}

	conn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, config)
	if err != nil {
		_ = tcpConn.Close()
		return nil, fmt.Errorf("SSH 协议握手失败: %w", err)
	}

	sshClient := ssh.NewClient(conn, chans, reqs)
	c := &Client{
		host:      host,
		sshClient: sshClient,
		stopKeep:  make(chan struct{}),
	}

	// 启动后台 KeepAlive 心跳，防止被防火墙或 NAT 踢掉
	go c.keepAliveLoop()

	return c, nil
}

// OpenTerminal 创建一个交互式 PTY 终端通道
func (c *Client) OpenTerminal(cols, rows int) (*TerminalConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		return nil, errors.New("SSH 连接已关闭")
	}

	session, err := c.sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建 SSH Session 失败: %w", err)
	}

	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 35
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // 启用回显
		ssh.TTY_OP_ISPEED: 14400, // 输入波特率
		ssh.TTY_OP_OSPEED: 14400, // 输出波特率
	}

	// 请求 xterm-256color 原生 PTY
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("请求 PTY 失败: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("绑定 Stdin 失败: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("绑定 Stdout 失败: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("绑定 Stderr 失败: %w", err)
	}

	// 合并 stdout 与 stderr
	reader := io.MultiReader(stdout, stderr)

	if err := session.Shell(); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("启动远端 Shell 失败: %w", err)
	}

	return &TerminalConn{
		session: session,
		stdin:   stdin,
		reader:  reader,
	}, nil
}

// RawClient 获取底层的 *ssh.Client (供 SFTP 或指标采集使用)
func (c *Client) RawClient() *ssh.Client {
	return c.sshClient
}

// Close 关闭客户端与所有关联信道
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.isClosed = true
		c.mu.Unlock()

		close(c.stopKeep)
		if c.sshClient != nil {
			_ = c.sshClient.Close()
		}
	})
	return nil
}

func (c *Client) keepAliveLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopKeep:
			return
		case <-ticker.C:
			// 发送心跳请求
			_, _, _ = c.sshClient.SendRequest("keepalive@openssh.com", true, nil)
		}
	}
}

func buildAuthMethods(host *model.Host) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	switch host.AuthType {
	case model.AuthTypePassword:
		if host.Password != "" {
			methods = append(methods, ssh.Password(host.Password))
		}
	case model.AuthTypeKeyFile, "key":
		keyPath := host.KeyPath
		if strings.HasPrefix(keyPath, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				keyPath = strings.Replace(keyPath, "~", home, 1)
			}
		}

		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥文件失败 (%s): %w", keyPath, err)
		}

		signer, err := parsePrivateKey(keyData, host.Passphrase)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))

	case model.AuthTypeKeyContent:
		signer, err := parsePrivateKey([]byte(host.KeyContent), host.Passphrase)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))

	default:
		// 尝试密码兜底
		if host.Password != "" {
			methods = append(methods, ssh.Password(host.Password))
		}
	}

	if len(methods) == 0 {
		return nil, errors.New("未提供有效的认证凭据 (密码或私钥)")
	}

	return methods, nil
}

func parsePrivateKey(pemBytes []byte, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passphrase))
	}
	return ssh.ParsePrivateKey(pemBytes)
}
