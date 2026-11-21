# 雷神自动暂停（Leigod Auto Paused）

**默认针对 PUBG（`TslGame.exe`）**：游戏退出后自动暂停雷神加速器，并通过 Bark 推送到手机。

Go + walk 编写的单文件 Windows 程序，常驻托盘，无控制台窗口。

## 功能

- **隐私**：账号和密码只以明文保存在你自己电脑上的 `config.ini` 里，不会上传到任何地方，仓库里也不会包含它（仓库只带一份空值的 `config.example.ini`）。程序访问网络仅限雷神登录/暂停接口和 Bark 推送。
- **自动暂停**：每 5 秒检测一次 `TslGame.exe`，检测到游戏退出后调用暂停接口
- **登录态**：优先用雷神客户端本地写入的 token；被服务端判过期时，自动用配置里的账号密码重新登录
- **Bark 推送**：只需填设备 Key，暂停成功/失败都会推送；留空则不推送。内置铃声 `glass`，可选时效性通知（专注模式下也能显示）
- **空闲检测**（默认关闭）：启动后 N 分钟仍没检测到游戏进程，说明只开了加速器，自动暂停并推送；触发后本轮不再空闲检测，等游戏运行过并退出后重新启用，如此循环
- **常驻托盘**：点关闭只是隐藏窗口。右键菜单：
  - `监控进程：TslGame` —— 灰色只读，展示当前监控的进程名
  - 打开设置 / 立即暂停
  - 开机启动 —— 勾选即开启，取消勾选即删除自启
  - 空闲检测 —— 子菜单：关闭 / 1 / 3 / 5 / 10 / 15 / 30 分钟，勾选当前值（选分钟即开启）
  - 时效性通知 —— 勾选后 Bark 用 `level=timeSensitive`
  - 退出
- **开机自启**：首次运行自动写入注册表 `HKCU\...\Run`，exe 路径变化时自动刷新
- 单实例运行，重复启动会提示

## 配置（本地 config.ini）

配置文件路径：

```
%APPDATA%\LeishenAutoPause\config.ini
```

一般是 `C:\Users\你的用户名\AppData\Roaming\LeishenAutoPause\config.ini`，首次启动会自动生成：

```ini
; 雷神自动暂停 · 本地配置（仅保存在本机，不会上传）
游戏进程名=TslGame
账号=
密码=
BarkAppKey=
空闲检测=关
空闲检测分钟=3
时效性通知=关
```

| 字段 | 说明 |
| --- | --- |
| `游戏进程名` | 要监控的游戏进程名，**不带 `.exe`**，默认 `TslGame`（PUBG） |
| `账号` | 雷神加速器登录手机号 |
| `密码` | 雷神加速器密码，明文 |
| `BarkAppKey` | Bark App 首页复制的设备 Key，**选填**：填了才推送，留空只是不推送 |
| `空闲检测` | `开` / `关`，默认关：N 分钟内没检测到游戏进程就自动暂停一次；触发后等游戏退出才重新启用 |
| `空闲检测分钟` | 空闲检测的等待分钟数，默认 `3` |
| `时效性通知` | `开` / `关`，默认关：开时 Bark 推送 `level=timeSensitive`，专注模式下也能显示 |

修改步骤：

1. 托盘右键 →「退出」，关闭正在运行的程序；
2. 用记事本编辑 `config.ini` 并保存；
3. 重新双击 exe 启动。

界面里的「手机号 / 密码 / Bark App Key」也可以直接改并点保存；进程名只能通过 `config.ini` 修改。

## 首次启动

账号或密码没填时，启动会显示主界面让你补齐；填好并保存后自动转入托盘后台，以后启动都直接后台运行。`BarkAppKey` 是选填项，不影响是否后台运行——填了就推送，不填就不推送。

## 换成其他游戏

把 `config.ini` 里的 `游戏进程名` 改成目标游戏的进程名（不带 `.exe`），重启程序即可，**不需要重新打包**。例如：

- 永劫无间：`NarakaBladepoint`
- APEX：`r5apex`
- 战地 1：`bf1`
- 原神：`YuanShen`

确认进程名的方法：游戏运行时打开任务管理器 →「详细信息」→ 看可执行文件名，去掉 `.exe`；或在 PowerShell 执行：

```powershell
Get-Process | Where-Object { $_.MainWindowTitle } | Select-Object ProcessName
```

> 逻辑是「监控的进程从存在变为不存在时暂停加速」，进程名填错就永远不会触发。想改检测间隔，修改 `config.go` 里的 `checkSeconds` 后重新打包。

## 构建

要求 Windows 64 位 + Go 1.24 及以上：

```bash
go build -trimpath -ldflags "-s -w -H windowsgui" -o LeishenPause.exe .
```

`rsrc.syso` 已随仓库提交（窗口控件需要 Common-Controls 6.0 的 manifest，缺失会启动失败）。若修改了 `app.manifest`，重新生成：

```bash
go install github.com/akavel/rsrc@v0.10.2
rsrc -manifest app.manifest -o rsrc.syso -arch amd64
```

## 版本与打包

- 版本号两位（如 `1.0`、`1.1`），显示在窗口标题后面：`雷神自动暂停 1.0`。
- 用 `build.ps1` 打包：按当前版本号生成 `LeishenPause-<版本>.exe`，然后把 `version.go` 里的版本号 +0.1，下次打包就是新版本。

## 代码结构

| 文件 | 作用 |
| --- | --- |
| `main.go` | 窗口、托盘、进程监控循环 |
| `api.go` | 登录 / 暂停 / Bark 推送接口 |
| `token.go` | 读取雷神客户端本地登录 token |
| `config.go` | `config.ini` 读写、进程检测、自启、单实例 |
| `version.go` | 版本号（由 `build.ps1` 递增） |
| `build.ps1` | 打包脚本 |

## 说明

- 接口签名常量来自雷神官网前端页面自身使用的公开常量，用于复现官网自己的请求格式，不包含任何账号或个人密钥。
- 本项目仅供个人学习使用，请遵守雷神加速器的服务条款；使用产生的账号风险由使用者自行承担。
