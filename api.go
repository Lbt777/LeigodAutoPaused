package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	apiBase = "https://webapi.leigod.com"
	// 雷神官网前端页面公开的签名常量（不是账号/个人密钥），登录和暂停接口都靠它做请求签名
	webSignKey = "5C5A639C20665313622F51E93E3F2783"
	browserUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var httpClient = &http.Client{Timeout: 12 * time.Second}

type apiResult struct {
	Code int
	Msg  string
	Raw  []byte
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func postForm(path string, form url.Values) (*apiResult, error) {
	body := form.Encode()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 1200 * time.Millisecond)
		}
		req, err := http.NewRequest(http.MethodPost, apiBase+path, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", browserUA)
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Referer", "https://www.leigod.com/")
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		// 418/429/5xx 是网关或风控的偶发响应，重试即可
		if resp.StatusCode == 418 || resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		text := decodeText(raw)
		var r struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal([]byte(text), &r); err != nil {
			return nil, fmt.Errorf("接口返回无法解析：%s", snippet(text))
		}
		return &apiResult{Code: r.Code, Msg: r.Msg, Raw: raw}, nil
	}
	return nil, fmt.Errorf("网络错误：%v", lastErr)
}

func decodeText(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b); err == nil {
		return string(out)
	}
	return string(b)
}

func snippet(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > 100 {
		return string(r[:100]) + "…"
	}
	return s
}

func pauseWithToken(token string) (int, string, error) {
	res, err := postForm("/api/user/pause", url.Values{
		"account_token": {token},
		"lang":          {"zh_CN"},
		"os_type":       {"4"},
	})
	if err != nil {
		return 0, "", err
	}
	return res.Code, res.Msg, nil
}

func loginAndGetToken(account, password string) (string, error) {
	form := url.Values{}
	form.Set("account_token", "null")
	form.Set("country_code", "86")
	form.Set("lang", "zh_CN")
	form.Set("mobile_num", account)
	form.Set("os_type", "4")
	form.Set("password", md5Hex(password))
	form.Set("region_code", "1")
	form.Set("src_channel", "guanwang")
	form.Set("username", account)
	form.Set("ts", strconv.FormatInt(time.Now().Unix(), 10))
	form.Set("sign", md5Hex(form.Encode()+"&key="+webSignKey))

	res, err := postForm("/api/auth/login/v1", form)
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", fmt.Errorf("登录失败：[%d] %s", res.Code, res.Msg)
	}
	var body struct {
		Data struct {
			LoginInfo struct {
				AccountToken string `json:"account_token"`
			} `json:"login_info"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(decodeText(res.Raw)), &body); err != nil {
		return "", fmt.Errorf("登录返回无法解析")
	}
	if body.Data.LoginInfo.AccountToken == "" {
		return "", fmt.Errorf("登录返回中没有 account_token")
	}
	return body.Data.LoginInfo.AccountToken, nil
}

// 本次运行里通过账号密码登录拿到的 token，供后续暂停复用
var sessionTok atomic.Value

func storeToken(t string) { sessionTok.Store(t) }

func cachedToken() string {
	t, _ := sessionTok.Load().(string)
	return t
}

// 用保存的账号密码静默登录（登录本身没有副作用），成功则缓存 token
func loginSession(cfg Config) error {
	if strings.TrimSpace(cfg.Account) == "" {
		return fmt.Errorf("未保存手机号")
	}
	pwd := decryptText(cfg.Password)
	if pwd == "" {
		return fmt.Errorf("未保存密码")
	}
	t, err := loginAndGetToken(cfg.Account, pwd)
	if err != nil {
		return err
	}
	storeToken(t)
	return nil
}

// 客户端 token 优先，失效则用保存的账号密码登录，再暂停
// 返回：是否成功、提示、是否需要用户填账号密码
func pauseNow(cfg Config) (ok bool, msg string, needLogin bool) {
	lastMsg := ""
	for _, t := range []string{clientToken(), cachedToken()} {
		if t == "" {
			continue
		}
		code, m, err := pauseWithToken(t)
		if err != nil {
			lastMsg = err.Error()
			continue
		}
		switch code {
		case 0:
			storeToken(t)
			return true, "已暂停", false
		case 400803:
			storeToken(t)
			return true, "已经处于暂停状态", false
		}
		lastMsg = fmt.Sprintf("[%d] %s", code, m)
	}

	// token 都不可用：登录后再暂停
	if err := loginSession(cfg); err != nil {
		if lastMsg != "" {
			return false, err.Error() + "；" + lastMsg, true
		}
		return false, err.Error(), true
	}
	code, m, err := pauseWithToken(cachedToken())
	if err != nil {
		return false, err.Error(), false
	}
	switch code {
	case 0:
		return true, "已暂停（账号密码登录）", false
	case 400803:
		return true, "已经处于暂停状态", false
	}
	return false, fmt.Sprintf("[%d] %s", code, m), false
}

func pushBark(barkKey, title, body string) {
	if barkKey == "" {
		return
	}
	u := "https://api.day.app/" + url.PathEscape(barkKey) + "/" +
		url.PathEscape(title) + "/" + url.PathEscape(body) +
		"?group=" + url.QueryEscape("雷神加速器") + "&level=active"
	resp, err := httpClient.Get(u)
	if err != nil {
		return
	}
	resp.Body.Close()
}
