package main

import (
	"github.com/spf13/cobra"
)

const (
	coreInstallURL = "https://github.com/kuberik/rollout-controller/releases/latest/download/install.yaml"
	allInstallURL  = "https://github.com/kuberik/kuberik/config/install"
)

var installAll bool

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Kuberik on the current cluster",
	Long: `Install Kuberik on the cluster the current kubeconfig points at.

By default installs only the core rollout-controller. Pass --all to install
the bundle with all integration controllers (Datadog, Prometheus, OpenKruise,
environment-controller).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		if installAll {
			return kubectl(applyArgs("-k", allInstallURL)...).Run()
		}
		return kubectl(applyArgs("-f", coreInstallURL)...).Run()
	},
}

func init() {
	installCmd.Flags().BoolVar(&installAll, "all", false, "Install all integration controllers in addition to core")
	rootCmd.AddCommand(installCmd)
}
