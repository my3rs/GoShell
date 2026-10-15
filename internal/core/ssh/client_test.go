package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"goshell/internal/core/model"
)

func TestSSHClient_WithMockServer(t *testing.T) {
	// 1. 生成服务端临时 RSA 主机私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	serverConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "secret123" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected")
		},
	}
	serverConfig.AddHostKey(signer)

	// 2. 启动本地监听
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	// 后台接收连接并处理 PTY 会话
	go func() {
		nConn, err := listener.Accept()
		if err != nil {
			return
		}
		defer nConn.Close()

		sConn, chans, reqs, err := ssh.NewServerConn(nConn, serverConfig)
		if err != nil {
			return
		}
		defer sConn.Close()
		go ssh.DiscardRequests(reqs)

		for newChannel := range chans {
			if newChannel.ChannelType() != "session" {
				newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
				continue
			}

			ch, requests, err := newChannel.Accept()
			if err != nil {
				return
			}

			go func() {
				defer ch.Close()
				for req := range requests {
					switch req.Type {
					case "pty-req":
						req.Reply(true, nil)
					case "shell":
						req.Reply(true, nil)
						// 写入欢迎提示
						_, _ = ch.Write([]byte("Welcome to Mock SSH Server\n$ "))
					case "window-change":
						req.Reply(true, nil)
					default:
						req.Reply(false, nil)
					}
				}
			}()
		}
	}()

	// 3. 客户端连接测试
	host := &model.Host{
		Address:  "127.0.0.1",
		Port:     port,
		Username: "testuser",
		AuthType: model.AuthTypePassword,
		Password: "secret123",
	}

	client, err := Dial(host)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping test in sandbox: %v", err)
		}
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	// 4. 打开终端通道
	term, err := client.OpenTerminal(80, 24)
	if err != nil {
		t.Fatalf("OpenTerminal failed: %v", err)
	}
	defer term.Close()

	// 测试读取欢迎语
	buf := make([]byte, 64)
	n, err := term.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("term read failed: %v", err)
	}
	if n == 0 {
		t.Fatalf("expected to read output from mock server")
	}

	// 测试窗口大小调整
	if err := term.Resize(100, 30); err != nil {
		t.Fatalf("term resize failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}
