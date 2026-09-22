// Package golden compares test output with a file under testdata; -update rewrites the files. The files are the
// review surface: a change to what the customer reads shows up as a diff a reviewer can judge.
package golden

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files with the current output")

// Check compares got with testdata/<name>; with -update it writes the file instead.
func Check(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // test data path under the package's own testdata directory
	if err != nil {
		t.Fatalf("no golden file %s (run with -update to create it): %v", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("%s differs from the golden file (run with -update if the change is intended)\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}
