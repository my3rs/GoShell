# GoShell · GO Terminal 备选方案

沿用 Open Terminal 的矩形比例、圆角、`>_` 提示符和 UI 蓝色 `#0366D6`。外框补全右上角，仅在右侧保留一道开口；开口下沿向内回折，形成 G 的短横，整体近乎闭合的矩形又带出 O 的轮廓。

该方案已选定并接入应用，正式素材位于 `assets/brand`。本目录保留设计预览与归档，原 Open Terminal 方案保留在相邻的 `terminal` 目录。

- `preview.png`：深浅底字标、桌面图标及单色预览。
- SVG、PNG、ICO、ICNS：与主方案相同的完整输出规格。
- 字标使用转为路径的 Go Regular，授权声明见 `../../FONT-LICENSE.txt`。

在项目根目录重新生成：

```sh
go run ./tools/brandgen -concept terminal-go -out assets/brand/concepts/terminal-go
```
