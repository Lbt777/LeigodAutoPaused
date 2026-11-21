package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var procAppendMenu = syscall.NewLazyDLL("user32.dll").NewProc("AppendMenuW")

const (
	menuIDSettings  = 1
	menuIDPause     = 2
	menuIDAutostart = 3
	menuIDQuit      = 4
	menuIDIdle      = 5
	menuIDLevel     = 6
	idleMinuteBase  = 100 // + 分钟数，区分空闲检测时长选项
)

func appendMenu(hMenu win.HMENU, flags uint32, id uintptr, text string) {
	p, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procAppendMenu.Call(uintptr(hMenu), uintptr(flags), id, uintptr(unsafe.Pointer(p)))
}

// 自己弹右键菜单：walk 内置的托盘菜单在主窗口隐藏时第一次点击会被吞掉
func showTrayMenu() {
	h := win.CreatePopupMenu()
	if h == 0 {
		return
	}
	defer win.DestroyMenu(h)

	c := cfgStore.Load().(Config)
	appendMenu(h, win.MF_STRING|win.MF_GRAYED, 0, "监控进程："+c.Process)
	appendMenu(h, win.MF_STRING, menuIDSettings, "打开设置")
	appendMenu(h, win.MF_STRING, menuIDPause, "立即暂停")
	appendMenu(h, checkFlags(autostartEnabled()), menuIDAutostart, "开机启动")

	appendMenu(h, checkFlags(c.IdlePause), menuIDIdle, "空闲检测")
	sub := win.CreatePopupMenu()
	for _, m := range []int{1, 3, 5, 10, 15, 30} {
		appendMenu(sub, checkFlags(c.IdleMinutes == m), uintptr(idleMinuteBase+m), fmt.Sprintf("%d 分钟", m))
	}
	appendMenu(h, win.MF_POPUP, uintptr(sub), "空闲检测时长")
	appendMenu(h, checkFlags(c.TimeSensitive), menuIDLevel, "时效性通知")

	appendMenu(h, win.MF_SEPARATOR, 0, "")
	appendMenu(h, win.MF_STRING, menuIDQuit, "退出")

	defer win.DestroyMenu(sub)

	hWnd := mw.Handle()
	win.SetForegroundWindow(hWnd)
	var p win.POINT
	win.GetCursorPos(&p)
	id := int(win.TrackPopupMenuEx(h, win.TPM_RETURNCMD|win.TPM_RIGHTBUTTON, p.X, p.Y, hWnd, nil))
	win.PostMessage(hWnd, win.WM_NULL, 0, 0) // 菜单关掉后把前台还回去

	switch {
	case id == menuIDSettings:
		showFromTray()
	case id == menuIDPause:
		manualPause()
	case id == menuIDAutostart:
		toggleAutostart()
	case id == menuIDIdle:
		updateConfig(func(c *Config) { c.IdlePause = !c.IdlePause })
	case id > idleMinuteBase:
		updateConfig(func(c *Config) { c.IdleMinutes = id - idleMinuteBase })
	case id == menuIDLevel:
		updateConfig(func(c *Config) { c.TimeSensitive = !c.TimeSensitive })
	case id == menuIDQuit:
		quit()
	}
}

func checkFlags(on bool) uint32 {
	f := uint32(win.MF_STRING)
	if on {
		f |= win.MF_CHECKED
	}
	return f
}

// 主窗口在屏幕居中显示（按真实窗口矩形算，布局完成后再调用）
func centerWindow() {
	var r win.RECT
	if !win.GetWindowRect(mw.Handle(), &r) {
		return
	}
	w := int(r.Right - r.Left)
	h := int(r.Bottom - r.Top)
	sw := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	sh := int(win.GetSystemMetrics(win.SM_CYSCREEN))
	x := int32((sw - w) / 2)
	y := int32((sh - h) / 2)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	win.SetWindowPos(mw.Handle(), 0, x, y, 0, 0, win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
}
