package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check that the cluster is ready for Kuberik or has Kuberik installed",
	Long: `Verify that the cluster has the required CRDs and controllers running.

Reports the install status of:
  - rollout-controller (required)
  - Kuberik CRDs (Rollout, RolloutGate, HealthCheck, RolloutSchedule)

Also reports, without failing, whether the CRDs for ReleaseSource
(release-controller, where Rollouts get their releases from) and
RolloutApproval (needed by "kuberik approve") are present, and whether Flux
image-reflector is installed (optional: only for a ReleaseSource that bridges
an existing ImagePolicy with spec.oci.imagePolicyRef).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		fmt.Println("Checking Kuberik CRDs...")
		if err := kubectl("get", "crd",
			"rollouts.kuberik.com",
			"rolloutgates.kuberik.com",
			"healthchecks.kuberik.com",
			"rolloutschedules.kuberik.com",
		).Run(); err != nil {
			return fmt.Errorf("CRDs not installed: %w", err)
		}
		fmt.Println("\nChecking rollout-controller pod...")
		if err := kubectl("get", "pods", "-n", "kuberik-system", "-l", "app.kubernetes.io/name=rollout-controller").Run(); err != nil {
			return err
		}
		fmt.Println("\nOptional components:")
		for _, c := range []struct{ crd, what, missing string }{
			{"releasesources.kuberik.com", "ReleaseSource CRD (release-controller)", "Rollouts get no releases without it; install release-controller"},
			{"rolloutapprovals.kuberik.com", "RolloutApproval CRD", "'kuberik approve' needs it; upgrade rollout-controller"},
			{"imagepolicies.image.toolkit.fluxcd.io", "Flux ImagePolicy CRD (image-reflector)", "only needed to bridge an existing ImagePolicy via spec.oci.imagePolicyRef"},
		} {
			status := "present"
			if !crdPresent(c.crd) {
				status = "not installed (" + c.missing + ")"
			}
			fmt.Printf("  %s: %s\n", c.what, status)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

// crdPresent reports whether the named CRD exists. Any lookup failure counts as absent.
func crdPresent(name string) bool {
	out, err := kubectlRun("", "get", "crd", name, "--ignore-not-found", "-o", "name")
	return err == nil && strings.TrimSpace(out) != ""
}
