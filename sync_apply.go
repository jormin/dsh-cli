package main

import (
	"fmt"
	"io"
	"sort"
)

// ApplyChanges 按最终清单调整本机插件,返回是否有实际变更。
// 命令统一走 pnpm -C <repo> dsh plugin --profile <n> add/remove(与查询同口径,自动 reconcile bundles)。
// add 使用 --save-exact 精确锁定版本,避免 pnpm 默认 ^ 前缀导致跨机版本漂移。
func ApplyChanges(repo string, final Manifest, local map[string][]Installed, run CmdRunner, out io.Writer) (bool, []error) {
	var errs []error
	changed := false
	lf := final.Normalize()

	// 按 (profile, 包名, 操作) 排序,保证多次执行顺序稳定、可测试
	type op struct {
		profile, verb, pkg string
	}
	var ops []op
	for profile, refs := range lf.Profiles {
		installed := map[string]Installed{}
		for _, i := range local[profile] {
			installed[i.Name] = i
		}
		for _, ref := range refs {
			cur, ok := installed[ref.Name]
			if !ok || cur.Version != ref.Version {
				ops = append(ops, op{profile, "add", ref.Name + "@" + ref.Version})
			}
		}
	}
	for profile, list := range local {
		refs := map[string]bool{}
		for _, r := range lf.Profiles[profile] {
			refs[r.Name] = true
		}
		for _, ins := range list {
			if !refs[ins.Name] {
				ops = append(ops, op{profile, "remove", ins.Name})
			}
		}
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].profile != ops[j].profile {
			return ops[i].profile < ops[j].profile
		}
		if ops[i].pkg != ops[j].pkg {
			return ops[i].pkg < ops[j].pkg
		}
		return ops[i].verb < ops[j].verb
	})
	for _, o := range ops {
		args := []string{"pnpm", "-C", repo, "dsh", "plugin", "--profile", o.profile}
		if o.verb == "add" {
			args = append(args, "add", "--save-exact", o.pkg)
		} else {
			args = append(args, "remove", o.pkg)
		}
		if _, err := run("", args[0], args[1:]...); err != nil {
			errs = append(errs, fmt.Errorf("profile %s %s %s 失败: %w", o.profile, o.verb, o.pkg, err))
			continue
		}
		changed = true
		fmt.Fprintf(out, "已%s %s (profile %s)\n", applyVerbLabel(o.verb), o.pkg, o.profile)
	}
	return changed, errs
}

// applyVerbLabel add/remove 的人类可读动词。
func applyVerbLabel(verb string) string {
	if verb == "add" {
		return "安装"
	}
	return "移除"
}

// decideRestart 决定是否重启:有变更且端口被占用(服务运行中)。
func decideRestart(changed bool, busy func() bool) bool {
	return changed && busy()
}

// restartWeb 复用现有服务管理:stopService + startService。
func restartWeb(repo string) error {
	if err := stopService(); err != nil {
		return err
	}
	return startService(repo, nil, nil)
}
