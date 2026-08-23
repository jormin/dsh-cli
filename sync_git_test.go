package main

import (
	"os"
	"path/filepath"
	"testing"
)

type fakeGitRunner struct {
	cmds [][]string
	err  error
}

func (f *fakeGitRunner) Run(dir, name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func TestGitOpsCommands(t *testing.T) {
	f := &fakeGitRunner{}
	g := GitOps{Repo: "/sync", Run: f.Run}
	if err := g.Pull(); err != nil {
		t.Fatal(err)
	}
	if err := g.CommitAll("sync: update plugin manifest"); err != nil {
		t.Fatal(err)
	}
	if err := g.Push(); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"git", "-C", "/sync", "pull"},
		{"git", "-C", "/sync", "add", "-A"},
		{"git", "-C", "/sync", "commit", "-m", "sync: update plugin manifest"},
		{"git", "-C", "/sync", "push"},
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
	f := &fakeGitRunner{err: os.ErrNotExist}
	g := GitOps{Repo: "/sync", Run: f.Run}
	if err := g.Pull(); err == nil {
		t.Fatal("expected pull error")
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
