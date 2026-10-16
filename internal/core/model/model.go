package model

import "time"

// AuthType 定义 SSH 认证类型
type AuthType string

const (
	AuthTypePassword   AuthType = "password"    // 密码认证
	AuthTypeKeyFile    AuthType = "key_file"    // 本地私钥文件
	AuthTypeKeyContent AuthType = "key_content" // 直接存储私钥文本
	AuthTypeAgent      AuthType = "agent"       // SSH-Agent 认证
	AuthTypeLocal      AuthType = "local"       // 本地终端测试
)

// Group 主机分组
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parent_id,omitempty"` // 支持嵌套子分组
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Host 远程主机连接配置
type Host struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"group_id"`    // 所属分组 ID，为空时属于默认未分组
	Name        string    `json:"name"`        // 显示别名，例如 "Prod-Web-01"
	Address     string    `json:"address"`     // 主机 IP 或域名
	Port        int       `json:"port"`        // SSH 端口，默认 22
	Username    string    `json:"username"`    // 登录用户名，默认 root
	AuthType    AuthType  `json:"auth_type"`   // 认证类型
	Password    string    `json:"-"`           // 登录密码（内存明文，落库时加密）
	KeyPath     string    `json:"key_path"`    // 私钥文件路径
	KeyContent  string    `json:"-"`           // 私钥内容（内存明文，落库时加密）
	Passphrase  string    `json:"-"`           // 私钥口令（内存明文，落库时加密）
	JumpHostID  string    `json:"jump_host_id"`// 跳板机 Host ID（链式跳板）
	Description string    `json:"description"` // 备注信息
	ColorTag    string    `json:"color_tag"`   // 标签颜色 (用于终端标签和侧边栏标识)
	ShellType   string    `json:"shell_type,omitempty"` // 本地终端指定的 Shell 类型 (auto/busybox/powershell/cmd)
	IsEphemeral bool      `json:"is_ephemeral,omitempty"` // 临时会话，不持久化保存到数据库
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// QuickCommand 快捷运维指令
type QuickCommand struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	Title     string    `json:"title"`   // 标题，例如 "实时查看 Nginx 日志"
	Command   string    `json:"command"` // 具体命令，例如 "tail -f /var/log/nginx/access.log"
	CreatedAt time.Time `json:"created_at"`
}

// Setting 全局键值配置
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
