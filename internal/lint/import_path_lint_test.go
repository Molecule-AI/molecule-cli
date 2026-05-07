// Issue molecule-ai/internal#71 lint gate.
//
// Walks every *.go file in the module + the go.mod declaration + any
// Dockerfile in the repo, and rejects any reference to the dead
// github.com/Molecule-AI/* identity (or the historical
// Molecule-AI/molecule-monorepo path).
//
// We had a 374+131+30+1-line "github.com/Molecule-AI/" footprint across
// the org pre-migration. The class of bug this gate prevents:
//
//   - copy-pastes from old branches re-introducing the dead path
//   - Dockerfile -ldflags strings drifting back to github.com on a
//     refactor (the path has to match the module declaration to inject
//     buildinfo correctly; if they disagree the binary builds but
//     reports a wrong / stale GitSHA)
//   - new modules added to the repo with the wrong import root because
//     someone copied an old go.mod without thinking
//
// Why not just a CI shell grep: a Go test runs everywhere `go test ./...`
// runs, including local pre-push hooks and contributor IDEs. The gate
// fires immediately, with a per-file message that points at the line —
// CI shell grep failures are silent until the runner picks them up.

package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenSubstrings is the literal-match list. Each string MUST NOT
// appear anywhere under the module root. Entries are checked with
// substring matching, not regex — keep the patterns specific enough
// that a false-positive needs an explicit allowlist entry.
var forbiddenSubstrings = []string{
	"github.com/Molecule-AI/",
	"Molecule-AI/molecule-monorepo",
}

// allowlistedFiles is the per-file escape hatch. Empty by default —
// add an entry only when there is a documented reason a forbidden
// string MUST appear (e.g. a regression-test fixture that asserts
// the lint gate itself rejects the string). Each entry MUST be
// accompanied by a comment explaining why.
var allowlistedFiles = map[string]bool{
	// (intentionally empty — add only with justification)
}

func TestNoLegacyGitHubImportPaths(t *testing.T) {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}

	checkExt := map[string]bool{
		".go":   true,
		".mod":  true,
		".sum":  false, // go.sum is auto-generated, refs flow from go.mod
		".sh":   true,
		".yml":  true,
		".yaml": true,
		".toml": true,
		".md":   true,
	}
	checkBasename := map[string]bool{
		"Dockerfile":        true,
		"Dockerfile.tenant": true,
	}

	violations := 0
	walkErr := filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip vendor + .git + node_modules — not our code.
			base := info.Name()
			if base == "vendor" || base == ".git" || base == "node_modules" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		base := filepath.Base(path)
		if !checkExt[ext] && !checkBasename[base] {
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		if allowlistedFiles[rel] {
			return nil
		}
		// Skip the lint test itself — it legitimately names the forbidden
		// strings as match patterns.
		if strings.HasSuffix(rel, "import_path_lint_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, bad := range forbiddenSubstrings {
			if strings.Contains(text, bad) {
				// Find the line number for a useful error message.
				for lineNo, line := range strings.Split(text, "\n") {
					if strings.Contains(line, bad) {
						t.Errorf("%s:%d — forbidden substring %q (use go.moleculesai.app/<area>/... per molecule-ai/internal#71)", rel, lineNo+1, bad)
						violations++
						break
					}
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
	if violations > 0 {
		t.Logf("Total violations: %d. Add to allowlistedFiles ONLY with a documented justification.", violations)
	}
}

// findModuleRoot walks up from the test's CWD to find go.mod. The Go
// test harness sets CWD to the package directory; the module root may
// be one or more parents up.
func findModuleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
