package main

import (
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe KIND NAME",
	Short: "Describe a Kuberik resource (rollout, gate, healthcheck)",
	Long: `Describe a Kuberik resource by short kind name. Equivalent to
"kubectl describe <kind>.kuberik.com <name>".

To stop a Rollout from deploying, pin it with spec.wantedVersion (set it to
the version to hold), or add a RolloutSchedule with action Deny for the
window ("kuberik init schedule" scaffolds one).`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		kind := args[0]
		switch kind {
		case "rollout", "rollouts", "ro":
			kind = "rollouts.kuberik.com"
		case "gate", "gates":
			kind = "rolloutgates.kuberik.com"
		case "healthcheck", "healthchecks", "hc":
			kind = "healthchecks.kuberik.com"
		case "schedule", "schedules":
			kind = "rolloutschedules.kuberik.com"
		}
		return kubectl("-n", namespace, "describe", kind, args[1]).Run()
	},
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
