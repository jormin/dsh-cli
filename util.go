package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const webPort = 3080

// dshBaseDir 返回 ~/.dsh 并确保存在.
func dshBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法确定用户主目录: %w", err)
	}
	dir := filepath.Join(home, ".dsh")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建 %s 失败: %w", dir, err)
	}
	return dir, nil
}

func webLogPath() string {
	if d, err := dshBaseDir(); err == nil {
		return filepath.Join(d, "web.log")
	}
	return "web.log"
}

// portPids 通过 lsof 返回监听 webPort 的 PID 列表.
func portPids() []string {
	out, err := exec.Command("lsof", "-t", "-nP", "-iTCP:"+strconv.Itoa(webPort), "-sTCP:LISTEN").Output()
	if err != nil {
		return nil
	}
	var pids []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			pids = append(pids, line)
		}
	}
	return pids
}

func portBusy() bool { return len(portPids()) > 0 }

// waitPort 轮询直到端口可连接或超时.
func waitPort(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(webPort), time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// logContainsMarker 报告日志中是否已出现给定标记,用于辅助确认服务实际就绪.
func logContainsMarker(path, marker string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), marker)
}

// tailLog 返回日志末尾 maxLines 行;读取失败时返回提示文本.
func tailLog(path string, maxLines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "(无法读取日志: " + err.Error() + ")"
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

func killPids(pids []string) {
	for _, p := range pids {
		pid, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || pid <= 0 {
			continue
		}
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
}

// askYN 交互询问,默认否.
func askYN(prompt string) bool {
	fmt.Print(prompt + " [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}

// runCmd 在 dir 中执行命令,标准输入/输出直连终端.
func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// startService 在后台启动 pnpm dsh web --no-open,记录 PID/日志并等待端口就绪.
func startService(repo string, launcherFlags, appArgs []string) error {
	// 端口占用:询问是否终止占用进程
	if portBusy() {
		fmt.Println("注意: 端口", webPort, "已被占用,占用进程如下:")
		if pids := portPids(); len(pids) > 0 {
			fmt.Println("    PID   :", strings.Join(pids, ", "))
		}
		fmt.Println("(终止该进程可能会中断正在运行的 dsh 会话/任务)")
		if !askYN("是否终止占用进程并继续?") {
			return errors.New("已取消,请自行停止占用 " + strconv.Itoa(webPort) + " 端口的进程后重试")
		}
		killPids(portPids())
		time.Sleep(time.Second)
		if portBusy() {
			return errors.New("进程未能终止,端口 " + strconv.Itoa(webPort) + " 仍被占用")
		}
		fmt.Println("    ✓ 端口", webPort, "已释放")
	}

	dir, err := dshBaseDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(dir, "web.log")
	logf, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志 %s 失败: %w", logPath, err)
	}
	defer logf.Close()

	args := []string{"-C", repo, "dsh", "web"}
	args = append(args, launcherFlags...)
	args = append(args, "--no-open")
	args = append(args, appArgs...)

	cmd := exec.Command("pnpm", args...)
	cmd.Dir = repo
	cmd.Stdout = logf
	cmd.Stderr = logf
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 pnpm dsh web 失败: %w", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "web.pid"), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)

	fmt.Println("==> 后台启动服务: pnpm dsh web --no-open")
	fmt.Println("    PID   :", cmd.Process.Pid)
	fmt.Println("    URL   : http://127.0.0.1:" + strconv.Itoa(webPort))
	fmt.Println("    日志  :", logPath)
	fmt.Println("    停止  : dsh web stop (或 kill " + strconv.Itoa(cmd.Process.Pid) + ")")

	// 就绪等待:冷启动需先完成 pnpm 依赖校验与 tsx 启动,放宽到 60s;
	// 日志出现服务就绪标记(dsh web:)也视为已就绪,避免启动稍慢时误报.
	const readyTimeout = 60 * time.Second
	const readyMarker = "dsh web:"
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	if waitPort(readyTimeout) {
		fmt.Println("    ✓ 服务已启动: http://127.0.0.1:" + strconv.Itoa(webPort))
		return nil
	}
	if logContainsMarker(logPath, readyMarker) {
		fmt.Println("    ✓ 服务进程已启动(日志已确认): http://127.0.0.1:" + strconv.Itoa(webPort))
		return nil
	}
	select {
	case err := <-done:
		return fmt.Errorf("后台进程已退出(%v),请查看日志尾部:\n%s", err, tailLog(logPath, 20))
	default:
	}
	return fmt.Errorf("未能在 %v 内确认端口 %d 监听,请查看日志尾部:\n%s", readyTimeout, webPort, tailLog(logPath, 20))
}

// stopService 终止占用端口的进程;未运行视为成功.
func stopService() error {
	pids := portPids()
	if len(pids) == 0 {
		fmt.Println("未检测到运行中的服务(端口", webPort, "空闲)")
		return nil
	}
	fmt.Println("==> 终止进程:", strings.Join(pids, ", "))
	killPids(pids)
	time.Sleep(2 * time.Second)
	if portBusy() {
		return errors.New("进程未能终止,端口 " + strconv.Itoa(webPort) + " 仍被占用")
	}
	if dir, err := dshBaseDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, "web.pid"))
	}
	fmt.Println("    ✓ 服务已停止")
	return nil
}
