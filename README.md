# 雷神自动暂停（Leigod Auto Paused）

**默认针对 PUBG（`TslGame.exe`）**：检测游戏退出后，自动调用雷神加速器的暂停接口，并通过 Bark 推送到手机。

Go + walk 写的单文件 Windows 桌面程序，常驻托盘，无控制台窗口。

## 功能

- 默认监控 PUBG 的 `TslGame.exe`，每 5 秒检测一次；检测到从运行变为未运行后自动暂停加速
- **登录态**：优先使用雷神客户端本地写入的 token；被服务端判为过期时，自动用配置里的账号密码重新登录
- **Bark 推送**：只填设备 Key，暂停成功/失败都会推送到手机；留空则不推送
- **常驻托盘**：点关闭只是隐藏窗口。右键菜单：
  - `监控进程：TslGame`（灰色，仅展示当前监控的进程名）
  - 打开设置 / 立即暂停
  - 开机启动（勾选状态，取消勾选即删除自启）
  - 退出
- **开机自启**：首次运行自动创建计划任务（登录 Windows 后启动），路径变化时自动刷新
- 单实例运行

## 配置文件

路径（每个用户自己的目录）：

```
%APPDATA%\LeishenAutoPause\config.ini
```

一般是 `C:\Users\你的用户名\AppData\Roaming\LeishenAutoPause\config.ini`。首次启动会自动生成一份空模板：

```ini
; 雷神自动暂停 · 本地配置（仅保存在本机，不会上传）
游戏进程名=TslGame
账号=
密码=
BarkAppKey=
```

| 字段 | 说明 |
| --- | --- |
| `游戏进程名` | 要监控的游戏进程名，**不带 `.exe`**，默认 `TslGame`（PUBG） |
| `账号` | 雷神加速器登录手机号 |
| `密码` | 雷神加速器密码，明文保存 |
| `BarkAppKey` | Bark App 首页复制的设备 Key，留空则不推送 |

**怎么改**：

1. 托盘右键 →「退出」，先关掉正在运行的程序；
2. 用记事本编辑上面的 `config.ini`，改完保存；
3. 重新双击 exe 启动。

也可以只在界面里改「手机号 / 密码 / Bark App Key」三项后点保存；进程名只能通过 `config.ini` 修改。

**隐私**：账号和密码只以明文保存在你自己电脑上的这个 `config.ini` 里，不会上传到任何地方，仓库里也不会包含它（仓库只带一份空值的 `config.example.ini`）。程序访问网络仅限雷神登录/暂停接口和 Bark 推送。

## 首次启动

配置没填全（账号、密码、BarkAppKey 任一为空）时，启动会显示主界面让你补齐；三项齐全并保存后，程序自动转入托盘后台运行，以后启动都直接后台。

## 换成其他游戏

把 `config.ini` 里的 `游戏进程名` 改成目标游戏的进程名（不带 `.exe`），重启程序即可，**不用重新打包**。例如：

- 永劫无间：`NarakaBladepoint`
- APEX：`r5apex`
- 战地 1：`bf1`
- 原神：`YuanShen`

确认进程名的方法：游戏运行时打开任务管理器 →「详细信息」→ 看可执行文件名，去掉 `.exe`；或在 PowerShell 里执行：

```powershell
Get-Process | Where-Object { $_.MainWindowTitle } | Select-Object ProcessName
```

> 逻辑是「监控的进程从存在变为不存在时暂停加速」，进程名填错就永远不会触发。想改检测间隔，改 `config.go` 里的 `checkSeconds` 后重新打包。

## 构建

要求 Windows 64 位 + Go 1.24 及以上。

```bash
go build -trimpath -ldflags "-s -w -H windowsgui" -o LeishenPause.exe .
```

`rsrc.syso` 已随仓库提交（窗口控件需要 Common-Controls 6.0 的 manifest，缺失会启动失败）。如果改了 `app.manifest`，重新生成：

```bash
go install github.com/akavel/rsrc@v0.10.2
rsrc -manifest app.manifest -o rsrc.syso -arch amd64
```

## 代码结构

| 文件 | 作用 |
| --- | --- |
| `main.go` | 窗口、托盘、进程监控循环 |
| `api.go` | 登录 / 暂停 / Bark 推送接口 |
| `token.go` | 读取雷神客户端本地登录 token |
| `config.go` | `config.ini` 读写、进程检测、自启、单实例 |

## 说明

- 接口签名常量来自雷神官网前端页面自身使用的公开常量，用于复现官网自己的请求格式，不包含任何账号或个人密钥。
- 本项目仅供个人学习使用，请遵守雷神加速器的服务条款；使用产生的账号风险由使用者自行承担。
