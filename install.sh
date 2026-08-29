#!/bin/sh
#
# dsh — DeepSeek Harness 统一命令行工具 一键安装脚本
# (参照 sysmon 项目的安装脚本模式)
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/jormin/dsh-cli/master/install.sh | sh
#   或下载后本地执行: sh install.sh [选项]
#
# 自动识别操作系统(macOS/Linux)与 CPU 架构(amd64/arm64),
# 从 GitHub Releases 下载对应二进制安装到 /usr/local/bin
# (安装目录不可写时自动通过 sudo 提示授权)。
#
# 可选环境变量:
#   DSH_VERSION      指定版本, 如 v0.0.1(默认自动获取最新 release)
#   DSH_OS           手动指定系统: darwin|linux
#   DSH_ARCH         手动指定架构: amd64|arm64
#   DSH_INSTALL_DIR  安装目录(默认 /usr/local/bin)
#   DSH_CLI_REPO     发布仓库, 如 jormin/dsh-cli(默认 jormin/dsh-cli;与 dsh upgrade 的约定一致)
#   DSH_BASE_URL     下载基础地址(默认 https://github.com/<repo>/releases/download, 一般无需设置)
#   DSH_CHECKSUM     可选: 期望的 SHA-256 校验和(64 位 hex)

set -eu

REPO="${DSH_CLI_REPO:-jormin/dsh-cli}"
INSTALL_DIR="${DSH_INSTALL_DIR:-/usr/local/bin}"
BASE_URL="${DSH_BASE_URL:-https://github.com/${REPO}/releases/download}"

info() { printf '%s\n' "$*"; }
warn() { printf '警告: %s\n' "$*" >&2; }
die()  { printf '错误: %s\n' "$*" >&2; exit 1; }

have_cmd() { command -v "$1" >/dev/null 2>&1; }

usage() {
	cat <<'EOF'
dsh 一键安装脚本(DeepSeek Harness 统一命令行工具)

用法:
  curl -fsSL https://raw.githubusercontent.com/jormin/dsh-cli/master/install.sh | sh
  sh install.sh [选项]

选项:
  -h, --help       显示本帮助
  -u, --uninstall  卸载 dsh(目录不可写时自动使用 sudo)

环境变量:
  DSH_VERSION      指定版本, 如 v0.0.1(默认最新 release)
  DSH_INSTALL_DIR  安装目录(默认 /usr/local/bin)
  DSH_OS           手动指定系统: darwin|linux
  DSH_ARCH         手动指定架构: amd64|arm64
  DSH_CLI_REPO     发布仓库(默认 jormin/dsh-cli)
  DSH_CHECKSUM     可选 SHA-256 校验和(64 位 hex)
EOF
}

# 检测操作系统: macOS -> darwin, Linux -> linux, 其余不支持
detect_os() {
	if [ -n "${DSH_OS:-}" ]; then
		os="$DSH_OS"
	else
		os=$(uname -s 2>/dev/null || echo unknown)
		case "$os" in
			Darwin) os=darwin ;;
			Linux)  os=linux ;;
			*) die "不支持的操作系统: $os(仅支持 macOS/Linux)" ;;
		esac
	fi
	case "$os" in
		darwin|linux) ;;
		*) die "DSH_OS 仅支持 darwin|linux, 当前值: $os" ;;
	esac
	printf '%s\n' "$os"
}

