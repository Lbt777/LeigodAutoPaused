package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var (
	cfg = loadConfig()

	idleAnchor = time.Now() // 上次见到游戏或上次暂停的时间，空闲检测按此每 N 分钟循环

	mw        *walk.MainWindow
	accountEd *walk.LineEdit
	passEdit  *walk.LineEdit
	barkEdit  *walk.LineEdit
	tray      *walk.NotifyIcon

	authed   atomic.Bool // 当前有可用的登录态（客户端 token 或账号密码登录）
	exiting  bool
	gameSeen bool
	seenMu   sync.Mutex
	cfgStore atomic.Value
)

const winWidth = 452

func buildUI() error {
	return MainWindow{
		AssignTo: &mw,
		Title:    "雷神自动暂停 " + appVersion,
		Visible:  false,
		Size:     Size{Width: winWidth, Height: 20},
		Layout: VBox{
			Spacing: 8,
			Margins: Margins{Left: 12, Top: 10, Right: 12, Bottom: 10},
		},
		Font: Font{Family: "Microsoft YaHei UI", PointSize: 9},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 4, Spacing: 6},
				Children: []Widget{
					Label{Text: "手机号"},
					LineEdit{AssignTo: &accountEd, MinSize: Size{Width: 140}},
					Label{Text: "密码"},
					LineEdit{AssignTo: &passEdit, PasswordMode: true, MinSize: Size{Width: 120}},
				},
			},

			Composite{
				Layout: Grid{Columns: 2, Spacing: 6},
				Children: []Widget{
					Label{Text: "Bark App Key"},
					LineEdit{AssignTo: &barkEdit, CueBanner: "留空则不推送"},
				},
			},

			Composite{
				Layout: HBox{Spacing: 8},
				Children: []Widget{
					PushButton{Text: "保存", MinSize: Size{Width: 90}, MaxSize: Size{Width: 90}, OnClicked: saveSettings},
				},
			},
		},
	}.Create()
}

func fillUI() {
	_ = accountEd.SetText(cfg.Account)
	_ = passEdit.SetText(cfg.Password)
	_ = barkEdit.SetText(cfg.BarkKey)
}

// 关掉最大化按钮（walk 没有对应 API，直接改窗口样式）
func disableMaximize() {
	const gwlStyle = ^uintptr(15) // GWL_STYLE = -16
	user32 := syscall.NewLazyDLL("user32.dll")
	h := uintptr(mw.Handle())
	style, _, _ := user32.NewProc("GetWindowLongW").Call(h, gwlStyle)
	user32.NewProc("SetWindowLongW").Call(h, gwlStyle, style&^uintptr(0x00010000)) // 去掉 WS_MAXIMIZEBOX
	user32.NewProc("SetWindowPos").Call(h, 0, 0, 0, 0, 0,
		uintptr(0x0001|0x0002|0x0004|0x0020)) // NOSIZE|NOMOVE|NOZORDER|FRAMECHANGED
}

// 窗口尺寸交给布局决定后固定住：既不裁切，也不留拉伸出来的空白
func freezeSize() {
	_ = mw.SetSize(walk.Size{Width: winWidth, Height: 1})
	size := mw.Size()
	_ = mw.SetMinMaxSize(size, size)
}

func showWindow() {
	mw.Synchronize(func() {
		mw.SetVisible(true)
		mw.Activate()
	})
}

func trayTip() string {
	if authed.Load() {
		return "雷神自动暂停 · 已就绪"
	}
	return "雷神自动暂停 · 需要手机号和密码"
}

func afterPause(reason string, ok bool, msg string, needLogin bool) {
	if !ok && needLogin {
		authed.Store(false)
		showWindow()
	}
	c := cfgStore.Load().(Config)
	title := "雷神已自动暂停"
	if !ok {
		title = "雷神暂停失败"
	}
	pushBark(c.BarkKey, title, reason+"\n"+msg+"\n"+time.Now().Format("2006-01-02 15:04:05"), c.TimeSensitive)
}

func manualPause() {
	go func() {
		ok, msg, needLogin := pauseNow(cfgStore.Load().(Config))
		afterPause("手动暂停", ok, msg, needLogin)
	}()
}

// 保存：顺带登录；已登录时只更新配置
func saveSettings() {
	c := cfgStore.Load().(Config)
	c.Account = strings.TrimSpace(accountEd.Text())
	c.Password = strings.TrimSpace(passEdit.Text())
	c.BarkKey = strings.TrimSpace(barkEdit.Text())
	if err := saveConfig(c); err != nil {
		walk.MsgBox(mw, "雷神自动暂停", "保存失败："+err.Error(), walk.MsgBoxIconError)
		return
	}
	cfgStore.Store(c)
	go verifyAndHide(c)
}

func verifyAndHide(c Config) {
	if !authed.Load() {
		if err := loginSession(c); err != nil {
			mw.Synchronize(func() {
				mw.SetVisible(true)
				walk.MsgBox(mw, "雷神自动暂停", "登录失败："+err.Error(), walk.MsgBoxIconWarning)
			})
			return
		}
		authed.Store(true)
	}
	if !c.ready() {
		return // 配置还没填全，留在界面
	}
	mw.Synchronize(func() {
		mw.SetVisible(false)
		if tray != nil {
			_ = tray.SetToolTip(trayTip())
		}
	})
}

