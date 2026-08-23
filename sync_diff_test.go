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
	if items[0].Name != "a/up" || items[len(items)-1].Name != "d/same" {
		t.Fatalf("输出应按 name 排序: %+v", items)
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
	for _, want := range []string{"@x/y", "3 项一致"} {
		if strings.Contains(out, "upgrade") {
			t.Fatalf("type 列不应出现在表格中:\n%s", out)
		}
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

func TestRenderTableStyle(t *testing.T) {
	items := []DiffItem{
		{Profile: "web", Name: "@x/y", Type: DiffAdded, Local: "-", Remote: "1.0.0"},
		{Profile: "web", Name: "b", Type: DiffRemoved, Local: "2.0.0", Remote: "-"},
	}
	out := RenderTable(items, nil)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("期望 4 行(表头/分隔线/2 数据),got %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "profile") {
		t.Fatalf("首行应为表头:%s", lines[0])
	}
	if !strings.Contains(lines[1], "-") {
		t.Fatalf("第二行应为分隔线:%s", lines[1])
	}
	if strings.Contains(out, "| \n") || strings.HasSuffix(strings.TrimRight(out, "\n"), "| ") {
		t.Fatalf("行尾不应有多余分隔符:\n%s", out)
	}
	if !strings.Contains(lines[2], "1.0.0") || !strings.Contains(lines[3], "2.0.0") {
		t.Fatalf("数据行内容缺失:\n%s", out)
	}
}

func TestDisplayWidth(t *testing.T) {
	if displayWidth("abc") != 3 {
		t.Fatalf("displayWidth(abc)=%d", displayWidth("abc"))
	}
	if displayWidth("插件") != 4 {
		t.Fatalf("displayWidth(插件)=%d", displayWidth("插件"))
	}
	if displayWidth("a中文1") != 6 {
		t.Fatalf("displayWidth(a中文1)=%d", displayWidth("a中文1"))
	}
}
