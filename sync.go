package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// syncMode 位标志。
type syncMode int

const (
	statusOnly syncMode = 1 << iota
	fullSync
	flagYes
	flagPreferLocal
)

// syncCtx 解析后的同步环境(便于测试注入)。
type syncCtx struct {
	repo, syncRepo, home string
}

// serviceHooks 服务生命周期操作(测试注入;nil 使用产品实现)。
type serviceHooks struct {
	busy    func() bool
	restart func(repo string) error
	start   func(repo string) error
}

// syncEnv 校验并解析同步环境;home 可选(默认 DSH_HOME 或 ~/.dsh)。
func syncEnv(repo, syncRepo string, home ...string) (syncCtx, error) {
	ctx := syncCtx{repo: repo, syncRepo: syncRepo, home: dshHome()}
	if len(home) > 0 && home[0] != "" {
		ctx.home = home[0]
	}
	if repo == "" {
		return ctx, errors.New("未检测到 DSH_REPO 环境变量")
	}
	if syncRepo == "" {
		return ctx, errors.New("未检测到 DSH_SYNC_REPO 环境变量(指向插件清单 git 仓库目录)")
	}
	return ctx, nil
}

// defaultRunner 是产品默认命令执行器(exec.Command)。
var defaultRunner CmdRunner = execRunner

// runSync 编排完整流程;run 为外部命令执行器(产品默认 defaultRunner,测试注入 fake);
// busy 为"web 是否运行中"探针(产品默认 portBusy,测试注入常量)。
func runSync(mode syncMode, repo, syncRepo, home string, out io.Writer, run CmdRunner, svc *serviceHooks, d *Decider) error {
	env, err := syncEnv(repo, syncRepo, home)
	if err != nil {
		return err
	}
	if run == nil {
		run = defaultRunner
	}
	if svc == nil {
		svc = &serviceHooks{
			busy:    portBusy,
			restart: restartWeb,
			start: func(repo string) error {
				return startService(repo, nil, nil)
			},
		}
	}
	if d == nil {
		d = NewDecider(os.Stdin, out)
	}
	git := GitOps{Repo: env.syncRepo, Run: run}
	if !git.IsRepo() {
		return ErrNotGitRepo
	}
	scanner := LocalScanner{Repo: env.repo, Home: env.home, Run: run}
	// 先 pull 再扫描本地,保证与本机实装状态一致(plugins.yaml 损坏时也会先暴露在 pull 之后)
	if mode&statusOnly == 0 {
		pulled, err := git.Pull()
		if err != nil {
			return fmt.Errorf("已中止,请先手动解决同步仓库的 pull 失败后重试: %w", err)
		}
		if !pulled {
			fmt.Fprintln(out, "(同步仓库远端为空,已跳过 git pull,可直接首次初始化)")
		}
	}
	local, err := scanner.All()
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(env.syncRepo, "plugins.yaml")
	remote, existed, err := readManifestExist(manifestPath)
	if err != nil {
		return err
	}
	if !existed {
		fmt.Fprintln(out, "⚠️ DSH_SYNC_REPO 中没有 plugins.yaml,当前按\"仓库为空清单\"处理")
	}

	if mode&statusOnly != 0 {
		return renderStatus(out, remote, local)
	}

	items := Compare(remote, local)
	fmt.Fprintln(out, RenderTable(onlyDiffs(items), sameCountByProfile(items)))

	// 4) 决策:文件缺失且本机有插件时,先问是否用本机清单播种;--yes 保持非交互
	var final Manifest
	seeded := false
	if !existed && mode&flagYes == 0 && hasAnyPlugin(local) {
		ok, err := d.YesNo("检测到 DSH_SYNC_REPO 中没有 plugins.yaml,是否用本机插件清单初始化仓库?")
		if err != nil {
			return err
		}
		if ok {
			final = decisionFinalBulk(remote, local, true)
			seeded = true
		}
	}
	if !seeded {
		if mode&flagYes != 0 {
			final = decisionFinalBulk(remote, local, mode&flagPreferLocal != 0)
		} else {
			final, err = decideInteractive(d, out, remote, local)
			if err != nil {
				return err
			}
		}
	}

	// 5) 写回 + commit + push(仅 DSH_SYNC_REPO);清单无变化则跳过
	if !final.Equal(remote) {
		if err := WriteManifest(manifestPath, final); err != nil {
			return err
		}
		if err := git.CommitAll("sync: update plugin manifest"); err != nil {
			return fmt.Errorf("commit 失败: %w", err)
		}
		if err := git.Push(); err != nil {
			return fmt.Errorf("push 失败(本地已 commit,解决远端冲突后重试): %w", err)
		}
		if seeded {
			fmt.Fprintln(out, "已用本机清单初始化 DSH_SYNC_REPO(plugins.yaml 已写入并推送)。")
		} else {
			fmt.Fprintln(out, "plugins.yaml 已同步到 DSH_SYNC_REPO。")
		}
	} else {
		fmt.Fprintln(out, "清单无变化,跳过写回。")
	}

	// 6) apply;7) 启动/重启询问
	changed, errs := ApplyChanges(env.repo, final, local, run, out)
	if len(errs) > 0 {
		return fmt.Errorf("部分插件调整失败(已成功的保留):\n%v", errs)
	}
	if !changed {
		fmt.Fprintln(out, "本地无变更,未做调整。")
		return nil
	}
	if mode&flagYes != 0 {
		// --yes:非交互,维持自动行为
		if decideRestart(changed, svc.busy) {
			fmt.Fprintln(out, "插件已变更,重启 web 服务…")
			if err := svc.restart(env.repo); err != nil {
				return err
			}
			fmt.Fprintln(out, "已重启。")
			return nil
		}
		fmt.Fprintln(out, "插件已调整,web 未在运行,可执行 dsh web start 启动。")
		return nil
	}
	// 交互:有变更时询问启动/重启,措辞随当前运行状态
	if svc.busy() {
		ok, err := d.YesNo("插件已变更,是否重启 web 服务?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "已取消重启。可稍后手动运行 dsh web restart。")
			return nil
		}
		fmt.Fprintln(out, "重启 web 服务…")
		if err := svc.restart(env.repo); err != nil {
			return err
		}
		fmt.Fprintln(out, "已重启。")
		return nil
	}
	ok, err := d.YesNo("插件已变更,是否启动 web 服务?")
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "已取消启动。可稍后手动运行 dsh web start。")
		return nil
	}
	fmt.Fprintln(out, "启动 web 服务…")
	if err := svc.start(env.repo); err != nil {
		return err
	}
	fmt.Fprintln(out, "已启动。")
	return nil
}

