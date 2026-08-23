package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
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
		return "新增"
	case DiffRemoved:
		return "移除"
	case DiffUpgrade:
		return "升级"
	case DiffDowngrade:
		return "降级"
	default:
		return "一致"
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

// RenderTable 输出差异表格;一致项按 profile 汇总为一行。
func RenderTable(items []DiffItem, sameCount map[string]int) string {
	var b strings.Builder
	fmt.Fprintln(&b, "profile | 插件 | 本机 | 仓库 | 类型")
	widths := []int{8, 32, 10, 10, 6}
	for _, it := range items {
		if it.Type == DiffSame {
			continue
		}
		row := []string{it.Profile, it.Name, it.Local, it.Remote, it.Type.String()}
		for i, cell := range row {
			w := utf8.RuneCountInString(cell)
			padding := widths[i] - w
			if padding < 1 {
				padding = 1
			}
			b.WriteString(cell + strings.Repeat(" ", padding) + "| ")
		}
		b.WriteString("\n")
	}
	if len(items) == 0 {
		b.WriteString("(无差异:仓库清单与本机实装完全一致)\n")
		return b.String()
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
