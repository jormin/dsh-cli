package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Decision 逐项决策。
type Decision int

const (
	KeepLocal Decision = iota
	KeepRemote
	Skip
)

// GlobalChoice 全局选择。
type GlobalChoice int

const (
	GlobalAsk GlobalChoice = iota
	GlobalLocal
	GlobalRemote
)

// DefaultFor 返回全局选择对应的逐项默认值。
func DefaultFor(g GlobalChoice) Decision {
	if g == GlobalLocal {
		return KeepLocal
	}
	return KeepRemote
}

// Decider 交互决策器;reader/writer 可注入。
type Decider struct {
	in  *bufio.Reader
	out io.Writer
}

// NewDecider 构造决策器。
func NewDecider(in io.Reader, out io.Writer) *Decider {
	return &Decider{in: bufio.NewReader(in), out: out}
}

// Global 询问全局选择:1=全部本机 2=全部仓库 3=逐项(默认 3)。
func (d *Decider) Global() (GlobalChoice, error) {
	fmt.Fprintln(d.out, "全局选择:1=全部用本机  2=全部用仓库  3=逐项确认(默认 3)")
	line, err := d.readLine()
	if err != nil {
		return GlobalAsk, err
	}
	switch strings.TrimSpace(line) {
	case "", "3":
		return GlobalAsk, nil
	case "1":
		return GlobalLocal, nil
	case "2":
		return GlobalRemote, nil
	default:
		return GlobalAsk, fmt.Errorf("无法识别的选择:%q(1=全部用本机 2=全部用仓库 3=逐项)", line)
	}
}

// PerItem 逐项询问;回车返回默认值 def。
func (d *Decider) PerItem(it DiffItem, def Decision) (Decision, error) {
	fmt.Fprintf(d.out, "  %s %s:本机 %s / 仓库 %s — 1=本机 2=仓库 3=跳过 [回车=%s]: ",
		it.Profile, it.Name, it.Local, it.Remote, decisionLabel(def))
	line, err := d.readLine()
	if err != nil {
		return Skip, err
	}
	switch strings.TrimSpace(line) {
	case "":
		return def, nil
	case "1":
		return KeepLocal, nil
	case "2":
		return KeepRemote, nil
	case "3":
		return Skip, nil
	default:
		return Skip, fmt.Errorf("无法识别的选择:%q(1=本机 2=仓库 3=跳过)", line)
	}
}

// YesNo 询问一个 y/N 问题;回车或非 y/Y 开头的输入视为否。
func (d *Decider) YesNo(prompt string) (bool, error) {
	fmt.Fprintf(d.out, "%s [y/N] ", prompt)
	line, err := d.readLine()
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (d *Decider) readLine() (string, error) {
	line, err := d.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

func decisionLabel(d Decision) string {
	if d == KeepLocal {
		return "本机"
	}
	return "仓库"
}
