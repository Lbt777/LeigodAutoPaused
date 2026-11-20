package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 配置文件沿用旧版的字段名，两个版本可以共用同一份 config.json
type Config struct {
	Interval int    `json:"IntervalSeconds"`
	Game     string `json:"GameProcess"`
	BarkURL  string `json:"BarkUrl"`
	Account  string `json:"Account"`
	Password string `json:"Password"` // DPAPI 加密后的 hex
}

const autostartTask = "LeishenAutoPause"

// 内置固定值，界面不再让用户填
const (
	gameProcess  = "TslGame"
	checkSeconds = 5
)

func dataDir() string   { return filepath.Join(os.Getenv("APPDATA"), "LeishenAutoPause") }
func configPath() string { return filepath.Join(dataDir(), "config.json") }

func defaultConfig() Config {
	return Config{Interval: checkSeconds, Game: gameProcess}
}

func loadConfig() Config {
	c := defaultConfig()
	if b, err := os.ReadFile(configPath()); err == nil {
		// 旧版 PowerShell 写出的文件带 UTF-8 BOM，JSON 解析前要剥掉
		b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
		_ = json.Unmarshal(b, &c)
	}
	c.Interval = checkSeconds
	c.Game = gameProcess
	return c
}

func saveConfig(c Config) error {
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o644)
}

// 界面上只填设备 Key，配置里存完整地址
func (c Config) barkKey() string {
	u := strings.TrimSpace(c.BarkURL)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "http") {
		u = strings.TrimRight(u, "/")
		if i := strings.LastIndex(u, "/"); i >= 0 {
			return u[i+1:]
		}
		return ""
	}
	return strings.Trim(u, "/")
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

// ---------------------------------------------------------------- DPAPI

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32              = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")
)

func dpapi(protect bool, data []byte) ([]byte, error) {
	var in, out dataBlob
	in.cbData = uint32(len(data))
	if len(data) > 0 {
		in.pbData = &data[0]
	}
	proc := procCryptUnprotectData
	if protect {
		proc = procCryptProtectData
	}
	ret, _, err := proc.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)))
	if ret == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return append([]byte(nil), unsafe.Slice(out.pbData, out.cbData)...), nil
}

func encryptText(s string) string {
	if s == "" {
		return ""
	}
	// DPAPI/SecureString 约定明文是 UTF-16LE，这样新旧版本可以互相解密
	b, err := dpapi(true, utf16LEBytes(s))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// 解不开（换机器 / 换 Windows 用户）时返回空串，界面上重新填一次即可
func decryptText(h string) string {
	if h == "" {
		return ""
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return ""
	}
	p, err := dpapi(false, b)
	if err != nil {
		return ""
	}
	return decodeUTF16LE(p)
}

func utf16LEBytes(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return b
}

func decodeUTF16LE(b []byte) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
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
	return hiddenCmd("schtasks", "/Query", "/TN", autostartTask).Run() == nil
}

func setAutostart(on bool) error {
	if on {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return hiddenCmd("schtasks", "/Create", "/SC", "ONLOGON", "/TN", autostartTask,
			"/TR", fmt.Sprintf(`"%s"`, exe), "/F").Run()
	}
	return hiddenCmd("schtasks", "/Delete", "/TN", autostartTask, "/F").Run()
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
