# AGENTS.md

`dsh` — DeepSeek Harness 的统一命令行工具(Go 实现)。

## 技术栈

- 语言:Go(本机工具链 go1.17.13,go.mod 声明 go 1.17)
- 框架:github.com/spf13/cobra(标准 Go CLI 框架)
- 构建:`go build ./...`(产物为单一二进制 `dsh`)
- 运行依赖:git、pnpm、Node.js,以及 DeepSeek Harness 源码仓库(通过环境变量 DSH_REPO 定位)

## 架构

单一可执行文件,命令树:

- `dsh web [start|stop|restart]`:Web 服务生命周期管理(后台启动、监听 3080、等待就绪;日志与 PID 落在 ~/.dsh/ 下)
- `dsh update [--check]`:将源码仓库更新到最新 release tag(--check 只检测)
- `dsh plugin [--profile <name>]`:转发源码仓库的 plugin 命令(管理 profile 插件)
- `dsh upgrade`:检测并更新 dsh 自身二进制(GitHub Release 分发)

关键约定:

- 仓库定位只使用环境变量 DSH_REPO;未设置或路径无效时直接报错退出,不做交互输入、不记忆路径
- 所有命令最终通过 `pnpm -C <repo> dsh …` 或 git/pnpm 原生命令在源码仓库根目录执行
- 发布产物命名 `dsh_<os>_<arch>_<version>`,由 GitHub Actions 自动编译
- **所有仓库调整一律只做本地变更(git checkout、install、build 等),禁止任何自动化流程(脚本、CI、Agent)自动执行 git commit 或 git push;提交与推送只能由人工显式执行**

## 分支管理

- 分支命名:一律 `feature/xxx`(如 `feature/web-command`)
- 禁止直接向 master 提交或推送代码
- 所有变更必须通过 Pull Request 合入 master
- master 始终由 PR 合并维护,保持随时可发布的稳定状态
- **创建 PR 或合并 PR 时不得删除本地/远程分支**(禁止 `gh pr merge --delete-branch` 及任何自动删分支操作);分支保留,清理仅由人工显式决定

## 提交流程

1. 从 master 切出 `feature/xxx`
2. 开发并在本地验证(`go build ./...`、`go vet ./...`)
3. 推送到远程,发起 PR 合入 master
4. 合并后由 GitHub Actions 自动完成版本发布
