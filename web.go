package main

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

// webCmd 实现 dsh web [start|stop|restart] [args...],未写子命令时默认 start.
func webCmd() *cobra.Command {
	var patches []string
	var dumpConfig bool
	var dumpDefaultConfig bool

	cmd := &cobra.Command{
		Use:   "web [start|stop|restart] [args...]",
		Short: "管理 Web 服务(默认 start)",
		Long:  "管理 dsh Web 服务:start(默认)后台启动、stop 停止、restart 重启。args 与 --patch 等 options 原样透传给源码仓库的 dsh web。",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := repoRoot()
			if err != nil {
				return err
			}

			// 第一个参数若是子命令则消费,其余全部作为 app 透传参数
			sub := "start"
			var appArgs []string
			if len(args) > 0 {
				switch args[0] {
				case "start", "stop", "restart":
					sub = args[0]
					appArgs = args[1:]
				default:
					appArgs = args
				}
			}

			var launcherFlags []string
			for _, p := range patches {
				launcherFlags = append(launcherFlags, "--patch", p)
			}
			if dumpConfig {
				launcherFlags = append(launcherFlags, "--dump-config")
			}
			if dumpDefaultConfig {
				launcherFlags = append(launcherFlags, "--dump-default-config")
			}

			switch sub {
			case "start":
				// dump 模式不透传 --no-open,也不后台启动;直接同步执行后退出
				if dumpConfig || dumpDefaultConfig {
					return runCmd(repo, "pnpm", append([]string{"-C", repo, "dsh", "web"}, launcherFlags...)...)
				}
				return startService(repo, launcherFlags, appArgs)
			case "stop":
				return stopService()
			case "restart":
				_ = stopService()
				return startService(repo, launcherFlags, appArgs)
			default:
				return errors.New("未知的 web 子命令: " + strings.Join(args, " "))
			}
		},
	}

	cmd.Flags().StringArrayVar(&patches, "patch", nil, "额外的 patch 覆盖层,可重复(--patch a.yml --patch b.yml)")
	cmd.Flags().BoolVar(&dumpConfig, "dump-config", false, "打印组装的 web profile 配置树并退出")
	cmd.Flags().BoolVar(&dumpDefaultConfig, "dump-default-config", false, "打印默认(不含用户层)配置树并退出")
	return cmd
}