package codemeta_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsumerAcrossGitHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for the history integration test")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "read")
	commandOutput(t, "", "go", "build", "-o", binary, "./examples/read")
	repo := filepath.Join(dir, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	commandOutput(t, repo, "git", "init", "-b", "main")
	commandOutput(t, repo, "git", "config", "user.name", "Fixture")
	commandOutput(t, repo, "git", "config", "user.email", "fixture@example.org")
	type revision struct {
		commit, path, name, version string
		broken                      bool
	}
	var revisions []revision
	record := func(path, name, version, input string, broken bool) {
		t.Helper()
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		commandOutput(t, repo, "git", "add", "--all")
		commandOutput(t, repo, "git", "commit", "-m", "Update metadata")
		commit := strings.TrimSpace(commandOutput(t, repo, "git", "rev-parse", "HEAD"))
		revisions = append(revisions, revision{commit, path, name, version, broken})
	}
	record("codemeta.json", "first", "2.0", `{"@context":"https://w3id.org/codemeta/2.0","name":"first"}`, false)
	record("codemeta.json", "edited", "3.0", `{"@context":"https://w3id.org/codemeta/3.0","name":"edited"}`, false)
	if err := os.Remove(filepath.Join(repo, "codemeta.json")); err != nil {
		t.Fatal(err)
	}
	record("metadata/codemeta.json", "moved", "3.0", `{"@context":"https://w3id.org/codemeta/3.0","name":"moved"}`, false)
	record("metadata/codemeta.json", "", "", `{"name":NaN}`, true)
	commandOutput(t, repo, "git", "rm", "metadata/codemeta.json")
	commandOutput(t, repo, "git", "commit", "-m", "Delete metadata")
	deleted := strings.TrimSpace(commandOutput(t, repo, "git", "rev-parse", "HEAD"))
	if output := commandOutput(t, repo, "git", "ls-tree", "-r", "--name-only", deleted); strings.TrimSpace(output) != "" {
		t.Fatal(output)
	}
	record("codemeta.json", "returned", "", `{"@context":"https://w3id.org/codemeta/4.0","name":"returned"}`, false)
	failures, successes := 0, 0
	for _, revision := range revisions {
		blob := commandOutput(t, repo, "git", "show", revision.commit+":"+revision.path)
		cmd := exec.Command(binary, "-")
		cmd.Stdin = strings.NewReader(blob)
		output, err := cmd.CombinedOutput()
		if revision.broken {
			failures++
			if err == nil || !bytes.Contains(output, []byte("expected a JSON value")) {
				t.Fatalf("bad blob: %s, %v", output, err)
			}
			continue
		}
		successes++
		if err != nil || !bytes.Contains(output, []byte("Name: "+revision.name+"\nContext: "+revision.version+"\n")) {
			t.Fatalf("blob output: %s, %v", output, err)
		}
		if revision.version == "" && !bytes.Contains(output, []byte("unsupported_version")) {
			t.Fatal(string(output))
		}
	}
	if successes != 4 || failures != 1 {
		t.Fatalf("history results: %d successful, %d malformed", successes, failures)
	}
	output := commandOutput(t, "", binary, filepath.Join(repo, "codemeta.json"))
	if !strings.Contains(output, "Name: returned") {
		t.Fatal(output)
	}
}
func commandOutput(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
	return string(output)
}
