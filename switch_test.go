package main

import (
	"strings"
	"testing"
)

func TestResolveSwitchTag(t *testing.T) {
	tags := []string{"dsh-v0.1.2-alpha.1", "dsh-v0.1.1-rc.2", "dsh-v0.1.0-rc.8"}
	if got := resolveSwitchTag("dsh-v0.1.2-alpha.1", tags); got != "dsh-v0.1.2-alpha.1" {
		t.Fatalf("got %q", got)
	}
	if got := resolveSwitchTag("v0.1.1-rc.2", tags); got != "dsh-v0.1.1-rc.2" {
		t.Fatalf("v 前缀应解析到 dsh- 前缀 tag,got %q", got)
	}
	if got := resolveSwitchTag("v9.9.9", tags); got != "" {
		t.Fatalf("应返回空,got %q", got)
	}
}

func TestSelectTagDefault(t *testing.T) {
	tags := []string{"dsh-v0.1.2-alpha.1", "dsh-v0.1.1-rc.2"}
	got, err := selectTag(tags, strings.NewReader("\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "dsh-v0.1.2-alpha.1" {
		t.Fatalf("回车应选最新,got %q", got)
	}
}

func TestSelectTagByIndex(t *testing.T) {
	tags := []string{"dsh-v0.1.2-alpha.1", "dsh-v0.1.1-rc.2"}
	got, err := selectTag(tags, strings.NewReader("2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "dsh-v0.1.1-rc.2" {
		t.Fatalf("选 2 应得第二个,got %q", got)
	}
}

func TestSelectTagInvalid(t *testing.T) {
	tags := []string{"dsh-v0.1.2-alpha.1"}
	if _, err := selectTag(tags, strings.NewReader("99\n")); err == nil {
		t.Fatal("越界序号应报错")
	}
	if _, err := selectTag(tags, strings.NewReader("abc\n")); err == nil {
		t.Fatal("非数字应报错")
	}
}
