# GoShell Logo

外圈字母 **G** 代表 GoShell，内部 **>** 是终端提示符。G 使用圆形字腹、圆润收口和弧形底部；内部提示符放大并加粗，G 的横笔缩短，为终端符号留出独立空间，兼顾柔和轮廓与小尺寸辨识度。

| 文件 | 用途 |
| --- | --- |
| `logo.svg` | 浅色背景上的蓝色标志，已接入应用顶部 |
| `logo-dark.svg` | 深色背景上的白色标志，青色提示符 |
| `logo-mono.svg` | 单色标志，通过 `currentColor` 继承颜色 |
| `wordmark.svg` / `wordmark-dark.svg` | 浅色 / 深色背景横版字标，文字已转路径 |
| `app-icon.svg` / `app-icon.png` | 透明边距的圆角应用图标，PNG 为 1024 × 1024 |
| `app-icon-{16,32,64,128,256,512}.png` | 常用尺寸图标 |
| `app-icon.icns` / `app-icon.ico` | macOS / Windows 图标文件 |
| `preview.png` | 深浅底、应用图标和单色版本预览 |

## 配色与留白

- 品牌蓝：`#0366D6`，与项目主题主色一致。
- 提示符青：`#67E8F9`，用于深色和应用图标版本。
- 字标墨色：`#111319`；深色背景使用白色。
- 图形 SVG 为 64 × 64 画布，保留内置留白；外侧建议至少留出 8 个画布单位。最小显示尺寸为 16 px，工具栏建议 24–32 px。
- 横版字标使用 Go Medium，随 `golang.org/x/image/font/gofont` 提供，已转为路径，不依赖使用者安装字体。

字标字体的授权声明见 `FONT-LICENSE.txt`，分发字标素材时请保留该声明。

## 重新生成

在项目根目录运行：

```sh
go run ./tools/brandgen
```

生成器共用一套路径定义，直接导出 SVG、PNG、ICO、ICNS 和预览，不需要额外图形软件。调整几何与颜色时修改 `tools/brandgen/main.go`，再运行以上命令。

应用在运行时从嵌入资源设置 Dock 或窗口图标；`mygo.json` 的 `icon` 指向同一份 PNG，MyGo 打包时会将图标写入平台应用包。
