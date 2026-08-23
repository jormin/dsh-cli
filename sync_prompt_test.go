package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestChooseGlobalBulk(t *testing.T) {
	var out bytes.Buffer
	d := NewDecider(strings.NewReader("2\n"), &out)
	g, err := d.Global()
	if err != nil || g != GlobalRemote {
		t.Fatalf("global=%v err=%v", g, err)
	}
}

func TestChooseGlobalDefaultIsAsk(t *testing.T) {
	var out bytes.Buffer
	d := NewDecider(strings.NewReader("\n"), &out)
	g, err := d.Global()
	if err != nil || g != GlobalAsk {
		t.Fatalf("global=%v err=%v,want GlobalAsk", g, err)
	}
}

func TestDecidePerItemDefaultFollowsGlobal(t *testing.T) {
	var out bytes.Buffer
	d := NewDecider(strings.NewReader("3\n\n"), &out)
	g, err := d.Global()
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultFor(g)
	if def != KeepRemote {
		t.Fatalf("default=%v", def)
	}
	got, err := d.PerItem(DiffItem{Profile: "web", Name: "a/pkg", Type: DiffUpgrade}, def)
	if err != nil || got != KeepRemote {
		t.Fatalf("perItem=%v err=%v", got, err)
	}
}

func TestDecidePerItemChoices(t *testing.T) {
	var out bytes.Buffer
	d := NewDecider(strings.NewReader("1\n2\n3\n"), &out)
	def := KeepRemote
	for _, want := range []Decision{KeepLocal, KeepRemote, Skip} {
		got, err := d.PerItem(DiffItem{Name: "x"}, def)
		if err != nil || got != want {
			t.Fatalf("got=%v want=%v err=%v", got, want, err)
		}
	}
}
