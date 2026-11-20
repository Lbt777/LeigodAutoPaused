package main

import (
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

	appendMenu(h, win.MF_STRING|win.MF_GRAYED, 0, "监控进程："+cfgStore.Load().(Config).Process)
	appendMenu(h, win.MF_STRING, menuIDSettings, "打开设置")
	appendMenu(h, win.MF_STRING, menuIDPause, "立即暂停")
	flags := uint32(win.MF_STRING)
	if autostartEnabled() {
		flags |= win.MF_CHECKED
	}
	appendMenu(h, flags, menuIDAutostart, "开机启动")
	appendMenu(h, win.MF_SEPARATOR, 0, "")
	appendMenu(h, win.MF_STRING, menuIDQuit, "退出")

	hWnd := mw.Handle()
	win.SetForegroundWindow(hWnd)
	var p win.POINT
	win.GetCursorPos(&p)
	id := win.TrackPopupMenuEx(h, win.TPM_RETURNCMD|win.TPM_RIGHTBUTTON, p.X, p.Y, hWnd, nil)
	win.PostMessage(hWnd, win.WM_NULL, 0, 0) // 菜单关掉后把前台还回去

	switch id {
	case menuIDSettings:
		showFromTray()
	case menuIDPause:
		manualPause()
	case menuIDAutostart:
		toggleAutostart()
	case menuIDQuit:
		quit()
	}
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
