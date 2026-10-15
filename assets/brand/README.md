# GoShell Logo

当前应用使用矩形 **GO Terminal** 方案：近乎闭合的矩形外框带出 O 的轮廓，右侧开口向内回折形成 G 的短横，内部保留 `>_` 提示符。配色与 UI 一致，已接入应用侧边栏及系统图标。

原终端框备选素材见 [concepts/terminal](concepts/terminal/README.md)，选定版本的预览归档见 [concepts/terminal-go](concepts/terminal-go/README.md)。

| 文件 | 用途 |
| --- | --- |
| `logo.svg` | 浅色背景上的蓝色 GO Terminal 标志，已接入侧边栏 |
| `logo-dark.svg` | 深色背景上的白色标志 |
| `logo-mono.svg` | 单色标志，通过 `currentColor` 继承颜色 |
| `wordmark.svg` / `wordmark-dark.svg` | 浅色 / 深色背景横版字标，文字已转路径 |
| `app-icon.svg` / `app-icon.png` | 透明边距的圆角应用图标，PNG 为 1024 × 1024 |
| `app-icon-{16,32,64,128,256,512}.png` | 常用尺寸图标 |
| `app-icon.icns` / `app-icon.ico` | macOS / Windows 图标文件 |
| `preview.png` | 深浅底、应用图标和单色版本预览 |

## 配色与留白

- 品牌蓝：`#0366D6`，与 UI 主色一致，用于浅底标志与应用图标底色。
- 正文色：`#111319`，用于浅底字标。
- 白色：`#FFFFFF`，用于深底标志与字标。
- 图形 SVG 为 64 × 64 画布，保留内置留白；外侧建议至少留出 8 个画布单位。最小显示尺寸为 16 px，工具栏建议 24–32 px。
- 横版字标使用较轻的 Go Regular，随 `golang.org/x/image/font/gofont` 提供，已转为路径，不依赖使用者安装字体。

字标字体的授权声明见 `FONT-LICENSE.txt`，分发字标素材时请保留该声明。

## 重新生成

在项目根目录运行：

```sh
go run ./tools/brandgen
```

默认生成已选定的 GO Terminal 方案，写入本目录。生成器共用一套路径定义，直接导出 SVG、PNG、ICO、ICNS 和预览，不需要额外图形软件。调整几何与颜色时修改 `tools/brandgen/main.go`，再运行以上命令。

应用在运行时从嵌入资源设置 Dock 或窗口图标；`mygo.json` 的 `icon` 指向同一份 PNG，MyGo 打包时会将图标写入平台应用包。
