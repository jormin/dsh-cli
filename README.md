# dsh — DeepSeek Harness 统一命令行工具

将原有分散的 shell 脚本(`dsh` / `dshweb` / `dshplugin` / `dshupdate` / `dsh-restart`)整合为一个 Go 可执行文件,统一管理源码仓库的命令入口、Web 服务生命周期、仓库更新与自身升级。

> 状态:当前为设计与规划文档,命令行为以本文件为准。

## 构建与安装

- 环境:Go 1.17+
- 构建:`go build -o dsh .`
- 安装:将产物放入 PATH,如 `/usr/local/bin/dsh`

## 环境变量

| 变量 | 必需 | 说明 |
|---|---|---|
| `DSH_REPO` | 是 | DeepSeek Harness 源码仓库的绝对路径。未设置或路径无效时命令直接报错退出 |
| `DSH_SYNC_REPO` | 否(仅 sync 需要) | 插件清单 git 仓库目录(含 plugins.yaml),仅 `dsh sync` 使用 |

## 命令总览

### `dsh web [start|stop|restart]`

管理 Web 服务(默认端口 3080)。

- **start(默认)**:后台启动 `pnpm dsh web --no-open`,等待服务监听就绪后输出 PID/URL/日志位置
- **stop**:终止正在运行的服务(读取 PID 记录,兜底查询 3080 占用进程)
- **restart**:先 stop 再 start
- 透传源码仓库 web 的 options(`--patch` 等)与 web app 参数

### `dsh update [--check]`

将源码仓库更新到最新 release tag 并重新构建。

- 按 tag 创建时间检测最新 tag
- 当前不在最新 tag 时询问确认,确认后:`git checkout` → `pnpm install` → `pnpm run build`
- 更新完成后询问是否重启服务
- `--check`:只检测,不更新;已是最新退出码 0,存在新版本退出码 1

### `dsh build`

清理并重新构建源码仓库(依次执行 `pnpm run clean`、`pnpm run build`)。仅本地变更,不 commit/push。

### `dsh switch <tag> [--yes]`

将源码仓库切换到指定的版本 tag 并重新构建(仅本地变更,不 commit/push)。

- 例:`dsh switch dsh-v0.1.2-alpha.1`、`dsh switch v0.1.1-rc.2`(不带 `dsh-` 前缀也支持)
- 切换前检查工作区干净并询问确认(`--yes` 跳过);切换后询问是否重启服务

### `dsh plugin [--profile <name>] <pnpm args...>`

管理指定 profile 的插件(默认 profile:web)。

- 等价于源码仓库的 `dsh plugin --profile <name> <pnpm args...>`
- 例:`dsh plugin add @deepseek-ai/dsh-xxx`、`dsh plugin remove <pkg>`
- add/remove/update 成功后询问是否启动/重启 web 服务(措辞随当前运行状态)

### `dsh sync [status]`

跨机同步插件清单(仅插件,不含配置与会话数据)。

- 环境变量 `DSH_SYNC_REPO` 指向已 clone 的私有 git 仓库,仓库内维护 `plugins.yaml`(按 profile 分组记录插件名与版本)
- **status**:只读,输出本机实装与仓库清单的对比表格,不做任何变更
- **默认流程**:git pull → 对比表格 → 全局选择(1=全部本机 / 2=全部仓库 / 3=逐项)→ 逐项选择(1=本机 / 2=仓库 / 3=跳过)→ 写回 plugins.yaml 并 commit/push(仅 DSH_SYNC_REPO)→ 有变更则 `dsh plugin add/remove` 调整本机插件,并询问是否启动/重启 web(措辞随当前运行状态,`--yes` 非交互自动重启)
- `--yes`:跳过交互,差异项跟随 `--prefer local|remote`(默认 remote)
- 仓库尚无 `plugins.yaml` 时视为空清单:`status`/`sync` 都会先提示;交互模式会先询问是否用本机清单初始化仓库(`--yes` 跳过询问)
- 同步仓库远端为空(首次 clone)时自动跳过 `git pull`,可直接首次初始化;首次推送自动 `--set-upstream`,无需手工配置
- 拉取/扫描阶段有进度提示,不会干等;两侧无差异时直接结束,不做任何选择与调整
- 凭据(`.credentials.yaml`)永不入库;插件使用 `--save-exact` 精确锁定版本

### `dsh upgrade`

检测并更新 dsh 自身二进制。

- 查询 GitHub Release 最新版本,与当前版本比较
- 有新版本时询问确认,确认后下载当前平台产物并替换自身
- 当前已是最新时直接提示

### 全局

- `dsh --help` / `dsh help [command]`:帮助
- `dsh --version`:版本号

## 文件与日志

| 路径 | 说明 |
|---|---|
| `~/.dsh/web.log` | Web 服务运行日志 |
| `~/.dsh/web.pid` | Web 服务进程 PID |

## 发布流程(GitHub Actions)

参照 gacode 的发布模式(自动递增版本 → 交叉编译 → 创建 Release):

1. push / PR 到 master:运行测试
2. push 到 master:自动计算下一个版本号(`vX.Y.Z` patch+1)
3. 交叉编译多平台产物:`dsh_<os>_<arch>_<version>`
4. 打 tag 并推送,创建 GitHub Release,附带全部产物

产物供 `dsh upgrade` 按当前平台下载更新。

## 规划中(未实现)

- `dsh run --profile <name>`:任意 profile 的通用启动入口(web/headless/tui 等)
- shell 脚本(`dshupdate` 等)的正式退役
