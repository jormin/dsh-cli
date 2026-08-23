package main

import (
	"os"
	"path/filepath"
	"testing"
)

type fakeGitRunner struct {
	cmds    [][]string
	err     error // 所有命令报错
	pullErr error // 仅 git pull 报错
	lsEmpty bool  // ls-remote 返回空(空远端)
}

func (f *fakeGitRunner) Run(dir, name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	if f.err != nil {
		return nil, f.err
	}
	for _, a := range args {
		if a == "ls-remote" {
			if f.lsEmpty {
				return nil, nil
			}
			return []byte("refs/heads/main\n"), nil
		}
	}
	if f.pullErr != nil {
		for _, a := range args {
			if a == "pull" {
				return nil, f.pullErr
			}
		}
	}
	return nil, nil
}

func TestGitOpsCommands(t *testing.T) {
	f := &fakeGitRunner{}
	g := GitOps{Repo: "/sync", Run: f.Run}
	if _, err := g.Pull(); err != nil {
		t.Fatal(err)
	}
	if err := g.CommitAll("sync: update plugin manifest"); err != nil {
		t.Fatal(err)
	}
	if err := g.Push(); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"git", "-C", "/sync", "ls-remote", "--heads", "origin"},
		{"git", "-C", "/sync", "pull"},
		{"git", "-C", "/sync", "add", "-A"},
		{"git", "-C", "/sync", "commit", "-m", "sync: update plugin manifest"},
		{"git", "-C", "/sync", "push", "-u", "origin", "HEAD"},
	}
	if len(f.cmds) != len(want) {
		t.Fatalf("cmd count=%d want=%d (%v)", len(f.cmds), len(want), f.cmds)
	}
	for i := range want {
		for j := range want[i] {
			if f.cmds[i][j] != want[i][j] {
				t.Fatalf("cmd[%d][%d]=%q want=%q", i, j, f.cmds[i][j], want[i][j])
			}
		}
	}
}

func TestGitOpsPullConflictAborts(t *testing.T) {
	f := &fakeGitRunner{pullErr: os.ErrNotExist}
	g := GitOps{Repo: "/sync", Run: f.Run}
	if _, err := g.Pull(); err == nil {
		t.Fatal("expected pull error")
	}
}

func TestGitOpsPullEmptyRemoteSkips(t *testing.T) {
	f := &fakeGitRunner{lsEmpty: true}
	g := GitOps{Repo: "/sync", Run: f.Run}
	pulled, err := g.Pull()
	if err != nil {
		t.Fatal(err)
	}
	if pulled {
		t.Fatal("空远端不应执行拉取")
	}
	for _, c := range f.cmds {
		for _, a := range c {
			if a == "pull" {
				t.Fatalf("空远端不应执行 git pull:%v", f.cmds)
			}
		}
	}
}

func TestIsGitRepo(t *testing.T) {
	dir := t.TempDir()
	if (GitOps{Repo: dir, Run: execRunner}).IsRepo() {
		t.Fatal("empty dir must not be a repo")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !(GitOps{Repo: dir, Run: execRunner}).IsRepo() {
		t.Fatal("dir with .git must be a repo")
	}
}
