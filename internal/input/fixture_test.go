package input

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateFixtures = flag.Bool("update-fixtures", false, "rewrite committed fixtures in testdata/")

// fixtureFile writes data to testdata/<name> when -update-fixtures is set and returns
// "testdata/<name>"; without the flag it fails the test if that file does not exist.
func fixtureFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *updateFixtures {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("fixture %s: %v", p, err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatalf("fixture %s: %v", p, err)
		}
		return p
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture %s missing (run go test -update-fixtures): %v", p, err)
	}
	return p
}
