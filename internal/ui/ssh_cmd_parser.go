package ui

import (
	"strconv"
	"strings"

	"goshell/internal/core/model"
)

// ParseSSHCommand 解析用户输入的 ssh 命令行文本，如：
//   - ssh root@192.168.1.1
//   - ssh -p 2222 user@example.com
//   - ssh user@example.com:2222
//   - ssh 10.0.0.1
// 若不是合法的 ssh 指令，返回 nil, false
func ParseSSHCommand(line string) (*model.Host, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "ssh ") && !strings.HasPrefix(line, "ssh\t") && line != "ssh" {
		return nil, false
	}

	parts := strings.Fields(line)
	if len(parts) == 0 || parts[0] != "ssh" {
		return nil, false
	}

	// 仅输入了 "ssh"
	if len(parts) == 1 {
		return &model.Host{
			Name:        "临时连接",
			Address:     "",
			Port:        22,
			Username:    "root",
			AuthType:    model.AuthTypePassword,
			IsEphemeral: true,
		}, true
	}

	var target string
	var user string
	var port = 22
	var password string

	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		if arg == "-p" && i+1 < len(parts) {
			i++
			if p, err := strconv.Atoi(parts[i]); err == nil && p > 0 && p <= 65535 {
				port = p
			}
		} else if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
			if p, err := strconv.Atoi(arg[2:]); err == nil && p > 0 && p <= 65535 {
				port = p
			}
		} else if arg == "-l" && i+1 < len(parts) {
			i++
			user = parts[i]
		} else if !strings.HasPrefix(arg, "-") && target == "" {
			target = arg
		}
	}

	if target == "" {
		return nil, false
	}

	// 解析 user:password@host:port 或 user@host:port
	if atIdx := strings.Index(target, "@"); atIdx != -1 {
		userInfo := target[:atIdx]
		target = target[atIdx+1:]
		if colonIdx := strings.Index(userInfo, ":"); colonIdx != -1 {
			user = userInfo[:colonIdx]
			password = userInfo[colonIdx+1:]
		} else {
			user = userInfo
		}
	}

	// 解析 host:port
	if colonIdx := strings.LastIndex(target, ":"); colonIdx != -1 {
		hostPart := target[:colonIdx]
		portPart := target[colonIdx+1:]
		if p, err := strconv.Atoi(portPart); err == nil && p > 0 && p <= 65535 {
			target = hostPart
			port = p
		}
	}

	if user == "" {
		user = "root"
	}

	target = strings.Trim(target, "[]'\"")
	if target == "" {
		return nil, false
	}

	return &model.Host{
		Name:        "临时: " + target,
		Address:     target,
		Port:        port,
		Username:    user,
		Password:    password,
		AuthType:    model.AuthTypePassword,
		IsEphemeral: true,
	}, true
}
