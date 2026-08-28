package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// buildCmd 清理并重新构建 DSH 源码仓库。
func buildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "清理并重新构建 DSH 源码仓库",
		Long:  "依次执行 pnpm run clean 与 pnpm run build(在 DSH_REPO 仓库内)。",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild()
		},
	}
}

// runBuild 顺序执行 clean 与 build;仅本地变更,不 commit/push。
func runBuild() error {
	repo, err := repoRoot()
	if err != nil {
		return err
	}
	fmt.Println("使用仓库:", repo)
	fmt.Println("清理: pnpm run clean")
	if err := runCmd(repo, "pnpm", "run", "clean"); err != nil {
		return fmt.Errorf("pnpm run clean 失败: %w", err)
	}
	fmt.Println("构建: pnpm run build (可能需要几分钟)")
	if err := runCmd(repo, "pnpm", "run", "build"); err != nil {
		return fmt.Errorf("pnpm run build 失败: %w", err)
	}
	fmt.Println("构建完成。")
	return nil
}
