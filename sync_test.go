package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failRunner struct {
	called bool
}

func (r *failRunner) Run(string, string, ...string) ([]byte, error) {
	r.called = true
	return nil, nil
}

func noopRunner(_ string, _ string, _ ...string) ([]byte, error) {
	return nil, nil
}

// noopHooks 测试用服务 hooks:未运行、操作空实现。
var noopHooks = &serviceHooks{
	busy:    func() bool { return false },
	restart: func(string) error { return nil },
	start:   func(string) error { return nil },
}

func TestValidateEnv(t *testing.T) {
	if _, err := syncEnv("", "/repo"); err == nil || !strings.Contains(err.Error(), "DSH_REPO") {
		t.Fatalf("repo 空 err=%v", err)
	}
	if _, err := syncEnv("/repo", ""); err == nil || !strings.Contains(err.Error(), "DSH_SYNC_REPO") {
		t.Fatalf("syncRepo 空 err=%v", err)
	}
}

func TestRunSyncStatusReadOnly(t *testing.T) {
	home := t.TempDir() // 无 profile:扫描不会调用外部命令
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	var runner failRunner
	err := runSync(statusOnly, "/repo", syncRepo, home, &out, runner.Run, noopHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if runner.called {
		t.Fatal("空 home 下 status 不应执行外部命令")
	}
	if !strings.Contains(out.String(), "无差异") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestRunSyncFullYesNoChanges(t *testing.T) {
	home := t.TempDir()
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err := runSync(fullSync|flagYes, "/repo", syncRepo, home, &out, noopRunner, noopHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(syncRepo, "plugins.yaml")); !os.IsNotExist(statErr) {
		t.Fatal("无差异时不应写 plugins.yaml")
	}
	if !strings.Contains(out.String(), "本地无变更") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestDecisionFinalBulk(t *testing.T) {
	remote := Manifest{Profiles: map[string][]PluginRef{
		"web": {{Name: "both", Version: "9.0.0"}, {Name: "r/only", Version: "2.0.0"}},
	}}
	local := map[string][]Installed{
		"web": {{Name: "both", Version: "1.0.0"}, {Name: "l/only", Version: "0.9.0"}},
	}
	gotRemote := decisionFinalBulk(remote, local, false)
	if !gotRemote.Equal(remote) {
		t.Fatalf("prefer remote got %+v,want remote", gotRemote)
	}
	wantLocal := Manifest{Profiles: map[string][]PluginRef{
		"web": {{Name: "both", Version: "1.0.0"}, {Name: "l/only", Version: "0.9.0"}},
	}}
	gotLocal := decisionFinalBulk(remote, local, true)
	if !gotLocal.Equal(wantLocal) {
		t.Fatalf("prefer local got %+v,want local", gotLocal)
	}
}

// 全流程 --yes --prefer remote:有差异时写回仓库并 apply。
func TestRunSyncFullYesAppliesRemote(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "profiles:\n  web:\n    - name: both\n      version: \"9.0.0\"\n    - name: r/only\n      version: \"2.0.0\"\n"
	if err := os.WriteFile(filepath.Join(syncRepo, "plugins.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"both\":{\"version\":\"1.0.0\"},\"l/only\":{\"version\":\"0.9.0\"}}}]"}
	var out strings.Builder
	err := runSync(fullSync|flagYes, "/repo", syncRepo, home, &out, rec.run, noopHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	// git 流程:至少经历 pull;清单与仓库一致时应跳过写回/commit
	if !rec.contains("git", "-C", syncRepo, "pull") {
		t.Fatalf("missing git pull:%v", rec.cmds)
	}
	if rec.contains("git", "-C", syncRepo, "add", "-A") {
		t.Fatalf("清单与仓库一致时不应 commit:%v", rec.cmds)
	}
	if !strings.Contains(out.String(), "清单无变化,跳过写回。") {
		t.Fatalf("output: %s", out.String())
	}
	for _, want := range [][]string{
		{"pnpm", "-C", "/repo", "dsh", "plugin", "--profile", "web", "add", "--save-exact", "both@9.0.0"},
		{"pnpm", "-C", "/repo", "dsh", "plugin", "--profile", "web", "add", "--save-exact", "r/only@2.0.0"},
		{"pnpm", "-C", "/repo", "dsh", "plugin", "--profile", "web", "remove", "l/only"},
	} {
		if !rec.contains(want...) {
			t.Fatalf("missing apply cmd %v in %v", want, rec.cmds)
		}
	}
	onDisk, err := ReadManifest(filepath.Join(syncRepo, "plugins.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := Manifest{Profiles: map[string][]PluginRef{
		"web": {{Name: "both", Version: "9.0.0"}, {Name: "r/only", Version: "2.0.0"}},
	}}
	if !onDisk.Equal(wantManifest) {
		t.Fatalf("on-disk manifest mismatch: %+v", onDisk)
	}
	if !strings.Contains(out.String(), "未在运行") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestRunSyncStatusMissingManifestWarns(t *testing.T) {
	home := t.TempDir() // 无 profile
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	var runner failRunner
	err := runSync(statusOnly, "/repo", syncRepo, home, &out, runner.Run, noopHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "没有 plugins.yaml") {
		t.Fatalf("missing warning: %s", out.String())
	}
}

// 交互模式 + 缺文件:选 y → 用本机清单播种,不做任何插件调整。
func TestRunSyncInteractiveInitSeeds(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"both\":{\"version\":\"1.0.0\"},\"l/only\":{\"version\":\"0.9.0\"}}}]", remoteEmpty: true}
	var out strings.Builder
	d := NewDecider(strings.NewReader("y\n"), &out)
	err := runSync(fullSync, "/repo", syncRepo, home, &out, rec.run, noopHooks, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "初始化") {
		t.Fatalf("missing init message: %s", out.String())
	}
	if !strings.Contains(out.String(), "跳过 git pull") {
		t.Fatalf("空远端应提示跳过 pull: %s", out.String())
	}
	if rec.contains("git", "-C", syncRepo, "pull") {
		t.Fatalf("空远端不应执行 git pull:%v", rec.cmds)
	}
	if !rec.contains("git", "-C", syncRepo, "add", "-A") {
		t.Fatalf("seed 应写回并 commit:%v", rec.cmds)
	}
	for _, c := range rec.cmds {
		if len(c) > 2 && c[0] == "pnpm" && (c[7] == "add" || c[7] == "remove") {
			t.Fatalf("播种不应调整插件:%v", c)
		}
	}
	onDisk, err := ReadManifest(filepath.Join(syncRepo, "plugins.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := Manifest{Profiles: map[string][]PluginRef{
		"web": {{Name: "both", Version: "1.0.0"}, {Name: "l/only", Version: "0.9.0"}},
	}}
	if !onDisk.Equal(want) {
		t.Fatalf("seed manifest mismatch: %+v", onDisk)
	}
}

// --yes 缺文件:不询问,仓库保持空 → 本机插件被移除。
func TestRunSyncFullYesMissingManifestRemovesAll(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"both\":{\"version\":\"1.0.0\"},\"l/only\":{\"version\":\"0.9.0\"}}}]"}
	var out strings.Builder
	err := runSync(fullSync|flagYes, "/repo", syncRepo, home, &out, rec.run, noopHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "没有 plugins.yaml") {
		t.Fatalf("missing warning: %s", out.String())
	}
	if !rec.contains("pnpm", "-C", "/repo", "dsh", "plugin", "--profile", "web", "remove", "both") {
		t.Fatalf("missing remove both:%v", rec.cmds)
	}
	if !rec.contains("pnpm", "-C", "/repo", "dsh", "plugin", "--profile", "web", "remove", "l/only") {
		t.Fatalf("missing remove l/only:%v", rec.cmds)
	}
	if _, statErr := os.Stat(filepath.Join(syncRepo, "plugins.yaml")); !os.IsNotExist(statErr) {
		t.Fatal("仓库保持空时不应写 plugins.yaml")
	}
}

// 交互模式 + 缺文件:选 n 后全局选 2 → 按仓库(空)处理,移除本机插件。
func TestRunSyncInteractiveDeclineInit(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"both\":{\"version\":\"1.0.0\"}}}]"}
	var out strings.Builder
	d := NewDecider(strings.NewReader("n\n2\n"), &out)
	err := runSync(fullSync, "/repo", syncRepo, home, &out, rec.run, noopHooks, d)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.contains("pnpm", "-C", "/repo", "dsh", "plugin", "--profile", "web", "remove", "both") {
		t.Fatalf("decline 后应按仓库处理并移除:%v", rec.cmds)
	}
	if _, statErr := os.Stat(filepath.Join(syncRepo, "plugins.yaml")); !os.IsNotExist(statErr) {
		t.Fatal("decline 后不应写 plugins.yaml")
	}
}

// 交互模式 + 有变更 + web 运行中:询问重启,选 y 则重启。
func TestRunSyncInteractiveAskRestart(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "profiles:\n  web: []\n"
	if err := os.WriteFile(filepath.Join(syncRepo, "plugins.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	restarted := false
	started := false
	hooks := &serviceHooks{
		busy: func() bool { return true },
		restart: func(string) error {
			restarted = true
			return nil
		},
		start: func(string) error {
			started = true
			return nil
		},
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"l/only\":{\"version\":\"0.9.0\"}}}]"}
	var out strings.Builder
	d := NewDecider(strings.NewReader("2\ny\n"), &out)
	err := runSync(fullSync, "/repo", syncRepo, home, &out, rec.run, hooks, d)
	if err != nil {
		t.Fatal(err)
	}
	if !restarted || started {
		t.Fatalf("restarted=%v started=%v,want 重启且不启动", restarted, started)
	}
	if !strings.Contains(out.String(), "是否重启 web 服务?") || !strings.Contains(out.String(), "已重启。") {
		t.Fatalf("output: %s", out.String())
	}
}

// 交互模式 + 有变更 + web 未运行:询问启动,选 y 则启动。
func TestRunSyncInteractiveAskStart(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "profiles:\n  web: []\n"
	if err := os.WriteFile(filepath.Join(syncRepo, "plugins.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	restarted := false
	started := false
	hooks := &serviceHooks{
		busy: func() bool { return false },
		restart: func(string) error {
			restarted = true
			return nil
		},
		start: func(string) error {
			started = true
			return nil
		},
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"l/only\":{\"version\":\"0.9.0\"}}}]"}
	var out strings.Builder
	d := NewDecider(strings.NewReader("2\ny\n"), &out)
	err := runSync(fullSync, "/repo", syncRepo, home, &out, rec.run, hooks, d)
	if err != nil {
		t.Fatal(err)
	}
	if started != true || restarted {
		t.Fatalf("started=%v restarted=%v,want 启动且不重启", started, restarted)
	}
	if !strings.Contains(out.String(), "是否启动 web 服务?") || !strings.Contains(out.String(), "已启动。") {
		t.Fatalf("output: %s", out.String())
	}
}

// 交互模式 + 有变更 + 拒绝启动:提示手动命令,不调用启动。
func TestRunSyncInteractiveDeclineStart(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "profiles:\n  web: []\n"
	if err := os.WriteFile(filepath.Join(syncRepo, "plugins.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	started := false
	hooks := &serviceHooks{
		busy:    func() bool { return false },
		restart: func(string) error { return nil },
		start: func(string) error {
			started = true
			return nil
		},
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"l/only\":{\"version\":\"0.9.0\"}}}]"}
	var out strings.Builder
	d := NewDecider(strings.NewReader("2\nn\n"), &out)
	err := runSync(fullSync, "/repo", syncRepo, home, &out, rec.run, hooks, d)
	if err != nil {
		t.Fatal(err)
	}
	if started {
		t.Fatalf("拒绝后不应启动:%v", rec.cmds)
	}
	if !strings.Contains(out.String(), "已取消启动") {
		t.Fatalf("output: %s", out.String())
	}
}

// 交互模式 + 两侧无差异:不进入任何选择,直接结束。
func TestRunSyncInteractiveNoDiffSkipsPrompt(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	syncRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "profiles:\n  web:\n    - name: both\n      version: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(syncRepo, "plugins.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &recorderRunner{lsJSON: "[{\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\"dependencies\":{\"both\":{\"version\":\"1.0.0\"}}}]"}
	var out strings.Builder
	d := NewDecider(strings.NewReader(""), &out) // 无输入:若代码询问则会读到 EOF
	err := runSync(fullSync, "/repo", syncRepo, home, &out, rec.run, noopHooks, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "两侧无差异") {
		t.Fatalf("output: %s", out.String())
	}
	if strings.Contains(out.String(), "全局选择") {
		t.Fatalf("无差异不应进入全局选择:\n%s", out.String())
	}
}

// recorderRunner 记录命令;命中 ls 时返回 fixture;remoteEmpty 时 ls-remote 返回空(空远端)。
type recorderRunner struct {
	cmds        [][]string
	lsJSON      string
	remoteEmpty bool
}

func (r *recorderRunner) run(dir, name string, args ...string) ([]byte, error) {
	full := append([]string{name}, args...)
	r.cmds = append(r.cmds, full)
	for _, a := range args {
		if a == "ls" {
			return []byte(r.lsJSON), nil
		}
		if a == "ls-remote" {
			if r.remoteEmpty {
				return nil, nil
			}
			return []byte("refs/heads/main\n"), nil
		}
	}
	return nil, nil
}

func (r *recorderRunner) contains(parts ...string) bool {
	for _, c := range r.cmds {
		if len(c) != len(parts) {
			continue
		}
		ok := true
		for i := range parts {
			if c[i] != parts[i] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
