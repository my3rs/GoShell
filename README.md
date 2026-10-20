<p align="center">
  <img src="assets/brand/app-icon.png" width="96" height="96" alt="GoShell Logo" />
</p>

<h1 align="center">GoShell</h1>

<p align="center">
  <strong>基于 Go 与 MyGo 构建的轻量跨平台桌面 SSH 终端与主机管理工具</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/UI-MyGo-0366D6?style=flat-square" alt="MyGo UI" />
  <img src="https://img.shields.io/badge/Engine-Ghostty--VT-FF5722?style=flat-square" alt="Ghostty-VT" />
  <img src="https://img.shields.io/badge/Platform-macOS%20|%20Windows%20|%20Linux-lightgrey?style=flat-square" alt="Platform" />
</p>

---

## 项目简介

GoShell 是一个跨平台桌面终端与远程主机管理工具。采用 Go 语言结合桌面框架 MyGo 开发，底层集成 Ghostty-VT 终端仿真引擎。程序启动迅速，资源占用低，专注于提供简洁高效的日常运维与本地开发体验。

---

## 主要特性

- **多标签与多分屏**：支持单标签、双分屏与四宫格视窗布局，可在同一窗口内并排操作多个会话。
- **内置本地终端**：启动即用本地命令行，Windows 支持 BusyBox、CMD 与 PowerShell 切换，macOS 与 Linux 接入原生 Shell。
- **SSH 主机管理**：支持服务器分组归类与凭据保存，支持直接输入标准 SSH 命令快速连接。
- **文件管理与在线编辑**：集成 SFTP 远程文件浏览、上传下载以及远程文件就地编辑保存。
- **安全本地存储**：服务器配置与登录凭据全部加密保存在本机，不上传云端。
- **独立便携运行**：内嵌核心依赖，解压即用，无外部依赖。

---

## 快速上手

### 环境要求

- Go 1.24 或更高版本
- 具备标准构建工具 make

### 本地运行

在项目根目录下执行以下命令即可启动开发版应用：

```bash
make run
```

---

## 构建与打包

项目使用 Makefile 管理各平台的构建流程：

```bash
make help
```

### 本地编译
```bash
make build
```
编译产物输出至 `bin/goshell`。

### Windows 编译与打包
```bash
# 编译 64 位图形界面程序
make build-windows

# 编译 ARM64 架构程序
make build-windows-arm64

# 打包为离线压缩包，内含主程序与配套运行库
make package-windows
```

打包后，`bin/GoShell-windows-amd64.zip` 始终更新为本次构建，同时保留带版本号的压缩包。Windows EXE 的文件图标在构建时由 `assets/brand/app-icon.ico` 嵌入；更新图标后需重新打包并使用新产物。

### macOS 编译
```bash
make build-darwin
```
编译产物输出至 `bin/GoShell-darwin` 及对应动态库。

### Linux 编译
```bash
make build-linux
```
编译产物输出至 `bin/GoShell-linux` 及对应动态库。

### 更新原生引擎库
如需重新下载各操作系统适配的终端引擎动态库，可执行：
```bash
make fetch-natives
```

---

## 项目结构

```text
goshell/
├── cmd/
│   └── goshell/             # 桌面程序入口，负责窗口生命周期与事件循环
├── docs/
│   └── ui_spec.md           # 界面与弹窗设计规范
├── internal/
│   ├── app/                 # 应用程序主状态与数据初始化
│   ├── core/                # 底层核心服务
│   │   ├── model/           # 数据结构定义
│   │   ├── security/        # 密钥管理与加解密逻辑
│   │   ├── storage/         # 本地数据库持久化
│   │   ├── ssh/             # SSH 连接与会话控制
│   │   ├── sftp/            # SFTP 远程文件操作
│   │   └── transfer/        # 后台文件传输调度
│   └── ui/                  # 界面视图实现
│       ├── app_view.go      # 主界面布局与事件调度
│       ├── session.go       # 终端会话与渲染逻辑
│       ├── editor_tab.go    # 远程文件文本编辑器
│       ├── ssh_cmd_parser.go# SSH 命令行解析
│       └── theme.go         # 界面主题与样式常量
├── assets/
│   ├── brand/               # 品牌视觉素材与图标
│   ├── fonts/               # 终端字体资源
│   └── natives/             # 各系统终端引擎动态库与命令行工具
├── tools/
│   ├── brandgen/            # 图标资源生成脚本
│   └── fetch_natives/       # 动态库同步脚本
├── Makefile                 # 编译构建指令
├── mygo.json                # 桌面应用配置清单
└── go.mod                   # 模块依赖描述
```

---

## 开发与设计规范

- [UI 设计规范与弹窗标准](docs/ui_spec.md)：详细阐述了桌面界面的文字阶梯、弹窗尺寸分类、表单间距节奏与统一错误提示组件。

---

## 数据安全说明

GoShell 的全部配置与连接信息仅保存在用户本地设备中：

- 数据存储路径为 `~/.goshell/`，其中包含数据库文件 `goshell.db` 和密钥文件 `master.key`。
- 服务器登录密码与私钥等凭据均通过本地密钥加密存储。若密钥文件丢失，将无法解密先前保存的凭据，请注意妥善保存该目录。

