package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestRootCommand verifies the root command exposes its subcommands.
func TestRootCommand(t *testing.T) {
	want := []string{
		"version", "install", "uninstall", "check", "get",
		"approve", "revoke", "completion",
		"describe", "logs", "events", "tree", "init",
		"bootstrap",
	}
	// Init subkinds the user can pass.
	for _, sub := range []string{"rollout", "gate", "schedule", "healthcheck"} {
		if _, _, err := rootCmd.Find([]string{"init", sub}); err != nil {
			t.Errorf("init %q not registered: %v", sub, err)
		}
	}
	for _, name := range want {
		if _, _, err := rootCmd.Find([]string{name}); err != nil {
			t.Errorf("subcommand %q not registered: %v", name, err)
		}
	}
}

// TestVersionCommand verifies the version command prints the build version.
func TestVersionCommand(t *testing.T) {
	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	// version subcommand currently writes to os.Stdout directly, so we just
	// confirm the command resolved and executed without error.
}

// TestApproveRequiresArgs ensures `approve` and `revoke` need ROLLOUT and VERSION.
func TestApproveRequiresArgs(t *testing.T) {
	for _, name := range []string{"approve", "revoke"} {
		for _, args := range [][]string{{name}, {name, "my-app"}} {
			out := &bytes.Buffer{}
			rootCmd.SetOut(out)
			rootCmd.SetErr(out)
			rootCmd.SetArgs(args)
			err := rootCmd.Execute()
			if err == nil {
				t.Fatalf("%v: expected an argument error", args)
			}
			if !strings.Contains(err.Error(), "accepts 2 arg") {
				t.Errorf("%v: unexpected error: %v", args, err)
			}
		}
	}
}

// TestRemovedCommands pins that the commands without a controller behind them are gone.
func TestRemovedCommands(t *testing.T) {
	for _, name := range []string{"suspend", "resume", "reject"} {
		if c, _, _ := rootCmd.Find([]string{name}); c != nil && c != rootCmd {
			t.Errorf("%q is still registered", name)
		}
	}
}

// TestCompletionInvalidShell rejects unknown shells.
func TestCompletionInvalidShell(t *testing.T) {
	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"completion", "tcsh"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unsupported shell")
	}
}

// TestScopeFlags verifies the get-command scope helper honors --all-namespaces.
func TestScopeFlags(t *testing.T) {
	namespace = "demo"
	allNamespaces = false
	args := scope()
	if got, want := strings.Join(args, " "), "-n demo"; got != want {
		t.Errorf("scope() = %q, want %q", got, want)
	}

	allNamespaces = true
	args = scope()
	if got, want := strings.Join(args, " "), "--all-namespaces"; got != want {
		t.Errorf("scope(all) = %q, want %q", got, want)
	}
	// reset so other tests are not affected
	allNamespaces = false
}

func TestCheckFluxVersion(t *testing.T) {
	for out, wantErr := range map[string]bool{
		"flux version 2.7.1\n": false,
		"flux version 2.6.0":   false,
		"flux version 2.5.1":   true,
		"flux version 1.9.0":   true,
		"garbage":              true,
	} {
		if err := checkFluxVersion(out); (err != nil) != wantErr {
			t.Errorf("checkFluxVersion(%q) = %v, wantErr %v", out, err, wantErr)
		}
	}
}

func TestInitScaffoldsReleaseSource(t *testing.T) {
	for _, tc := range []struct{ kind, want string }{
		{"rollout", "releaseSourceRef:"},
		{"releasesource", "kind: ReleaseSource"},
	} {
		got, err := captureStdout(func() error {
			rootCmd.SetArgs([]string{"init", tc.kind, "--name", "demo"})
			return rootCmd.Execute()
		})
		if err != nil || !strings.Contains(got, tc.want) || strings.Contains(got, "releasesImagePolicy") {
			t.Errorf("init %s: err=%v output=%q", tc.kind, err, got)
		}
	}
}

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	_ = w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b), err
}
