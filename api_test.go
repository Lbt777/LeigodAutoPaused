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
	c := Config{BarkURL: "https://api.day.app/abcDEF123"}
	if got := c.barkKey(); got != "abcDEF123" {
		t.Fatalf("取 Key 失败：%s", got)
	}
}
