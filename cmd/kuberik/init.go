package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	initName                 string
	initImagePolicy          string
	initImagePolicyNamespace string
	initReleaseSource        string
	initImage                string
	initBakeTime             string
	initRollout              string
)

var initCmd = &cobra.Command{
	Use:   "init KIND",
	Short: "Scaffold a starter manifest for a Kuberik resource",
	Long: `Print a starter manifest to stdout for one of the Kuberik kinds.

Examples:
  kuberik init releasesource --name my-app --image ghcr.io/my-org/my-app > source.yaml
  kuberik init rollout --name my-app > rollout.yaml
  kuberik init gate --name my-app-approval > gate.yaml
  kuberik init schedule --name business-hours > schedule.yaml

The KIND must be one of: rollout, releasesource, gate, schedule, healthcheck.

A Rollout reads its releases from a ReleaseSource (release-controller), which
scans the registry itself; Flux image-reflector is not needed.`,
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"rollout", "releasesource", "gate", "schedule", "healthcheck"},
	RunE: func(cmd *cobra.Command, args []string) error {
		var tmpl string
		switch args[0] {
		case "rollout":
			source := initReleaseSource
			if source == "" {
				source = initName
			}
			tmpl = fmt.Sprintf(rolloutTemplate, initName, namespace, source, initBakeTime)
		case "releasesource":
			tmpl = fmt.Sprintf(releaseSourceTemplate, initName, namespace, initImage)
		case "gate":
			rollout := initRollout
			if rollout == "" {
				rollout = initName
			}
			tmpl = fmt.Sprintf(gateTemplate, initName, namespace, rollout)
		case "schedule":
			// The label value defaults to the schedule's name; Rollouts opt
			// in by carrying that label.
			tmpl = fmt.Sprintf(scheduleTemplate, initName, namespace, initName)
		case "healthcheck":
			label := initRollout
			if label == "" {
				label = initName
			}
			tmpl = fmt.Sprintf(healthcheckTemplate, initName, namespace, label)
		default:
			return fmt.Errorf("unknown kind: %s", args[0])
		}
		_, err := os.Stdout.WriteString(tmpl)
		return err
	},
}

const rolloutTemplate = `apiVersion: kuberik.com/v1alpha1
kind: Rollout
metadata:
  name: %s
  namespace: %s
spec:
  releaseSourceRef:
    name: %s
  versionHistoryLimit: 10
  bakeTime: %s
`

const releaseSourceTemplate = `apiVersion: kuberik.com/v1alpha1
kind: ReleaseSource
metadata:
  name: %s
  namespace: %s
spec:
  oci:
    image: %s
    # Order tags by semver; change to alphabetical or numerical if your
    # tags are not versions.
    policy:
      semver:
        range: ">=0.0.0"
`

const gateTemplate = `apiVersion: kuberik.com/v1alpha1
kind: RolloutGate
metadata:
  name: %s
  namespace: %s
spec:
  rolloutRef:
    name: %s
  # false blocks every version until you set it to true. For per-version
  # manual approval use "kuberik approve ROLLOUT VERSION" instead.
  passing: false
`

const healthcheckTemplate = `apiVersion: kuberik.com/v1alpha1
kind: HealthCheck
metadata:
  name: %s
  namespace: %s
  labels:
    # The Rollout's spec.healthCheckSelector matches HealthChecks on labels.
    # Set this to whatever your Rollout's selector picks up.
    app: %s
spec:
  # Optional. Names the producer (e.g. datadog, prometheus, custom).
  class: custom
`

const scheduleTemplate = `apiVersion: kuberik.com/v1alpha1
kind: RolloutSchedule
metadata:
  name: %s
  namespace: %s
spec:
  # Rollouts opt in by carrying a matching label. Change the selector
  # below to whatever fits your Rollout labels.
  rolloutSelector:
    matchLabels:
      kuberik.com/schedule: %s
  # Allow: open during the window, closed outside.
  # Deny:  closed during the window, open outside.
  action: Allow
  timezone: "America/New_York"
  rules:
    - daysOfWeek: [Monday, Tuesday, Wednesday, Thursday, Friday]
      timeRange:
        start: "09:00"
        end: "17:00"
`

func init() {
	initCmd.Flags().StringVar(&initName, "name", "my-app", "Name to set in metadata.name")
	initCmd.Flags().StringVar(&initImagePolicy, "image-policy", "", "Deprecated and ignored: releases come from a ReleaseSource")
	initCmd.Flags().StringVar(&initImagePolicyNamespace, "image-policy-namespace", "", "Deprecated and ignored")
	_ = initCmd.Flags().MarkDeprecated("image-policy", "a Rollout reads a ReleaseSource; use --release-source and 'kuberik init releasesource'")
	_ = initCmd.Flags().MarkDeprecated("image-policy-namespace", "no longer used")
	initCmd.Flags().StringVar(&initReleaseSource, "release-source", "", "ReleaseSource name (Rollout only). Defaults to --name.")
	initCmd.Flags().StringVar(&initImage, "image", "ghcr.io/my-org/my-app", "OCI repository to scan for releases (releasesource only)")
	initCmd.Flags().StringVar(&initBakeTime, "bake-time", "10m", "Bake duration (Rollout only)")
	initCmd.Flags().StringVar(&initRollout, "rollout", "", "Rollout name to reference (gate/schedule only). Defaults to --name.")
	rootCmd.AddCommand(initCmd)
}