# 检测 CPU 架构: x86_64 -> amd64, arm64/aarch64 -> arm64
detect_arch() {
	if [ -n "${DSH_ARCH:-}" ]; then
		arch="$DSH_ARCH"
	else
		arch=$(uname -m 2>/dev/null || echo unknown)
		case "$arch" in
			x86_64|amd64|x64)    arch=amd64 ;;
			arm64|aarch64|armv8l) arch=arm64 ;;
			*) die "不支持的 CPU 架构: $arch(仅支持 amd64/arm64)" ;;
		esac
		# Apple Silicon 在 Rosetta 下 uname -m 返回 x86_64, 用 sysctl 纠正为 arm64
		if [ "$arch" = amd64 ] && [ "$(uname -s)" = Darwin ] && have_cmd sysctl \
			&& [ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = 1 ]; then
			arch=arm64
		fi
	fi
	case "$arch" in
		amd64|arm64) ;;
		*) die "DSH_ARCH 仅支持 amd64|arm64, 当前值: $arch" ;;
	esac
	printf '%s\n' "$arch"
}

# 获取最新 release 的版本号, 返回形如 v0.0.1
latest_release_version() {
	# 优先用 GitHub API 获取最新 release 的 tag_name
	if have_cmd curl; then
		resp=$(curl -fsSL --max-time 20 "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)
		v=$(printf '%s\n' "$resp" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
		[ -n "$v" ] && { printf '%s\n' "$v"; return; }
	fi
	# 回退: 利用 /releases/latest 的 302 跳转地址解析版本(不消耗 API 配额)
	if have_cmd curl; then
		loc=$(curl -s -o /dev/null --max-time 20 -w '%{redirect_url}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)
		v=$(printf '%s\n' "$loc" | sed -n 's#.*/tag/\(v[^/]*\)$#\1#p')
		[ -n "$v" ] && { printf '%s\n' "$v"; return; }
	fi
	die "无法获取最新版本号, 请稍后重试或设置 DSH_VERSION=vX.Y.Z 指定版本"
}

# 解析最终安装版本, 统一补 v 前缀
resolve_version() {
	if [ -n "${DSH_VERSION:-}" ]; then
		version="$DSH_VERSION"
	else
		version=$(latest_release_version)
	fi
	case "$version" in
		v*) ;;
		*) version="v${version}" ;;
	esac
	printf '%s\n' "$version"
}

# 下载文件: 优先 curl, 其次 wget
download() {
	url=$1
	dest=$2
	if have_cmd curl; then
		curl -fsSL --max-time 120 -o "$dest" "$url"
	elif have_cmd wget; then
		wget -q -O "$dest" "$url"
	else
		die "未找到 curl 或 wget, 无法下载"
	fi
}

# 校验下载内容确实是 ELF(Linux)或 Mach-O(macOS)可执行文件,
# 避免 HTML 错误页/代理干扰被当成二进制安装
check_binary() {
	f=$1
	[ -s "$f" ] || return 1
	if have_cmd file; then
		kind=$(file -b "$f")
		case "$kind" in
			*ELF*|*Mach-O*) return 0 ;;
			*) return 1 ;;
		esac
	fi
	magic=$(head -c 4 "$f" 2>/dev/null | od -An -tx1 2>/dev/null | tr -d ' \n') || return 1
	case "$magic" in
		7f454c46|cffaedfe|cefaedfe) return 0 ;; # ELF | Mach-O 64(小端/大端)
		*) return 1 ;;
	esac
}

# 可选: 校验 SHA-256
verify_checksum() {
	[ -n "${DSH_CHECKSUM:-}" ] || return 0
	if have_cmd sha256sum; then
		actual=$(sha256sum "$1" | awk '{print $1}')
	elif have_cmd shasum; then
		actual=$(shasum -a 256 "$1" | awk '{print $1}')
	else
		warn "未找到 sha256sum/shasum, 跳过校验和验证"
		return 0
	fi
	[ "$actual" = "$DSH_CHECKSUM" ] || die "校验和不匹配: 期望 $DSH_CHECKSUM, 实际 $actual"
	info "==> 校验和验证通过 ($actual)"
}

