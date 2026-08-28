package main

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var versionTagRe = regexp.MustCompile(`^(dsh-)?v?[0-9]+\.[0-9]+\.[0-9]+`)

func gitOut(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	return string(out), err
}

// latestTagByCreation 按 tag 创建时间倒序返回第一个版本 tag.
func latestTagByCreation(repo string) (string, error) {
	cmd := exec.Command("git", "for-each-ref", "refs/tags", "--sort=-creatordate", "--format=%(refname:short)")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && versionTagRe.MatchString(line) {
			return line, nil
		}
	}
	return "", nil
}

// currentOnTag 当前检出是否正处在该标签上(分支名等于标签名,或 detached 且精确指向).
func currentOnTag(repo, tag string) bool {
	if b, err := gitOut(repo, "branch", "--show-current"); err == nil && strings.TrimSpace(b) == tag {
		return true
	}
	if d, err := gitOut(repo, "describe", "--tags", "--exact-match", "HEAD"); err == nil && strings.TrimSpace(d) == tag {
		return true
	}
	return false
}

func updateCmd() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update [--check]",
		Short: "更新源码仓库到最新 release tag(--check 只检测)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "只检测最新版本,不更新")
	return cmd
}

func runUpdate(checkOnly bool) error {
	repo, err := repoRoot()
	if err != nil {
		return err
	}
	fmt.Println("使用仓库:", repo)

	fmt.Println("拉取远程 tags: git fetch --tags --prune origin")
	if err := runCmd(repo, "git", "fetch", "--tags", "--prune", "origin"); err != nil {
		fmt.Println("警告: 无法连接远程仓库,将基于本地已有 tag 判断")
	}

	latest, err := latestTagByCreation(repo)
	if err != nil {
		return fmt.Errorf("读取 tag 列表失败: %w", err)
	}
	if latest == "" {
		return errors.New("未找到任何版本 tag")
	}

	head, _ := gitOut(repo, "rev-parse", "--short", "HEAD")
	desc, _ := gitOut(repo, "describe", "--tags", "--always")
	branch, _ := gitOut(repo, "branch", "--show-current")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "(detached HEAD)"
	}

	fmt.Println()
	fmt.Println("最新版本 tag:", latest)
	fmt.Println("当前分支    :", branch)
	fmt.Println("当前检出位置:", strings.TrimSpace(head)+" ("+strings.TrimSpace(desc)+")")

	if currentOnTag(repo, latest) {
		fmt.Println("当前已是最新版本:", latest)
		if checkOnly {
			return nil
		}
		if askYN("是否启动服务?") {
			return startService(repo, nil, nil)
		}
		return nil
	}

	if checkOnly {
		fmt.Println("发现新版本:", latest, "当前不在该版本上,运行 dsh update 可一键更新。")
		return errors.New("检测到新版本: " + latest)
	}

	fmt.Println("当前未检出在最新标签", latest, "上(当前分支:", branch+")")
	if !askYN("是否更新到 " + latest + "?") {
		fmt.Println("已取消,未做任何更改。")
		return nil
	}

	dirty, _ := gitOut(repo, "status", "--porcelain")
	if strings.TrimSpace(dirty) != "" {
		return errors.New("工作区存在未提交的改动,已中止。请先 git stash 或提交后重试")
	}

	fmt.Println()
	fmt.Println("切换到版本:", latest)
	if err := runCmd(repo, "git", "checkout", latest); err != nil {
		return err
	}

	fmt.Println("安装依赖: pnpm install")
	if err := runCmd(repo, "pnpm", "install"); err != nil {
		return fmt.Errorf("pnpm install 失败: %w", err)
	}

	fmt.Println("清理: pnpm run clean")
	if err := runCmd(repo, "pnpm", "run", "clean"); err != nil {
		return fmt.Errorf("pnpm run clean 失败: %w", err)
	}
	fmt.Println("重新构建: pnpm run build (可能需要几分钟)")
	if err := runCmd(repo, "pnpm", "run", "build"); err != nil {
		return fmt.Errorf("pnpm run build 失败: %w", err)
	}

	fmt.Println()
	return askStartOrRestart(repo, "更新完成")
}
