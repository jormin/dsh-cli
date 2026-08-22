package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

var verRe = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)`)

func parseVersion(v string) ([]int, error) {
	m := verRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return nil, fmt.Errorf("无法解析版本号: %s", v)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return []int{major, minor, patch}, nil
}

func compareVersions(a, b []int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func upgradeRepoSlug() string {
	slug := strings.TrimSpace(os.Getenv("DSH_CLI_REPO"))
	if slug == "" {
		slug = upgradeRepo
	}
	// 容忍 "github.com/owner/repo" / "https://github.com/owner/repo" 形式的默认值或环境变量
	slug = strings.TrimSuffix(slug, "/")
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "github.com/"} {
		slug = strings.TrimPrefix(slug, prefix)
	}
	return slug
}

func latestReleaseVersion(repo string) (string, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("查询 GitHub Release 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", errors.New("该项目暂无 GitHub Release")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return "", fmt.Errorf("查询 GitHub Release 失败(HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return strings.TrimSpace(rel.TagName), nil
}

func upgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "检测并更新 dsh 自身二进制",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade()
		},
	}
}

func runUpgrade() error {
	repo := upgradeRepoSlug()
	current := strings.TrimPrefix(version, "v")
	fmt.Println("当前版本: v" + current)
	fmt.Println("检查仓库:", repo)

	latest, err := latestReleaseVersion(repo)
	if err != nil {
		return err
	}
	latest = strings.TrimPrefix(latest, "v")

	cv, err := parseVersion(current)
	if err != nil {
		return err
	}
	lv, err := parseVersion(latest)
	if err != nil {
		return err
	}

	if compareVersions(lv, cv) <= 0 {
		fmt.Println("✓ 已是最新版本: v" + current)
		return nil
	}

	fmt.Println("发现新版本: v" + latest + " (当前 v" + current + ")")
	if !askYN("是否下载并更新?") {
		fmt.Println("已取消。")
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位当前可执行文件: %w", err)
	}

	// 产物命名与 build.sh 保持一致: dsh_<os>_<arch>_<version>
	plat := runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		plat += ".exe"
	}
	url := fmt.Sprintf("https://github.com/%s/releases/latest/download/dsh_%s_v%s", repo, plat, latest)
	fmt.Println("==> 下载:", url)

	tmp, err := os.CreateTemp("", "dsh-upgrade-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败(HTTP %d): %s", resp.StatusCode, url)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("下载中断: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	backup := exe + ".old"
	_ = os.Rename(exe, backup)
	if err := os.Rename(tmp.Name(), exe); err != nil {
		_ = os.Rename(backup, exe)
		return fmt.Errorf("替换可执行文件失败: %w", err)
	}
	_ = os.Remove(backup)
	fmt.Println("✓ 已更新到 v" + latest + ",请重新运行 dsh(当前进程退出后新版本生效)。")
	return nil
}
