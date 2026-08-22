package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// repoRoot 只从 DSH_REPO 环境变量定位 dsh 源码仓库,未设置或无效直接报错.
func repoRoot() (string, error) {
	p := strings.TrimSpace(os.Getenv("DSH_REPO"))
	if p == "" {
		return "", errors.New("未检测到 DSH_REPO 环境变量,请先设置:\n  export DSH_REPO=/path/to/deepseek-harness")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	pk := filepath.Join(abs, "package.json")
	data, err := os.ReadFile(pk)
	if err != nil {
		return "", fmt.Errorf("DSH_REPO 不是有效的 dsh 仓库(找不到 package.json): %s", abs)
	}
	if !strings.Contains(string(data), "\"@deepseek-ai/dsh-root\"") {
		return "", fmt.Errorf("DSH_REPO 不是有效的 dsh 仓库(package.json 缺少 @deepseek-ai/dsh-root): %s", abs)
	}
	return abs, nil
}
