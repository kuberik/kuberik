package main

import (
	"fmt"
	"os"
	"os/exec"
)

// kubectl returns an exec.Cmd configured with the global kubeconfig/namespace flags.
// Output streams are wired to the parent process so users see kubectl's output directly.
func kubectl(args ...string) *exec.Cmd {
	full := []string{}
	if kubeconfig != "" {
		full = append(full, "--kubeconfig", kubeconfig)
	}
	full = append(full, args...)
	cmd := exec.Command("kubectl", full...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd
}

// requireKubectl returns an error if kubectl is not on PATH.
func requireKubectl() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found on PATH: %w", err)
	}
	return nil
}

// applyArgs builds a server-side `kubectl apply` for an install manifest.
// Client-side apply stores the whole object in an annotation, and the
// openkruise RolloutTest CRD (660 KB) is over the 256 KiB annotation limit,
// so a plain `kubectl apply` of the bundle fails with
// "metadata.annotations: Too long". Conflicts are forced because these are
// our own install manifests: the last apply wins, as an installer expects.
func applyArgs(source, ref string) []string {
	return []string{"apply", "--server-side", "--force-conflicts", source, ref}
}
