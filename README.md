# 快传助手（文件传输助手）
https://wwbed.lanzoup.com/iy29y4cyk2ud
密码:bwp4

局域网内 **电脑 ↔ 手机** 互传文件的超轻量工具：单文件 exe（约 1.7MB），免安装，双击即用。

## 特性
- 电脑把文件放进「快传」文件夹，手机浏览器扫码 / 输网址即可下载
- 手机也能传回电脑（分块上传，支持大视频）
- 批量下载、自动接收、进度提示
- 免密零操作，局域网直连
- 每次启动随机端口，防固定端口扫描
- 单文件、绿色、无后台驻留
<img width="491" height="598" alt="image" src="https://github.com/user-attachments/assets/5be1b482-af4d-46d5-9b0c-22c29ae73e87" />
<img width="1220" height="2712" alt="image" src="https://github.com/user-attachments/assets/99fce7a1-f27a-493d-8f18-571384eb5db6" />

## 使用方法
1. 双击 `快传助手.exe`（Windows 若提示未知发布者，点「更多信息」→「仍要运行」）
2. 手机连同一个 WiFi，用浏览器打开控制台显示的网址（或扫二维码）
3. 电脑 ↔ 手机互传

详细步骤见「使用说明.txt」。

## 从源码构建
需要 Go 工具链。在 `gosrc` 目录下执行：

```
go build -ldflags "-s -w" -o ..\快传助手.exe .
```

可选 UPX 压缩体积：

```
upx --lzma --best 快传助手.exe
```

## 安全说明
- **仅局域网**：公网默认访问不到（除非路由器做了端口映射）
- **免密**：能连上你 WiFi 的设备即可访问（安全靠 WiFi 密码本身）
- **随机端口**：每次启动换端口，降低被固定扫描的风险

## 目录结构
- `gosrc/main.go` —— 全部源码（Go 单文件实现）
- `快传助手.exe` —— 已编译成品（Windows x64）

## 免责声明
本工具按「现状」免费提供，仅供学习交流与个人使用；请勿用于任何非法用途，使用者需自行承担使用风险。

## 随意打赏
<img width="1080" height="1620" alt="750e21d0bb7faacde5262599c8b04df1" src="https://github.com/user-attachments/assets/e6d76860-7c37-45a1-88af-d3af9a5dd6b3" />
