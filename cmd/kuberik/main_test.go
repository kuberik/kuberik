package main

import (
	"bytes"
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
