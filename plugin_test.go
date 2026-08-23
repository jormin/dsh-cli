package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestIsBundleVerb(t *testing.T) {
	cases := []struct {
		verb string
		want bool
	}{
		{"add", true},
		{"remove", true},
		{"rm", true},
		{"update", true},
		{"up", true},
		{"ls", false},
		{"why", false},
		{"install", false},
	}
	for _, c := range cases {
		if got := isBundleVerb(c.verb); got != c.want {
			t.Fatalf("isBundleVerb(%q)=%v,want %v", c.verb, got, c.want)
		}
	}
}

func TestMaybeRestartAfterChangeRunningYes(t *testing.T) {
	var out bytes.Buffer
	restarted, started := false, false
	var prompt string
	err := maybeRestartAfterChange("/repo",
		func() bool { return true },
		func(string) error { restarted = true; return nil },
		func(string) error { started = true; return nil },
		func(p string) bool { prompt = p; return true },
		&out)
	if err != nil {
		t.Fatal(err)
	}
	if !restarted || started {
		t.Fatalf("restarted=%v started=%v,want 重启且不启动", restarted, started)
	}
	if !strings.Contains(prompt, "是否重启 web 服务?") || !strings.Contains(out.String(), "已重启。") {
		t.Fatalf("prompt=%q output: %s", prompt, out.String())
	}
}

func TestMaybeRestartAfterChangeRunningNo(t *testing.T) {
	var out bytes.Buffer
	restarted := false
	err := maybeRestartAfterChange("/repo",
		func() bool { return true },
		func(string) error { restarted = true; return nil },
		func(string) error { return nil },
		func(string) bool { return false },
		&out)
	if err != nil {
		t.Fatal(err)
	}
	if restarted {
		t.Fatal("拒绝后不应重启")
	}
	if !strings.Contains(out.String(), "已取消重启") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestMaybeRestartAfterChangeNotRunningYes(t *testing.T) {
	var out bytes.Buffer
	restarted, started := false, false
	var prompt string
	err := maybeRestartAfterChange("/repo",
		func() bool { return false },
		func(string) error { restarted = true; return nil },
		func(string) error { started = true; return nil },
		func(p string) bool { prompt = p; return true },
		&out)
	if err != nil {
		t.Fatal(err)
	}
	if started != true || restarted {
		t.Fatalf("started=%v restarted=%v,want 启动且不重启", started, restarted)
	}
	if !strings.Contains(prompt, "是否启动 web 服务?") || !strings.Contains(out.String(), "已启动。") {
		t.Fatalf("prompt=%q output: %s", prompt, out.String())
	}
}

func TestMaybeRestartAfterChangeNotRunningNo(t *testing.T) {
	var out bytes.Buffer
	started := false
	err := maybeRestartAfterChange("/repo",
		func() bool { return false },
		func(string) error { return nil },
		func(string) error { started = true; return nil },
		func(string) bool { return false },
		&out)
	if err != nil {
		t.Fatal(err)
	}
	if started {
		t.Fatal("拒绝后不应启动")
	}
	if !strings.Contains(out.String(), "已取消启动") {
		t.Fatalf("output: %s", out.String())
	}
}
