package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugins.yaml")
	want := Manifest{Profiles: map[string][]PluginRef{
		"web": {{Name: "@linxin666/dsh-liangshen", Version: "0.2.8"}},
	}}
	if err := WriteManifest(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestWriteManifestHeaderAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.yaml")
	if err := WriteManifest(path, Manifest{Profiles: map[string][]PluginRef{"web": {{Name: "a", Version: "1.0.0"}}}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# auto-managed by dsh sync") {
		t.Fatalf("missing header comment:\n%s", data)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v err=%v,want 0644", st.Mode().Perm(), err)
	}
}

func TestManifestNormalizeSortsAndDedupes(t *testing.T) {
	m := Manifest{Profiles: map[string][]PluginRef{
		"web": {
			{Name: "z/pkg", Version: "1.0.0"},
			{Name: "a/pkg", Version: "2.0.0"},
			{Name: "a/pkg", Version: "2.0.0"},
		},
	}}
	got := m.Normalize()
	names := []string{got.Profiles["web"][0].Name, got.Profiles["web"][1].Name}
	if names[0] != "a/pkg" || names[1] != "z/pkg" {
		t.Fatalf("expected sorted unique entries, got %v", names)
	}
}

func TestReadManifestMissingFile(t *testing.T) {
	if _, err := ReadManifest(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
