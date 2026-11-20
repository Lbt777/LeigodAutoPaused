package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// 配置全部来自本机的 config.ini，密码是明文，不加密、不上传
type Config struct {
	Process  string // 游戏进程名，不带 .exe
	Account  string
	Password string
	BarkKey  string
}

const (
	defaultProcess = "TslGame"
	checkSeconds   = 5
	autostartKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartValue = "LeishenAutoPause"
)

func dataDir() string    { return filepath.Join(os.Getenv("APPDATA"), "LeishenAutoPause") }
func configPath() string { return filepath.Join(dataDir(), "config.ini") }

func defaultConfig() Config { return Config{Process: defaultProcess} }

// 账号密码填了就算就绪；BarkAppKey 选填，为空只是不推送
func (c Config) ready() bool {
	return c.Account != "" && c.Password != ""
}

const configTemplate = `; 雷神自动暂停 · 本地配置（仅保存在本机，不会上传）
; 游戏进程名不带 .exe，例如 PUBG 是 TslGame、永劫无间是 NarakaBladepoint
游戏进程名=%s
; 雷神加速器账号（手机号）
账号=%s
; 雷神加速器密码（明文，仅存本机）
密码=%s
; Bark App 首页复制的设备 Key，留空则不推送
BarkAppKey=%s
`

func loadConfig() Config {
	b, err := os.ReadFile(configPath())
	if err != nil {
		return defaultConfig()
	}
	return parseConfig(string(b))
}

func parseConfig(text string) Config {
	c := defaultConfig()
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "游戏进程名", "进程名":
			c.Process = v
		case "账号", "手机号":
			c.Account = v
		case "密码":
			c.Password = v
		case "BarkAppKey", "BarkKey":
			c.BarkKey = v
		}
	}
	if c.Process == "" {
		c.Process = defaultProcess
	}
	return c
}

func saveConfig(c Config) error {
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath(), []byte(fmt.Sprintf(configTemplate,
		c.Process, c.Account, c.Password, c.BarkKey)), 0o644)
}

// 配置文件不存在时写一份空模板，方便用户直接改
func ensureConfigFile() {
	if _, err := os.Stat(configPath()); err == nil {
		return
	}
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(configPath(), []byte(fmt.Sprintf(configTemplate,
		defaultProcess, "", "", "")), 0o644)
}

func barkURLFromKey(key string) string {
	k := strings.TrimSpace(key)
	if k == "" {
		return ""
	}
	if strings.HasPrefix(k, "http") {
		return strings.TrimRight(k, "/")
	}
	return "https://api.day.app/" + strings.Trim(k, "/")
}

// ---------------------------------------------------------------- 进程 / 自启 / 单实例

func processRunning(name string) bool {
	want := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), ".exe"))
	if want == "" {
		return false
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		got := strings.ToLower(strings.TrimSuffix(windows.UTF16ToString(e.ExeFile[:]), ".exe"))
		if got == want {
			return true
		}
	}
	return false
}

func hiddenCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(autostartValue)
	if err != nil {
		return false
	}
	exe, err := os.Executable()
	return err == nil && v == `"`+exe+`"`
}

// 写 HKCU\...\Run，登录后由系统在用户桌面里拉起，托盘一定能建出来
func setAutostart(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, autostartKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		return k.DeleteValue(autostartValue)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// 清掉旧版本用计划任务写的自启，避免登录时启动两份
	_ = hiddenCmd("schtasks", "/Delete", "/TN", "LeishenAutoPause", "/F").Run()
	return k.SetStringValue(autostartValue, `"`+exe+`"`)
}

var singleMutex windows.Handle

func acquireSingleInstance() bool {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(`Global\LeishenAutoPauseGo`))
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	singleMutex = h
	return true
}
