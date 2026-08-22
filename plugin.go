package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// pluginCmd 透传给源码仓库的 dsh plugin(管理 profile 插件).
func pluginCmd() *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "plugin [pnpm args...]",
		Short: "管理 profile 插件(透传 dsh plugin)",
		Long:  "等价于源码仓库的 dsh plugin --profile <name> <pnpm args...>。例:dsh plugin add <package>、dsh plugin remove <pkg>。",
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
			return runCmd(repo, "pnpm", full...)
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "web", "profile 名称(默认 web)")
	return cmd
}