// readManifestExist 读取清单;existed=false 表示文件不存在(视为空清单)。
func readManifestExist(path string) (Manifest, bool, error) {
	remote, err := ReadManifest(path)
	if os.IsNotExist(err) {
		return emptyManifest(), false, nil
	}
	return remote, true, err
}

// hasAnyPlugin 判断本机任意 profile 是否装有插件。
func hasAnyPlugin(local map[string][]Installed) bool {
	for _, list := range local {
		if len(list) > 0 {
			return true
		}
	}
	return false
}

func onlyDiffs(items []DiffItem) []DiffItem {
	out := items[:0]
	for _, it := range items {
		if it.Type != DiffSame {
			out = append(out, it)
		}
	}
	return out
}

func sameCountByProfile(items []DiffItem) map[string]int {
	m := map[string]int{}
	for _, it := range items {
		if it.Type == DiffSame {
			m[it.Profile]++
		}
	}
	return m
}

func renderStatus(out io.Writer, remote Manifest, local map[string][]Installed) error {
	items := Compare(remote, local)
	fmt.Fprintln(out, RenderTable(onlyDiffs(items), sameCountByProfile(items)))
	return nil
}

// decisionFinalBulk --yes 批量决策:一侧整体胜出。
// preferLocal=true → 最终清单 = 本机实装(丢弃 remote-only 项);
// preferLocal=false → 最终清单 = 仓库清单(丢弃 local-only 项)。
func decisionFinalBulk(remote Manifest, local map[string][]Installed, preferLocal bool) Manifest {
	profiles := map[string]struct{}{}
	for p := range remote.Profiles {
		profiles[p] = struct{}{}
	}
	for p := range local {
		profiles[p] = struct{}{}
	}
	out := Manifest{Profiles: map[string][]PluginRef{}}
	for profile := range profiles {
		refs := map[string]PluginRef{}
		if preferLocal {
			for _, ins := range local[profile] {
				refs[ins.Name] = PluginRef{Name: ins.Name, Version: ins.Version}
			}
		} else {
			for _, r := range remote.Profiles[profile] {
				refs[r.Name] = PluginRef{Name: r.Name, Version: r.Version}
			}
		}
		out.Profiles[profile] = refsToSlice(refs)
	}
	return out
}

