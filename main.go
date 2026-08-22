package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version 通过 -ldflags "-X main.version=vX.Y.Z" 注入;upgradeRepo 同理.
var (
	version     = "0.0.1"
	upgradeRepo = "github.com/jormin/dsh-cli"
)

func main() {
	root := &cobra.Command{
		Use:     "dsh",
		Short:   "DeepSeek Harness 统一命令行工具",
		Long:    "统一管理 DeepSeek Harness 源码仓库的 Web 服务、更新与插件(Go 版)。",
		Version: version,
	}
	root.SetVersionTemplate("dsh {{.Version}}\n")
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.AddCommand(webCmd(), updateCmd(), pluginCmd(), upgradeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误: "+err.Error())
		os.Exit(1)
	}
}
