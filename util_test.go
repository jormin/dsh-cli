package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadyWebURLWithToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "web.log")
	if err := os.WriteFile(p, []byte("dsh web: http://127.0.0.1:3080/?token=abc123_xyz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readyWebURL(p); got != "http://127.0.0.1:3080/?token=abc123_xyz" {
		t.Fatalf("got %q", got)
	}
}

func TestReadyWebURLLegacyNoToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "web.log")
	if err := os.WriteFile(p, []byte("xxx\ndsh web: http://127.0.0.1:3080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readyWebURL(p); got != "http://127.0.0.1:3080" {
		t.Fatalf("got %q", got)
	}
}

func TestDisplayURLFallback(t *testing.T) {
	if got := displayURL(""); got != "http://127.0.0.1:3080" {
		t.Fatalf("got %q", got)
	}
}
