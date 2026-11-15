package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	bootstrapAll      bool
	bootstrapFluxOnly bool
	bootstrapVersion  string
)

// fluxComponents is what Kuberik needs from Flux: the rollout-controller patches
// Flux sources and Kustomizations. Releases come from a ReleaseSource, which scans
// the registry itself, so image-reflector-controller is not installed.
const fluxComponents = "source-controller,kustomize-controller"

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Install the Flux controllers Kuberik needs, then Kuberik",
	Long: `Run the full first-time install: the Flux source-controller and
kustomize-controller (Flux 2.6 or newer), followed by the Kuberik core
controller (or the full bundle with --all).

Equivalent to:
  flux install --components=source-controller,kustomize-controller
  kuberik install [--all]

Flux image-reflector-controller is not installed: Kuberik gets releases from
a ReleaseSource, which scans the registry itself. Install image-reflector
yourself only if a ReleaseSource bridges an existing ImagePolicy
(spec.oci.imagePolicyRef).

The Flux step runs the flux CLI ("flux install --export"), which must be on
PATH. Skip it with --flux=false if your cluster already has Flux.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		if bootstrapFluxOnly {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Installing Flux "+fluxComponents+"...")
			if err := installFlux(); err != nil {
				return fmt.Errorf("flux install: %w", err)
			}
		}
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Installing Kuberik...")
		if bootstrapAll {
			return kubectl(applyArgs("-k", allInstallURL)...).Run()
		}
		return kubectl(applyArgs("-f", coreInstallURL)...).Run()
	},
}

// minFluxMinor is the oldest Flux 2.x minor Kuberik supports.
const minFluxMinor = 6

// fluxVersionRE finds the version in `flux --version` ("flux version 2.7.1").
var fluxVersionRE = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// checkFluxVersion rejects a flux CLI older than 2.6.
func checkFluxVersion(out string) error {
	m := fluxVersionRE.FindStringSubmatch(out)
	if m == nil {
		return fmt.Errorf("cannot read the flux version from %q", strings.TrimSpace(out))
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 2 || (major == 2 && minor < minFluxMinor) {
		return fmt.Errorf("flux %s.%s.%s is too old: Kuberik needs Flux 2.%d or newer", m[1], m[2], m[3], minFluxMinor)
	}
	return nil
}

// installFlux applies only the Flux controllers Kuberik needs. The published
// install.yaml always contains every component (image controllers included),
// so the manifests are rendered with the flux CLI and applied with kubectl.
func installFlux() error {
	if _, err := exec.LookPath("flux"); err != nil {
		return fmt.Errorf("flux CLI not found on PATH (https://fluxcd.io/flux/installation/); install it, or use --flux=false if the cluster already has Flux 2.%d+: %w", minFluxMinor, err)
	}
	ver, err := exec.Command("flux", "--version").Output()
	if err != nil {
		return fmt.Errorf("flux --version: %w", err)
	}
	if err := checkFluxVersion(string(ver)); err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd := exec.Command("flux", "install", "--components="+fluxComponents, "--export")
	cmd.Stderr = &stderr
	manifests, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("flux install --export: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	_, err = kubectlRun(string(manifests), applyArgs("-f", "-")...)
	return err
}

func init() {
	bootstrapCmd.Flags().BoolVar(&bootstrapAll, "all", false, "Install all Kuberik integration controllers")
	bootstrapCmd.Flags().BoolVar(&bootstrapFluxOnly, "flux", true, "Install the Flux controllers first; set to false if the cluster already has Flux 2.6+")
	bootstrapCmd.Flags().StringVar(&bootstrapVersion, "kuberik-version", "", "Pin a specific Kuberik release (default: latest)")
	rootCmd.AddCommand(bootstrapCmd)
}
