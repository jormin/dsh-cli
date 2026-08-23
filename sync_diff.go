package main

import (
	"fmt"
	"sort"
	"strings"
)

// DiffType 描述一个插件在两边的差异。
type DiffType int

const (
	DiffSame DiffType = iota
	DiffAdded
	DiffRemoved
	DiffUpgrade
	DiffDowngrade
)

func (t DiffType) String() string {
	switch t {
	case DiffAdded:
		return "added"
	case DiffRemoved:
		return "removed"
	case DiffUpgrade:
		return "upgrade"
	case DiffDowngrade:
		return "downgrade"
	default:
		return "same"
	}
}

// DiffItem 一行差异;Local/Remote 为 "-" 表示该侧不存在。
type DiffItem struct {
	Profile string
	Name    string
	Type    DiffType
	Local   string
	Remote  string
}

// Compare 计算"仓库清单 vs 本机实装清单"的差异项(含一致项,由调用方决定展示)。
func Compare(remote Manifest, local map[string][]Installed) []DiffItem {
	remote = remote.Normalize()
	profiles := map[string]struct{}{}
	for p := range remote.Profiles {
		profiles[p] = struct{}{}
	}
	for p := range local {
		profiles[p] = struct{}{}
	}
	var items []DiffItem
	names := make([]string, 0, len(profiles))
	for p := range profiles {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, profile := range names {
		remoteByName := map[string]PluginRef{}
		for _, ref := range remote.Profiles[profile] {
			remoteByName[ref.Name] = ref
		}
		localByName := map[string]Installed{}
		for _, ins := range local[profile] {
			localByName[ins.Name] = ins
		}
		seen := map[string]bool{}
		for name, ref := range remoteByName {
			seen[name] = true
			ins, ok := localByName[name]
			switch {
			case !ok:
				items = append(items, DiffItem{profile, name, DiffAdded, "-", ref.Version})
			case ins.Version == ref.Version:
				items = append(items, DiffItem{profile, name, DiffSame, ins.Version, ref.Version})
			case versionNewer(ref.Version, ins.Version):
				items = append(items, DiffItem{profile, name, DiffUpgrade, ins.Version, ref.Version})
			default:
				items = append(items, DiffItem{profile, name, DiffDowngrade, ins.Version, ref.Version})
			}
		}
		for name, ins := range localByName {
			if !seen[name] {
				items = append(items, DiffItem{profile, name, DiffRemoved, ins.Version, "-"})
			}
		}
	}
	// 输出顺序固定:按 (profile, 插件名) 排序,保证每次输出稳定
	sort.Slice(items, func(i, j int) bool {
		if items[i].Profile != items[j].Profile {
			return items[i].Profile < items[j].Profile
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// versionNewer 按三段数字比较;a > b 返回 true;解析失败回退字符串比较。
func versionNewer(a, b string) bool {
	av, aerr := parseVersion(a) // 复用 upgrade.go 的 parseVersion;返回 ([]int, error)
	bv, berr := parseVersion(b)
	if aerr == nil && berr == nil {
		return compareVersions(av, bv) > 0 // 复用 upgrade.go 的 compareVersions
	}
	return a > b
}

// isWide 判断是否为东亚宽/全角字符(终端中占 2 列)。
func isWide(r rune) bool {
	return r == 0x2329 || r == 0x232A ||
		(r >= 0x1100 && r <= 0x115F) ||
		(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE30 && r <= 0xFE4F) ||
		(r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6)
}

// displayWidth 计算终端显示宽度:东亚宽/全角字符按 2 列计,保证 CJK 表格对齐。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// RenderTable 输出差异表格;列宽按内容动态计算(CJK 按 2 列),带表头分隔线,
// 行尾无多余分隔符;一致项按 profile 汇总为一行。
func RenderTable(items []DiffItem, sameCount map[string]int) string {
	var b strings.Builder
	rows := [][]string{{"profile", "plugin", "local", "repo"}}
	for _, it := range items {
		if it.Type == DiffSame {
			continue
		}
		rows = append(rows, []string{it.Profile, it.Name, it.Local, it.Remote})
	}
	if len(rows) == 1 {
		b.WriteString("(无差异:仓库清单与本机实装完全一致)\n")
		return b.String()
	}
	cols := len(rows[0])
	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	for ri, row := range rows {
		parts := make([]string, cols)
		for i, cell := range row {
			parts[i] = cell + strings.Repeat(" ", widths[i]-displayWidth(cell))
		}
		b.WriteString(strings.Join(parts, " | "))
		b.WriteString("\n")
		if ri == 0 {
			seps := make([]string, cols)
			for i := range widths {
				seps[i] = strings.Repeat("-", widths[i])
			}
			b.WriteString(strings.Join(seps, " | "))
			b.WriteString("\n")
		}
	}
	names := make([]string, 0, len(sameCount))
	for p := range sameCount {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		if n := sameCount[p]; n > 0 {
			fmt.Fprintf(&b, "profile %s 另有 %d 项一致(省略)\n", p, n)
		}
	}
	return b.String()
}
