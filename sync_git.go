package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GitOps 在 DSH_SYNC_REPO 上执行 git 操作;所有命令只带 -C <repo>,不切工作目录。
type GitOps struct {
	Repo string
	Run  CmdRunner
}

// ErrNotGitRepo 表示同步目录不是 git 仓库。
var ErrNotGitRepo = errors.New("DSH_SYNC_REPO 不是 git 仓库,请先 git clone")

// IsRepo 判断目录是否为 git 仓库(存在 .git)。
func (g GitOps) IsRepo() bool {
	st, err := os.Stat(filepath.Join(g.Repo, ".git"))
	return err == nil && st.IsDir()
}

// Pull 拉取远端并合并;失败(含冲突)由调用方中止流程。
func (g GitOps) Pull() error {
	out, err := g.Run("", "git", "-C", g.Repo, "pull")
	if err != nil {
		return fmt.Errorf("git pull 失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CommitAll 提交全部变更(仅 DSH_SYNC_REPO 工作树)。
func (g GitOps) CommitAll(msg string) error {
	if out, err := g.Run("", "git", "-C", g.Repo, "add", "-A"); err != nil {
		return fmt.Errorf("git add 失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	if out, err := g.Run("", "git", "-C", g.Repo, "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit 失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Push 推送当前分支。
func (g GitOps) Push() error {
	out, err := g.Run("", "git", "-C", g.Repo, "push")
	if err != nil {
		return fmt.Errorf("git push 失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
