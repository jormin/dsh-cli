package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	out []byte
	err error
	cmd []string // 最后一条命令
}

func (f *fakeRunner) Run(dir, name string, args ...string) ([]byte, error) {
	f.cmd = append([]string{name}, args...)
	return f.out, f.err
}

const lsJSON = "[\n" +
	"  {\"name\":\"dsh-profile-web\",\"version\":\"0.0.0\",\"path\":\"/x/web\",\"private\":true,\n" +
	"   \"dependencies\":{\n" +
	"     \"@linxin666/dsh-liangshen\":{\"version\":\"0.2.8\"},\n" +
	"     \"@linxin666/dsh-client-ui-git-graph\":{\"version\":\"0.2.8\"}\n" +
	"   }}\n" +
	"]\n"

func TestLocalScannerPrimary(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\"}")
	f := &fakeRunner{out: []byte(lsJSON)}
	s := LocalScanner{Repo: "/repo", Home: home, Run: f.Run}
	got, err := s.List("web")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{}
	for _, ins := range got {
		byName[ins.Name] = ins.Version
	}
	want := map[string]string{
		"@linxin666/dsh-liangshen":           "0.2.8",
		"@linxin666/dsh-client-ui-git-graph": "0.2.8",
	}
	if len(byName) != len(want) {
		t.Fatalf("unexpected list: %+v", got)
	}
	for n, v := range want {
		if byName[n] != v {
			t.Fatalf("plugin %s version %q,want %q", n, byName[n], v)
		}
	}
	wantArgs := "pnpm -C /repo dsh plugin --profile web ls --depth 0 --json"
	if strings.Join(f.cmd, " ") != wantArgs {
		t.Fatalf("args = %q,want %q", strings.Join(f.cmd, " "), wantArgs)
	}
}

// 回退只读直接依赖,hoisted 的传递依赖不得混入。
func TestLocalScannerFallbackOnError(t *testing.T) {
	home := t.TempDir()
	mkPackageJSON(t, home, "web", "{\"name\":\"dsh-profile-web\",\"dependencies\":{\"@linxin666/dsh-liangshen\":\"^0.2.8\",\"plain-lib\":\"^1.0.0\"}}")
	mkNodePkg(t, home, "web", "@linxin666/dsh-liangshen", "@linxin666/dsh-liangshen", "0.2.8")
	mkNodePkg(t, home, "web", "plain-lib", "plain-lib", "1.0.0")
	// hoisted 传递依赖:不在 dependencies 里,必须被忽略
	mkNodePkg(t, home, "web", "transitive-dep", "transitive-dep", "9.9.9")

	f := &fakeRunner{err: errPNPMLsFailed}
	got, err := LocalScanner{Repo: "/repo", Home: home, Run: f.Run}.List("web")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("fallback scan must return only direct deps, got %+v", got)
	}
}

func TestLocalScannerEmptyProfile(t *testing.T) {
	f := &fakeRunner{}
	got, err := LocalScanner{Repo: "/repo", Home: t.TempDir(), Run: f.Run}.List("ghost")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}

func mkPackageJSON(t *testing.T, home, profile, body string) {
	t.Helper()
	dir := filepath.Join(home, "profiles", profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkNodePkg(t *testing.T, home, profile, name, pkgName, version string) {
	t.Helper()
	dir := filepath.Join(home, "profiles", profile, "node_modules", filepath.FromSlash(name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{\"name\":\"" + pkgName + "\",\"version\":\"" + version + "\"}"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