func quit() {
	exiting = true
	if tray != nil {
		_ = tray.Dispose()
	}
	mw.Close()
}

func showFromTray() {
	showWindow()
}

func setupTray() bool {
	if tray != nil {
		return true
	}
	icon, err := walk.NewIconFromImage(appIcon())
	if err != nil {
		icon = walk.IconApplication()
	}
	t, err := walk.NewNotifyIcon(mw)
	if err != nil {
		return false // 登录时资源管理器可能还没起来，由调用方重试
	}
	if err := t.SetIcon(icon); err != nil {
		t.Dispose()
		return false
	}
	_ = t.SetToolTip(trayTip())
	if err := t.SetVisible(true); err != nil {
		t.Dispose()
		return false
	}
	tray = t

	tray.MessageClicked().Attach(showFromTray)
	tray.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		switch button {
		case walk.LeftButton:
			showFromTray()
		case walk.RightButton:
			showTrayMenu()
		}
	})
	return true
}

// 托盘建不出来（开机自启时 shell 未就绪）就重试，实在不行把窗口露出来
func setupTrayWithRetry() {
	if setupTray() {
		return
	}
	go func() {
		for i := 0; i < 40; i++ {
			time.Sleep(3 * time.Second)
			ok := false
			mw.Synchronize(func() { ok = setupTray() })
			if ok {
				return
			}
		}
		showWindow()
	}()
}

func toggleAutostart() {
	go func() {
		if autostartEnabled() {
			_ = setAutostart(false)
		} else {
			_ = setAutostart(true)
		}
	}()
}

// 每次启动刷新一次计划任务，保证路径正确（幂等）
func ensureAutostart() { _ = setAutostart(true) }

func appIcon() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 32, G: 120, B: 216, A: 255}}, image.Point{}, draw.Src)
	white := image.NewUniform(color.White)
	draw.Draw(img, image.Rect(9, 9, 14, 23), white, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(18, 9, 23, 23), white, image.Point{}, draw.Src)
	return img
}

func monitorLoop() {
	for {
		time.Sleep(checkSeconds * time.Second)
		c := cfgStore.Load().(Config)
		if processRunning(c.Process) {
			seenMu.Lock()
			gameSeen = true
			seenMu.Unlock()
			idleAnchor = time.Now() // 见到游戏，空闲计时重来
			continue
		}
		seenMu.Lock()
		wasSeen := gameSeen
		gameSeen = false
		seenMu.Unlock()

		// 空闲检测：每 N 分钟都没见到游戏，说明只开了加速器，暂停一次并推送
		if !wasSeen && c.IdlePause &&
			time.Since(idleAnchor) >= time.Duration(c.IdleMinutes)*time.Minute {
			idleAnchor = time.Now()
			ok, msg, needLogin := pauseNow(c)
			afterPause(fmt.Sprintf("空闲检测：已 %d 分钟未运行 %s.exe", c.IdleMinutes, c.Process), ok, msg, needLogin)
			continue
		}
		if !wasSeen {
			continue
		}
		idleAnchor = time.Now() // 游戏退出已经暂停过，空闲计时从此刻重新开始
		ok, msg, needLogin := pauseNow(c)
		afterPause("游戏 "+c.Process+".exe 已退出", ok, msg, needLogin)
	}
}

// 静默确认登录态：客户端 token 优先，其次用保存的账号密码登录
func detectSession() {
	c := cfgStore.Load().(Config)
	if processRunning(c.Process) {
		// 游戏运行中不做暂停验证，避免影响加速
		if clientToken() != "" {
			authed.Store(true)
			return
		}
		if loginSession(c) == nil {
			authed.Store(true)
		}
		return
	}
	if t := clientToken(); t != "" {
		if code, _, err := pauseWithToken(t); err == nil && (code == 0 || code == 400803) {
			storeToken(t)
			authed.Store(true)
			return
		}
	}
	if loginSession(c) == nil {
		authed.Store(true)
	}
}

func main() {
	if !acquireSingleInstance() {
		walk.MsgBox(nil, "雷神自动暂停", "程序已经在运行，请查看任务栏托盘图标。", walk.MsgBoxIconInformation)
		return
	}

	ensureConfigFile()
	cfgStore.Store(cfg)

	if err := buildUI(); err != nil {
		walk.MsgBox(nil, "启动失败", err.Error(), walk.MsgBoxIconError)
		return
	}
	fillUI()
	freezeSize()
	disableMaximize()

	// 布局把窗口撑到最终尺寸后再居中，否则拿到的是初始高度
	centered := false
	mw.SizeChanged().Attach(func() {
		if !centered {
			centered = true
			centerWindow()
		}
	})

	// 点关闭只隐藏，不退出
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if !exiting {
			*canceled = true
			mw.SetVisible(false)
		}
	})

	setupTrayWithRetry()

	seenMu.Lock()
	gameSeen = processRunning(cfg.Process)
	seenMu.Unlock()

	go monitorLoop()
	// 第一次启动/配置没填全，直接把界面露出来，填好保存后才转后台
	if !cfg.ready() {
		mw.SetVisible(true)
	}
	go func() {
		ensureAutostart()
		detectSession()
		if tray != nil {
			_ = tray.SetToolTip(trayTip())
		}
		if !authed.Load() {
			showWindow() // 配置齐全但登录失败时也要能改配置
		}
	}()

	mw.Run()
}
