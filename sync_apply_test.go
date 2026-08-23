package main

import (
	"bytes"
	"strings"
	"testing"
)

type fakeApplyRunner struct {
	cmds [][]string
}

func (f *fakeApplyRunner) Run(dir, name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	return nil, nil
}

func TestApplyChanges(t *testing.T) {
	f := &fakeApplyRunner{}
	var out bytes.Buffer
	final := Manifest{Profiles: map[string][]PluginRef{
		"web": {
			{Name: "a/keep", Version: "1.0.0"},
			{Name: "b/up", Version: "2.0.0"},
			{Name: "c/new", Version: "1.1.0"},
		},
	}}
	local := map[string][]Installed{
		"web": {
			{Name: "a/keep", Version: "1.0.0"},
			{Name: "b/up", Version: "1.0.0"},
			{Name: "d/old", Version: "0.5.0"},
		},
	}
	changed, errs := ApplyChanges("/repo", final, local, f.Run, &out)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(f.cmds) != 3 {
		t.Fatalf("expected 3 commands, got %v", f.cmds)
	}
	joined := make([]string, len(f.cmds))
	for i, c := range f.cmds {
		joined[i] = strings.Join(c, " ")
	}
	wantCmds := []string{
		"pnpm -C /repo dsh plugin --profile web add --save-exact b/up@2.0.0",
		"pnpm -C /repo dsh plugin --profile web add --save-exact c/new@1.1.0",
		"pnpm -C /repo dsh plugin --profile web remove d/old",
	}
	for i, w := range wantCmds {
		if joined[i] != w {
			t.Fatalf("cmd[%d]=%q,want %q", i, joined[i], w)
		}
	}
}

func TestApplyNoChanges(t *testing.T) {
	f := &fakeApplyRunner{}
	var out bytes.Buffer
	final := Manifest{Profiles: map[string][]PluginRef{"web": {{Name: "a/keep", Version: "1.0.0"}}}}
	local := map[string][]Installed{"web": {{Name: "a/keep", Version: "1.0.0"}}}
	changed, errs := ApplyChanges("/repo", final, local, f.Run, &out)
	if len(errs) != 0 || changed || len(f.cmds) != 0 {
		t.Fatalf("changed=%v cmds=%v errs=%v", changed, f.cmds, errs)
	}
}

func TestDecideRestart(t *testing.T) {
	if !decideRestart(true, func() bool { return true }) {
		t.Fatal("有变更且服务运行中应重启")
	}
	if decideRestart(true, func() bool { return false }) {
		t.Fatal("有变更但服务未运行不应重启")
	}
	if decideRestart(false, func() bool { return true }) {
		t.Fatal("无变更不应重启")
	}
}
