# GoShell 图标

当前使用确认过的 **D04「提示符眼神」**：黑底白色小幽灵从底部冒头，左眼为 `>`，右眼为竖线光标。系统图标、侧边栏、关于页和项目字标使用同一母版。

## 原图与备选

| 文件 | 用途 |
| --- | --- |
| `source/d04.png` | 当前 D04 原始母版，保持原图不变 |
| `source/r1d.png` | 保留的 R1D「低处冒头」原始版本，不参与默认生成 |
| `app-icon.png` | 1024 × 1024 应用图标，嵌入程序并用于 MyGo 打包 |
| `app-icon-{16,32,64,128,256,512}.png` | 常用尺寸 |
| `app-icon.icns` / `app-icon.ico` | macOS / Windows 平台图标 |
| `app-icon.svg` | 内嵌同款 PNG 的 SVG，不是重新描摹的矢量 |
| `logo.svg` / `logo-dark.svg` / `logo-mono.svg` | 保留原有文件名的同款黑白标识，内嵌 PNG，颜色固定 |
| `wordmark.svg` / `wordmark-dark.svg` | 图标与 GoShell 字标，字体继续使用转为路径的 Go Regular |
| `preview.png` | 深浅底字标与常用尺寸预览 |

## 生成方式

在项目根目录执行：

```sh
go run ./tools/brandgen
go test ./tools/brandgen
```

生成器仅对确认的 PNG 母版进行缩放与平台封装，不重新绘制角色或表情。桌面图标保留现有的超椭圆外形和主体占比：1024 画布内主体为 824 × 824，四边透明留白各 100 px。小幽灵在母版中的低处冒头构图保持不变。

需要预览保留的 R1D 时，可以单独输出到其他目录，不覆盖当前图标：

```sh
go run ./tools/brandgen -source assets/brand/source/r1d.png -out bin/brand-r1d
```

SVG 使用内嵌 PNG，查看器需要支持 SVG 的 `image` 元素。MyGo 的 SVG 解析器不支持内嵌位图，因此应用界面直接解码共享的 `brand.IconPNG`，与 Dock、窗口图标保持一致。

`mygo.json`、Windows 资源生成和发布工作流继续使用本目录的既有文件路径。R1D 原图不会被重新生成命令覆盖。

字标字体的授权说明保留于 `FONT-LICENSE.txt`，分发字标时请保留该文件。
