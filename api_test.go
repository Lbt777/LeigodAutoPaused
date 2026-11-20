package main

import (
	"net/url"
	"strconv"
	"testing"
)

// 登录签名必须和官方前端一致：参数按键名排序拼成 a=b&c=d，末尾加 &key=，再取 md5
func TestLoginSign(t *testing.T) {
	form := url.Values{}
	form.Set("account_token", "null")
	form.Set("country_code", "86")
	form.Set("lang", "zh_CN")
	form.Set("mobile_num", "13800138000")
	form.Set("os_type", "4")
	form.Set("password", md5Hex("123456"))
	form.Set("region_code", "1")
	form.Set("src_channel", "guanwang")
	form.Set("username", "13800138000")
	form.Set("ts", strconv.FormatInt(1700000000, 10))

	const want = "35636ae19bf320a5f43e143cf5b4d4e1"
	if got := md5Hex(form.Encode() + "&key=" + webSignKey); got != want {
		t.Fatalf("签名不一致：%s != %s", got, want)
	}
}

func TestBarkKeyRoundTrip(t *testing.T) {
	if got := barkURLFromKey("abcDEF123"); got != "https://api.day.app/abcDEF123" {
		t.Fatalf("补全失败：%s", got)
	}
	if got := barkURLFromKey(""); got != "" {
		t.Fatalf("空 Key 应返回空：%s", got)
	}
}

// config.ini 解析：注释、空格、密码里的 = 都要正确处理
func TestParseConfig(t *testing.T) {
	c := parseConfig("; 注释行\n\n游戏进程名 = NarakaBladepoint \n账号=13800138000\n密码=abc=123\nBarkAppKey=k1\n")
	if c.Process != "NarakaBladepoint" || c.Account != "13800138000" || c.Password != "abc=123" || c.BarkKey != "k1" {
		t.Fatalf("解析结果不对：%+v", c)
	}
	if !c.ready() {
		t.Fatal("账号密码齐全应视为就绪")
	}
	// BarkAppKey 选填，没有也算就绪
	if noBark := (Config{Account: "13800138000", Password: "x"}); !noBark.ready() {
		t.Fatal("缺少 BarkAppKey 不应影响就绪判断")
	}
	if got := parseConfig("").Process; got != defaultProcess {
		t.Fatalf("缺省进程名应为 %s，实际 %s", defaultProcess, got)
	}
}