// decideInteractive 全局选择 + 逐项确认。
// 逐项覆盖所有差异项(含新增/移除),回车默认 KeepRemote;3=跳过(与 2 同效,沿用仓库清单)。
func decideInteractive(d *Decider, out io.Writer, remote Manifest, local map[string][]Installed) (Manifest, error) {
	g, err := d.Global()
	if err != nil {
		return Manifest{}, err
	}
	if g == GlobalLocal {
		return decisionFinalBulk(remote, local, true), nil
	}
	if g == GlobalRemote {
		return decisionFinalBulk(remote, local, false), nil
	}
	final := remote.Normalize()
	for _, it := range Compare(remote, local) {
		if it.Type == DiffSame {
			continue
		}
		dec, err := d.PerItem(it, KeepRemote)
		if err != nil {
			return Manifest{}, err
		}
		applyDecision(final, local, it, dec)
	}
	return final, nil
}

func refsToSlice(refs map[string]PluginRef) []PluginRef {
	out := []PluginRef{}
	for _, r := range refs {
		out = append(out, r)
	}
	return out
}

// localVersion 返回本机实装版本;本机没有该插件时返回空字符串。
func localVersion(local map[string][]Installed, it DiffItem) string {
	for _, ins := range local[it.Profile] {
		if ins.Name == it.Name {
			return ins.Version
		}
	}
	return ""
}

// setEntry 把 (name, version) 写入 final 的 profile;version 为空表示移除该项。
func setEntry(m Manifest, profile, name, version string) {
	refs := m.Profiles[profile]
	out := refs[:0]
	for _, r := range refs {
		if r.Name != name {
			out = append(out, r)
		}
	}
	if version != "" {
		out = append(out, PluginRef{Name: name, Version: version})
	}
	m.Profiles[profile] = out
}

// applyDecision 语义:
//   - KeepLocal:最终清单采用本机版本;本机没有(新增项拒绝采纳)→ 从最终清单移除
//   - KeepRemote/Skip:沿用仓库清单(最终清单保持仓库现状;对"移除"项即为不恢复)
func applyDecision(m Manifest, local map[string][]Installed, it DiffItem, dec Decision) {
	if m.Profiles[it.Profile] == nil {
		m.Profiles[it.Profile] = []PluginRef{}
	}
	switch dec {
	case KeepLocal:
		setEntry(m, it.Profile, it.Name, localVersion(local, it))
	case KeepRemote, Skip:
		// 仓库现状即 final 当前内容:新增/升级项已存在,移除项不存在,无需操作
	}
}

// syncCmd 命令定义。
func syncCmd() *cobra.Command {
	var yes bool
	var prefer string
	cmd := &cobra.Command{
		Use:   "sync [status]",
		Short: "同步插件清单(DSH_SYNC_REPO 中的 plugins.yaml)",
		Long:  "对比本机插件实装与 DSH_SYNC_REPO 中的 plugins.yaml,用户逐项决策后写回 git 仓库;本机有变更时调整插件并重启 web。",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("sync 仅接受 0 或 1 个参数:status")
			}
			if len(args) == 1 && args[0] != "status" {
				return fmt.Errorf("未知子命令:%s(仅支持 status)", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if prefer != "local" && prefer != "remote" {
				return errors.New("--prefer 仅支持 local 或 remote")
			}
			repo, err := repoRoot()
			if err != nil {
				return err
			}
			mode := fullSync
			if len(args) > 0 && args[0] == "status" {
				mode = statusOnly
			}
			if yes {
				mode |= flagYes
			}
			if prefer == "local" {
				mode |= flagPreferLocal
			}
			return runSync(mode, repo, os.Getenv("DSH_SYNC_REPO"), "", os.Stdout, defaultRunner, nil, nil)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "跳过全部交互,差异项跟随全局倾向(默认用仓库版本)")
	cmd.Flags().StringVar(&prefer, "prefer", "remote", "配合 --yes:local 或 remote")
	return cmd
}
