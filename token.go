package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// 雷神客户端（Electron）把登录信息明文写在自己的 localStorage 里，
// userInfo 里带 32 位 hex 的 token。实测这个 token 常是客户端上次登录留下的、
// 可能已被服务端判定过期，所以只当 best-effort：能用就用，不能用再登录。
var tokenRe = regexp.MustCompile(`"token"\s*:\s*"([0-9a-fA-F]{32})"`)

func clientToken() string {
	dir := filepath.Join(os.Getenv("APPDATA"), "leigod", "Local Storage", "leveldb")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	type fileInfo struct {
		path string
		mod  int64
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if !strings.HasSuffix(n, ".log") && !strings.HasSuffix(n, ".ldb") {
			continue
		}
		if info, err := e.Info(); err == nil {
			files = append(files, fileInfo{filepath.Join(dir, e.Name()), info.ModTime().UnixNano()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })

	for _, f := range files {
		b, err := readShared(f.path)
		if err != nil || len(b) < 4 {
			continue
		}
		if len(b)%2 == 1 {
			b = b[:len(b)-1]
		}
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
		if m := tokenRe.FindAllStringSubmatch(string(utf16.Decode(u)), -1); len(m) > 0 {
			return m[len(m)-1][1]
		}
	}
	return ""
}

// 客户端运行时会独占这些文件，必须以共享模式打开
func readShared(path string) ([]byte, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}
