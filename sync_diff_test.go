package main

import (
	"strings"
	"testing"
)

func TestCompareKinds(t *testing.T) {
	remote := Manifest{Profiles: map[string][]PluginRef{
		"web": {
			{Name: "a/up", Version: "2.0.0"},
			{Name: "b/new", Version: "1.0.0"},
			{Name: "d/same", Version: "0.1.0"},
		},
	}}
	local := map[string][]Installed{
		"web": {
			{Name: "a/up", Version: "1.0.0"},
			{Name: "c/old", Version: "0.9.0"},
			{Name: "d/same", Version: "0.1.0"},
		},
	}
	items := Compare(remote, local)
	kind := map[string]DiffType{}
	for _, it := range items {
		kind[it.Name] = it.Type
	}
	if kind["a/up"] != DiffUpgrade {
		t.Fatalf("a/up kind=%v,want 升级", kind["a/up"])
	}
	if kind["b/new"] != DiffAdded {
		t.Fatalf("b/new kind=%v,want 新增", kind["b/new"])
	}
	if kind["c/old"] != DiffRemoved {
		t.Fatalf("c/old kind=%v,want 移除", kind["c/old"])
	}
	if kind["d/same"] != DiffSame {
		t.Fatalf("d/same kind=%v,want 一致", kind["d/same"])
	}
}

func TestRenderTable(t *testing.T) {
	items := []DiffItem{
		{Profile: "web", Name: "@x/y", Type: DiffUpgrade, Local: "0.2.8", Remote: "0.4.0"},
	}
	out := RenderTable(items, map[string]int{"web": 3})
	for _, want := range []string{"@x/y", "升级", "3 项一致"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q:\n%s", want, out)
		}
	}
}

func TestRenderTableEmpty(t *testing.T) {
	out := RenderTable(nil, nil)
	if !strings.Contains(out, "无差异") {
		t.Fatalf("expected 无差异,got:\n%s", out)
	}
}
