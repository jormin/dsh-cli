package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// listVersionTags 返回按创建时间降序的全部版本 tag(与 update 使用同一正则)。
func listVersionTags(repo string) ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "refs/tags", "--sort=-creatordate", "--format=%(refname:short)")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && versionTagRe.MatchString(line) {
			tags = append(tags, line)
		}
	}
	return tags, nil
}

// resolveSwitchTag 在可用 tag 中解析用户输入;支持 v0.1.0 与 dsh-v0.1.0 两种写法。
func resolveSwitchTag(arg string, tags []string) string {
	variants := []string{arg}
	if strings.HasPrefix(arg, "v") {
		variants = append(variants, "dsh-"+arg)
	}
	for _, v := range variants {
		for _, t := range tags {
			if t == v {
				return t
			}
		}
	}
	return ""
}

// selectTag 交互式选择版本 tag;返回选中的 tag。默认(回车)选最新。
func selectTag(tags []string, in io.Reader) (string, error) {
	const displayLimit = 20
	show := tags
	if len(tags) > displayLimit {
		show = tags[:displayLimit]
	}
	fmt.Println("可用版本(按创建时间从新到旧):")
	for i, t := range show {
		fmt.Printf("  [%d] %s\n", i+1, t)
	}
	if len(tags) > displayLimit {
		fmt.Printf("  … 共 %d 个版本(仅列前 %d 个)\n", len(tags), displayLimit)
	}
	fmt.Print("选择版本序号(回车 = 1 最新): ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return tags[0], nil
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(tags) {
		return "", fmt.Errorf("无效的序号 %q(可选 1~%d)", line, len(tags))
	}
	return tags[idx-1], nil
}

// switchCmd 切换到指定版本 tag,或交互选择版本。
func switchCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "switch [tag]",
		Short: "切换 DSH 源码仓库到指定版本 tag",
		Long:  "检出到指定版本 tag 并重新安装依赖、构建。不带 tag 时进入交互选择;支持 dsh-v0.1.2-alpha.1 或 v0.1.1-rc.2 两种写法。",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}
			return runSwitch(arg, yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "跳过确认,直接切换")
	return cmd
}

// runSwitch 执行版本切换:仅本地变更(checkout/install/build),不 commit/push。
func runSwitch(arg string, yes bool) error {
	repo, err := repoRoot()
	if err != nil {
		return err
	}
	fmt.Println("使用仓库:", repo)
	fmt.Println("拉取远程 tags: git fetch --tags --prune origin")
	if err := runCmd(repo, "git", "fetch", "--tags", "--prune", "origin"); err != nil {
		fmt.Println("警告: 无法连接远程仓库,将基于本地已有 tag 判断")
	}
	tags, err := listVersionTags(repo)
	if err != nil {
		return fmt.Errorf("读取 tag 列表失败: %w", err)
	}
	if len(tags) == 0 {
		return errors.New("未找到任何版本 tag")
	}
	var target string
	if arg == "" {
		target, err = selectTag(tags, os.Stdin)
		if err != nil {
			return err
		}
	} else {
		target = resolveSwitchTag(arg, tags)
		if target == "" {
			fmt.Printf("找不到版本 tag %q。\n", arg)
			if !askYN("是否从可用版本中选择一个?") {
				fmt.Println("已取消。")
				return nil
			}
			target, err = selectTag(tags, os.Stdin)
			if err != nil {
				return err
			}
		}
	}
	if currentOnTag(repo, target) {
		fmt.Println("当前已在该版本:", target)
		return nil
	}
	dirty, _ := gitOut(repo, "status", "--porcelain")
	if strings.TrimSpace(dirty) != "" {
		return errors.New("工作区存在未提交的改动,已中止。请先 git stash 或提交后重试")
	}
	fmt.Println("切换目标:", target)
	if !yes {
		if !askYN("是否切换到 " + target + "?") {
			fmt.Println("已取消,未做任何更改。")
			return nil
		}
	}
	fmt.Println()
	fmt.Println("切换到版本:", target)
	if err := runCmd(repo, "git", "checkout", target); err != nil {
		return err
	}
	fmt.Println("安装依赖: pnpm install")
	if err := runCmd(repo, "pnpm", "install"); err != nil {
		return fmt.Errorf("pnpm install 失败: %w", err)
	}
	fmt.Println("重新构建: pnpm run build (可能需要几分钟)")
	if err := runCmd(repo, "pnpm", "run", "build"); err != nil {
		return fmt.Errorf("pnpm run build 失败: %w", err)
	}
	fmt.Println()
	if askYN("切换完成。是否启动服务?") {
		_ = stopService()
		return startService(repo, nil, nil)
	}
	return nil
}
