package paths

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDBOverride(t *testing.T) {
	path, err := DB("./test.db")
	if err != nil || path != "./test.db" {
		t.Fatalf("path=%q, err=%v", path, err)
	}
}

func TestDBLinuxDataDirectory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/XDG")
	}
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	for _, tc := range []struct {
		name, xdg, base string
	}{
		{"explicit XDG", xdg, xdg},
		{"default", "", filepath.Join(home, ".local", "share")},
		{"ignore relative XDG", "relative", filepath.Join(home, ".local", "share")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", tc.xdg)
			path, err := DB("")
			want := filepath.Join(tc.base, "gradia", "gradia.db")
			if err != nil || path != want {
				t.Fatalf("path=%q, want=%q, err=%v", path, want, err)
			}
		})
	}
}
