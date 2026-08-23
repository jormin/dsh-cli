package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginRef 是插件清单中的一条记录:插件名 + 实装(解析)版本。
type PluginRef struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Manifest 是 plugins.yaml 的根结构,按 profile 分组。
type Manifest struct {
	Profiles map[string][]PluginRef `yaml:"profiles"`
}

// Normalize 返回排序、去重后的副本;空 Profile 键折叠为空列表。
func (m Manifest) Normalize() Manifest {
	out := Manifest{Profiles: map[string][]PluginRef{}}
	for profile, list := range m.Profiles {
		byName := map[string]PluginRef{}
		for _, p := range list {
			if p.Name == "" {
				continue
			}
			byName[p.Name] = PluginRef{Name: p.Name, Version: strings.TrimSpace(p.Version)}
		}
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)
		refs := make([]PluginRef, 0, len(names))
		for _, n := range names {
			refs = append(refs, byName[n])
		}
		if len(refs) > 0 {
			out.Profiles[profile] = refs // 空列表与"无此 profile"等价,统一折叠
		}
	}
	return out
}

// Equal 比较两份清单(忽略顺序,语义等价)。
func (m Manifest) Equal(o Manifest) bool {
	a, b := m.Normalize(), o.Normalize()
	if len(a.Profiles) != len(b.Profiles) {
		return false
	}
	for profile, refs := range a.Profiles {
		bRefs, ok := b.Profiles[profile]
		if !ok || len(refs) != len(bRefs) {
			return false
		}
		for i := range refs {
			if refs[i] != bRefs[i] {
				return false
			}
		}
	}
	return true
}

// ReadManifest 读取 plugins.yaml;文件不存在返回显式错误(由调用方决定初始化)。
func ReadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	if m.Profiles == nil {
		m.Profiles = map[string][]PluginRef{}
	}
	return m, nil
}

// manifestHeader 说明该文件由 dsh sync 托管,人工修改会被覆盖。
const manifestHeader = "# auto-managed by dsh sync;manual edits are overwritten on next sync\n"

// WriteManifest 原子写盘(临时文件 + rename),内容为 Normalize 后的规范形式。
func WriteManifest(path string, m Manifest) error {
	m = m.Normalize()
	body, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	data := append([]byte(manifestHeader), body...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".plugins-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil { // 避免 0600 造成 git mode 噪声
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// emptyManifest 返回"空清单"(本机无任何 profile 或仓库无文档时使用)。
func emptyManifest() Manifest { return Manifest{Profiles: map[string][]PluginRef{}} }

// dshHome 解析 DSH_HOME,未设置时回退 ~/.dsh(与 DSH 源码 resolveDshHome 一致)。
func dshHome() string {
	if v := strings.TrimSpace(os.Getenv("DSH_HOME")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".dsh"
	}
	return filepath.Join(home, ".dsh")
}

// profileDir 返回指定 profile 的目录。
func profileDir(home, name string) string { return filepath.Join(home, "profiles", name) }
