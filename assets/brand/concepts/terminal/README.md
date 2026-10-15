# GoShell · Open Terminal 备选方案

新的方向使用带开口的终端框，以及清晰的 `>_` 命令提示符，不再使用 G 字母。开口与留白让符号更轻，配色与现有界面保持一致：品牌蓝 `#0366D6`、正文色 `#111319`、白色 `#FFFFFF`。

此目录为方案预览与独立素材，不修改当前应用已接入的 Logo。

- `preview.png`：深浅底字标、桌面图标与单色预览。
- SVG、PNG、ICO、ICNS：与主方案相同的完整输出规格。
- 字标使用转为路径的 Go Regular，授权声明见 `../../FONT-LICENSE.txt`。

在项目根目录重新生成：

```sh
go run ./tools/brandgen -concept terminal
```
