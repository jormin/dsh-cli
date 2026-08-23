package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// errPNPMLsFailed 标记 pnpm ls 主路径失败,触发 node_modules 回退。
var errPNPMLsFailed = errors.New("dsh plugin ls 失败")

// Installed 是一条本机实装记录。
type Installed struct {
	Name    string
	Version string
}

// CmdRunner 抽象外部命令;产品默认 exec.Command,测试注入 fake。
type CmdRunner func(dir, name string, args ...string) ([]byte, error)

// execRunner 调用 exec.Command 并合并 stdout/stderr。
func execRunner(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return out, err
}

// LocalScanner 扫描本机已存在的 profile,返回实装插件列表。
type LocalScanner struct {
	Repo string // DSH_REPO,用于 pnpm -C <repo> dsh plugin 透传
	Home string // DSH home(解析后的绝对路径)
	Run  CmdRunner
}

// lsOutput 对应 pnpm ls --json 的顶层项目对象(只取 dependencies 的直接依赖)。
type lsOutput struct {
	Dependencies map[string]struct {
		Version string `json:"version"`
	} `json:"dependencies"`
}

// List 返回指定 profile 的实装清单;profile 目录不存在则返回空列表。
func (s LocalScanner) List(profile string) ([]Installed, error) {
	dir := profileDir(s.Home, profile)
	if _, err := os.Stat(filepath.Join(dir, "package.json")); os.IsNotExist(err) {
		return nil, nil
	}
	list, err := s.listViaPluginCmd(profile)
	if err == nil {
		return list, nil
	}
	return s.listViaNodeModules(dir)
}

// listViaPluginCmd 主路径:dsh plugin --profile <n> ls --depth 0 --json。
// pnpm/脚本可能输出额外行,先截取首个 [ 到末个 ] 再做 JSON 解析。
func (s LocalScanner) listViaPluginCmd(profile string) ([]Installed, error) {
	out, err := s.Run("", "pnpm", "-C", s.Repo, "dsh", "plugin", "--profile", profile,
		"ls", "--depth", "0", "--json")
	if err != nil {
		return nil, errPNPMLsFailed
	}
	start := bytes.IndexByte(out, '[')
	end := bytes.LastIndexByte(out, ']')
	if start < 0 || end <= start {
		return nil, errPNPMLsFailed
	}
	var outs []lsOutput
	if err := json.Unmarshal(out[start:end+1], &outs); err != nil {
		return nil, errPNPMLsFailed
	}
	var list []Installed
	for _, o := range outs {
		for name, dep := range o.Dependencies {
			list = append(list, Installed{Name: name, Version: dep.Version})
		}
	}
	return list, nil
}

// listViaNodeModules 回退:只读 profile package.json 的 direct dependencies,
// 再到 node_modules 取各自实装版本(与主路径 pnpm ls --depth 0 口径一致,
// 不会把 hoisted 提升的传递依赖混入清单)。
func (s LocalScanner) listViaNodeModules(profileRoot string) ([]Installed, error) {
	data, err := os.ReadFile(filepath.Join(profileRoot, "package.json"))
	if err != nil {
		return nil, errPNPMLsFailed
	}
	var pm struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, errPNPMLsFailed
	}
	var list []Installed
	for name := range pm.Dependencies {
		pkgJSON := filepath.Join(profileRoot, "node_modules", filepath.FromSlash(name), "package.json")
		d, err := os.ReadFile(pkgJSON)
		if err != nil {
			continue // 声明但未实装:跳过
		}
		var p struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(d, &p) == nil && p.Version != "" {
			list = append(list, Installed{Name: name, Version: p.Version})
		}
	}
	return list, nil
}

// All 汇总所有存在的 profile 的实装清单。
func (s LocalScanner) All() (map[string][]Installed, error) {
	profilesRoot := filepath.Join(s.Home, "profiles")
	entries, err := os.ReadDir(profilesRoot)
	if os.IsNotExist(err) {
		return map[string][]Installed{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]Installed{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		list, err := s.List(e.Name())
		if err != nil {
			return nil, err
		}
		out[e.Name()] = list
	}
	return out, nil
}
