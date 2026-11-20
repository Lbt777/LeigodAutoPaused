# 雷神自动暂停（Leigod Auto Paused）

检测 PUBG（`TslGame.exe`）退出后，自动调用雷神加速器的暂停接口，并通过 Bark 推送到手机。

用 Go + walk 写的单文件 Windows 桌面程序，常驻托盘，无控制台窗口。

## 功能

- 每 5 秒检测一次 `TslGame.exe`，检测到退出后自动暂停加速
- **登录态**：优先使用雷神客户端本地写入的 token；被服务端判为过期时，自动用保存的手机号/密码重新登录（无感）
- **Bark 推送**：只填设备 Key 即可，暂停成功/失败都会通知
- **常驻托盘**：点关闭只是隐藏窗口；右键菜单提供 打开设置 / 立即暂停 / 开机启动 / 退出
- **开机自启**：首次运行自动创建计划任务（登录 Windows 后启动），托盘菜单可勾选取消
- **本地安全**：手机号和密码用 Windows DPAPI（当前用户）加密后存盘，明文不落盘
- 单实例运行，重复启动会提示已在运行

## 界面

三个输入框：手机号 / 密码 / Bark Key，一个「保存」按钮。

点保存 = 保存配置 + 校验登录（已登录时只保存 Bark Key）；保存成功后窗口自动转入托盘后台运行。

## 构建

要求 Windows 64 位 + Go 1.24 及以上。

```bash
go build -trimpath -ldflags "-s -w -H windowsgui" -o LeishenPause.exe .
```

`rsrc.syso` 已随仓库提交（窗口控件需要 Common-Controls 6.0 的 manifest，缺失会启动失败）。如果你改了 `app.manifest`，重新生成：

```bash
go install github.com/akavel/rsrc@v0.10.2
rsrc -manifest app.manifest -o rsrc.syso -arch amd64
```

## 配置

配置保存在 `%APPDATA%\LeishenAutoPause\config.json`：

```json
{
  "IntervalSeconds": 5,
  "GameProcess": "TslGame",
  "BarkUrl": "https://api.day.app/<设备Key>",
  "Account": "手机号",
  "Password": "DPAPI 加密后的 hex"
}
```

`IntervalSeconds` / `GameProcess` 为固定内置值（5 秒、TslGame），仅保留字段以兼容旧配置。

## 代码结构

| 文件 | 作用 |
| --- | --- |
| `main.go` | 窗口、托盘、进程监控循环 |
| `api.go` | 登录 / 暂停 / Bark 推送接口 |
| `token.go` | 读取雷神客户端本地登录 token |
| `config.go` | 配置读写、DPAPI 加解密、自启、单实例 |

## 说明

- 接口签名常量来自雷神官网前端页面自身使用的公开常量，用于复现官网自己的请求格式，不包含任何账号或个人密钥。
- 本项目仅供个人学习使用，请遵守雷神加速器的服务条款；使用产生的账号风险由使用者自行承担。
