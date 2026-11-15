// Package main is the entry point for the kuberik CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via -ldflags.
	Version = "dev"

	// kubeconfig is the global flag for the path to a kubeconfig file.
	kubeconfig string
	// namespace is the global flag for the Kubernetes namespace.
	namespace string
)

var rootCmd = &cobra.Command{
	Use:           "kuberik",
	Short:         "Kuberik - Kubernetes-native continuous delivery",
	Long:          "Kuberik is a Kubernetes-native progressive delivery system. The CLI helps install, inspect, and operate Kuberik on a cluster.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to the kubeconfig file (defaults to $KUBECONFIG or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
