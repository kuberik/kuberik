package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	bootstrapAll      bool
	bootstrapFluxOnly bool
	bootstrapVersion  string
)

const fluxImageReflectorURL = "https://github.com/fluxcd/flux2/releases/latest/download/install.yaml"

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Install Flux's image-reflector-controller and Kuberik in one step",
	Long: `Run the full first-time install: Flux image-reflector-controller (the
upstream component Kuberik reads ImagePolicy from) followed by the
Kuberik core controller (or the full bundle with --all).

Equivalent to:
  flux install --components=source-controller,kustomize-controller,image-reflector-controller
  kuberik install [--all]

Skip the Flux install with --flux=false if your cluster already has it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		if !bootstrapFluxOnly {
			_, _ = fmt.Fprintln(cmd.OutOrStderr(), "Installing Flux core (source, kustomize, image-reflector)...")
			if err := kubectl("apply", "-f", fluxImageReflectorURL).Run(); err != nil {
				return fmt.Errorf("flux install: %w", err)
			}
		}
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), "Installing Kuberik...")
		url := coreInstallURL
		args2 := []string{"apply", "-f", url}
		if bootstrapAll {
			args2 = []string{"apply", "-k", allInstallURL}
		}
		return kubectl(args2...).Run()
	},
}

func init() {
	bootstrapCmd.Flags().BoolVar(&bootstrapAll, "all", false, "Install all Kuberik integration controllers")
	bootstrapCmd.Flags().BoolVar(&bootstrapFluxOnly, "flux", true, "Install Flux first; set to false to skip Flux install")
	bootstrapCmd.Flags().StringVar(&bootstrapVersion, "kuberik-version", "", "Pin a specific Kuberik release (default: latest)")
	rootCmd.AddCommand(bootstrapCmd)
}
