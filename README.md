# 雷神自动暂停（Leigod Auto Paused）

**针对 PUBG（`TslGame.exe`）**：检测游戏退出后，自动调用雷神加速器的暂停接口，并通过 Bark 推送到手机。

用 Go + walk 写的单文件 Windows 桌面程序，常驻托盘，无控制台窗口。

> 只监控 `TslGame.exe`。要监控别的游戏，见下方「换成其他游戏」。

## 功能

- 每 5 秒检测一次 `TslGame.exe`，检测到退出后自动暂停加速
- **登录态**：优先使用雷神客户端本地写入的 token；被服务端判为过期时，自动用保存的手机号/密码重新登录（无感）
- **Bark 推送**：只填设备 Key 即可，暂停成功/失败都会通知
- **常驻托盘**：点关闭只是隐藏窗口；右键菜单提供 打开设置 / 立即暂停 / 开机启动 / 退出
- **开机自启**：首次运行自动创建计划任务（登录 Windows 后启动），托盘菜单可勾选取消
- **本地安全**：手机号和密码用 Windows DPAPI（当前用户）加密后存盘，明文不落盘
- 单实例运行，重复启动会提示已在运行

## 界面

三个输入框：手机号 / 密码 / Bark App Key，一个「保存」按钮。

点保存 = 保存配置 + 校验登录（已登录时只保存 Bark App Key）；保存成功后窗口自动转入托盘后台运行。

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

构建产物就在当前目录，双击即可运行（无控制台窗口）。

## 换成其他游戏

程序把游戏进程名写死在 `config.go` 里，默认 `TslGame`（PUBG）。换游戏只要改一行再重新打包：

1. 打开 `config.go`，找到：

   ```go
   const (
       gameProcess  = "TslGame"
       checkSeconds = 5
   )
   ```

   - `gameProcess`：目标游戏的进程名，**不带 `.exe`**，例如：
     - 永劫无间：`NarakaBladepoint`
     - APEX：`r5apex`
     - 战地：`bf1`
     - 原神：`YuanShen`
   - `checkSeconds`：检测间隔秒数，默认 5 秒

   进程名可以这样确认：游戏运行时打开任务管理器 →「详细信息」→ 看可执行文件名字，去掉 `.exe` 即可；也可以在 PowerShell 里跑
   `Get-Process | Where-Object { $_.MainWindowTitle } | Select-Object ProcessName`。

2. 重新打包：

   ```bash
   go build -trimpath -ldflags "-s -w -H windowsgui" -o LeishenPause.exe .
   ```

   想改程序名/窗口标题就一起改 `main.go` 里的 `Title` 和输出文件名。

3. 用新 exe 替换旧文件，然后：
   - 托盘右键 →「退出」，关掉正在运行的旧实例；
   - 双击新 exe 启动（会自动刷新开机自启任务的路径指向新文件）。

> 注意：程序逻辑是「监控的进程从运行变为不存在时暂停加速」，所以填的进程名必须和游戏真实进程名一致，否则永远不会触发暂停。

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
