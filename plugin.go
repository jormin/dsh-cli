package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// bundleVerbs 会改变插件集合、需要重启 profile 才生效的 pnpm verb(pnpm 的 rm 是 remove 别名)。
var bundleVerbs = map[string]bool{
	"add":    true,
	"remove": true,
	"rm":     true,
	"update": true,
	"up":     true,
}

// isBundleVerb 判断该 pnpm verb 是否需要重启提示。
func isBundleVerb(verb string) bool { return bundleVerbs[verb] }

// startWeb 启动 web 服务(与 restartWeb 配对,均复用 util.go/web.go 的服务管理)。
func startWeb(repo string) error { return startService(repo, nil, nil) }

// maybeRestartAfterChange 插件增删/更新成功后,按 web 运行状态询问"重启"或"启动"。
// busy/restart/start/ask/writer 均可注入便于测试;传 nil 时使用产品实现。
func maybeRestartAfterChange(repo string, busy func() bool, restart, start func(string) error, ask func(string) bool, out io.Writer) error {
	if busy == nil {
		busy = portBusy
	}
	if restart == nil {
		restart = restartWeb
	}
	if start == nil {
		start = startWeb
	}
	if ask == nil {
		ask = askYN
	}
	if out == nil {
		out = os.Stdout
	}
	if busy() {
		if !ask("插件已变更,是否重启 web 服务?") {
			fmt.Fprintln(out, "已取消重启。可稍后手动运行 dsh web restart。")
			return nil
		}
		fmt.Fprintln(out, "重启 web 服务…")
		if err := restart(repo); err != nil {
			return err
		}
		fmt.Fprintln(out, "已重启。")
		return nil
	}
	if !ask("插件已变更,是否启动 web 服务?") {
		fmt.Fprintln(out, "已取消启动。可稍后手动运行 dsh web start。")
		return nil
	}
	fmt.Fprintln(out, "启动 web 服务…")
	if err := start(repo); err != nil {
		return err
	}
	fmt.Fprintln(out, "已启动。")
	return nil
}

// pluginCmd 透传给源码仓库的 dsh plugin(管理 profile 插件).
func pluginCmd() *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "plugin [pnpm args...]",
		Short: "管理 profile 插件(透传 dsh plugin)",
		Long:  "等价于源码仓库的 dsh plugin --profile <name> <pnpm args...>。例:dsh plugin add <package>、dsh plugin remove <pkg>。增删/更新成功后询问是否重启 web。",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("plugin 需要参数,例如: dsh plugin add <package>")
			}
			repo, err := repoRoot()
			if err != nil {
				return err
			}
			full := append([]string{"-C", repo, "dsh", "plugin", "--profile", profile}, args...)
			if err := runCmd(repo, "pnpm", full...); err != nil {
				return err
			}
			if isBundleVerb(args[0]) {
				return maybeRestartAfterChange(repo, nil, nil, nil, nil, nil)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "web", "profile 名称(默认 web)")
	return cmd
}