# 安装到目标目录, 不可写时自动用 sudo
install_to() {
	src=$1
	dest_dir=$2
	if [ ! -d "$dest_dir" ]; then
		if ! mkdir -p "$dest_dir" 2>/dev/null; then
			have_cmd sudo || die "无法创建安装目录 $dest_dir(且无 sudo)"
			sudo mkdir -p "$dest_dir" || die "无法创建安装目录 $dest_dir"
		fi
	fi
	if [ -w "$dest_dir" ]; then
		if have_cmd install; then
			install -m 0755 "$src" "${dest_dir}/dsh"
		else
			cp "$src" "${dest_dir}/dsh"
			chmod 0755 "${dest_dir}/dsh"
		fi
	elif have_cmd sudo; then
		info "安装目录 $dest_dir 不可写, 使用 sudo 安装..."
		sudo install -m 0755 "$src" "${dest_dir}/dsh"
	else
		die "安装目录 $dest_dir 不可写且没有 sudo; 可用 DSH_INSTALL_DIR 指定可写目录"
	fi
}

# 安装目录不在 PATH 中时提醒
check_path() {
	dest_dir=$1
	case ":${PATH}:" in
		*":${dest_dir}:"*) return 0 ;;
	esac
	warn "$dest_dir 不在 PATH 中, 使用时请写全路径 ${dest_dir}/dsh 或将其加入 PATH"
}

# 卸载(同时清理历史升级残留的 dsh.old 备份)
uninstall() {
	target="${INSTALL_DIR}/dsh"
	old_backup="${INSTALL_DIR}/dsh.old"
	if [ ! -e "$target" ] && [ ! -L "$target" ] && [ ! -e "$old_backup" ] && [ ! -L "$old_backup" ]; then
		info "未找到 $target, 无需卸载"
		return 0
	fi
	if [ -w "$INSTALL_DIR" ]; then
		rm -f "$target" "$old_backup"
	elif have_cmd sudo; then
		sudo rm -f "$target" "$old_backup"
	else
		die "无法删除 $target: 目录不可写且没有 sudo"
	fi
	info "==> 已卸载 $target"
}

main() {
	case "${1:-}" in
		'') ;;
		-h|--help) usage; exit 0 ;;
		-u|--uninstall) uninstall; exit 0 ;;
		*) usage; exit 2 ;;
	esac

	os=$(detect_os)
	arch=$(detect_arch)
	version=$(resolve_version)
	case "$os" in
		darwin) os_name=macOS ;;
		linux)  os_name=Linux ;;
	esac

	asset="dsh_${os}_${arch}_${version}"
	url="${BASE_URL}/${version}/${asset}"

	info "==> 检测到 ${os_name} / ${arch}, 安装 dsh ${version}"
	info "==> 下载: ${url}"

	tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/dsh-install.XXXXXX") || die "创建临时目录失败"
	trap 'rm -rf "${tmpdir}"' EXIT HUP INT TERM
	tmpfile="${tmpdir}/dsh"

	download "$url" "$tmpfile" || die "下载失败: ${url}(可用 DSH_VERSION 指定其他版本)"
	check_binary "$tmpfile" || die "下载内容不是有效的可执行文件, 可能该版本缺少 ${os}/${arch} 产物或网络被代理干扰: ${url}"
	verify_checksum "$tmpfile"

	install_to "$tmpfile" "$INSTALL_DIR"
	check_path "$INSTALL_DIR"

	installed=$("${INSTALL_DIR}/dsh" --version 2>/dev/null || true)
	if [ -n "$installed" ]; then
		info "==> 安装完成: ${INSTALL_DIR}/dsh (${installed})"
		expect=$(printf '%s\n' "$version" | sed 's/^v//')
		if [ "$installed" != "$expect" ]; then
			warn "安装的版本(${installed})与期望(${expect})不一致"
		fi
	else
		warn "安装完成, 但版本自检失败, 请手动运行 ${INSTALL_DIR}/dsh --version 检查"
	fi
	info "==> 使用: 设置 DSH_REPO 指向 DeepSeek Harness 源码仓库后运行 dsh --help"
}

main "$@"