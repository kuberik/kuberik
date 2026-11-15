# Kuberik CLI

The `kuberik` CLI is a small wrapper around `kubectl` that streamlines installing Kuberik, inspecting Kuberik resources, and operating common day-2 actions like approving gates.

## Install

### Homebrew

```bash
brew install kuberik/tap/kuberik
```

### Binary release

Download an archive for your OS/arch from [Releases](https://github.com/kuberik/kuberik/releases/latest):

```bash
curl -L -o kuberik.tar.gz \
  https://github.com/kuberik/kuberik/releases/latest/download/kuberik_$(uname -s | tr A-Z a-z)_amd64.tar.gz
tar xzf kuberik.tar.gz
sudo install kuberik /usr/local/bin/
```

### Container image

```bash
docker run --rm ghcr.io/kuberik/kuberik-cli:latest version
```

The container includes both `kuberik` and `kubectl`, so it works in CI without extra setup.

### From source

```bash
git clone https://github.com/kuberik/kuberik.git
cd kuberik
make build
sudo install bin/kuberik /usr/local/bin/
```

## Global flags

| Flag | Description |
| --- | --- |
| `--kubeconfig` | Path to a kubeconfig file. Defaults to `$KUBECONFIG` or `~/.kube/config`. |
| `-n, --namespace` | Kubernetes namespace. Defaults to `default`. |
| `-v, --version` | Print version and exit. |

## Commands

### `kuberik bootstrap`

One-shot first-time install: the Flux source-controller and kustomize-controller (Flux 2.6 or newer), followed by Kuberik. Image-reflector is not installed: a ReleaseSource scans the registry itself, so you only need it to bridge an existing ImagePolicy (`spec.oci.imagePolicyRef`). The Flux step uses the `flux` CLI (`flux install --export`), which must be on `PATH`.

```bash
kuberik bootstrap            # Flux source + kustomize controllers, then Kuberik core
kuberik bootstrap --all      # + all integration controllers
kuberik bootstrap --flux=false   # Kuberik only (Flux already installed)
```

### `kuberik install`

Install Kuberik on the cluster the current kubeconfig points at.

```bash
# Core controller only
kuberik install

# Core + all integration controllers
kuberik install --all
```

### `kuberik uninstall`

Remove the bundled Kuberik components from the cluster.

```bash
kuberik uninstall
```

### `kuberik check`

Verify Kuberik CRDs and the rollout-controller pod are present. Also reports, without failing, whether the ReleaseSource and RolloutApproval CRDs and Flux image-reflector are installed.

```bash
kuberik check
```

### `kuberik get`

Inspect Kuberik resources. Wraps `kubectl get` with the right resource kinds.

```bash
kuberik get rollouts            # in default namespace
kuberik get gates -A            # across all namespaces
kuberik get healthchecks -n production
```

### `kuberik approve` / `kuberik revoke`

Approve or withdraw one release candidate of a Rollout. `approve` creates the same `RolloutApproval` the dashboard's Approve button creates; `revoke` deletes it.

```bash
kuberik approve my-app v1.2.3 -n production
kuberik revoke  my-app v1.2.3 -n production
```

The object is named from the rollout and version (`my-app-v1-2-3`), owned by the Rollout, and carries your username in `approval.kuberik.com/by` (omitted for service accounts). Approving twice is a no-op; `revoke` of a missing approval succeeds. Approvals committed to git under other names are left alone and listed, because they keep the version approved.

### Stopping deploys

There is no `suspend` command. To hold a Rollout, pin it with `spec.wantedVersion` (set it to the version to stay on), or add a `RolloutSchedule` with `action: Deny` for the window (`kuberik init schedule`).

### `kuberik describe`

Describe a Kuberik resource by short kind name.

```bash
kuberik describe rollout my-app -n production
kuberik describe gate my-app-approval -n production
kuberik describe healthcheck my-app-error-rate -n production
```

Equivalent to `kubectl describe <kind>.kuberik.com <name>` but lets you use the short kind names (`rollout`, `gate`, `healthcheck`, `schedule`).

### `kuberik logs`

Stream logs from the rollout-controller pod(s).

```bash
kuberik logs            # last 100 lines
kuberik logs --tail 500
kuberik logs -f         # follow
```

### `kuberik init`

Scaffold a starter YAML manifest for a Kuberik resource. Writes to stdout so you can pipe it to a file or to `kubectl apply`.

```bash
kuberik init releasesource --name webapp --image ghcr.io/my-org/webapp > source.yaml
kuberik init rollout --name webapp > rollout.yaml
kuberik init gate --name webapp-approval --rollout webapp -n production > gate.yaml
kuberik init schedule --name business-hours > schedule.yaml
kuberik init healthcheck --name webapp-error-rate -n production > hc.yaml
```

Available kinds: `rollout`, `releasesource`, `gate`, `schedule`, `healthcheck`. A Rollout's `spec.releaseSourceRef` names the ReleaseSource (`--release-source`, default `--name`).

### `kuberik tree`

Print a tree of every RolloutGate and HealthCheck that references a Rollout. Useful for spotting which gate is blocking promotion at a glance.

```bash
kuberik tree my-app -n production
```

### `kuberik events`

Show Kubernetes events whose involved object is a Kuberik resource (Rollout, RolloutGate, HealthCheck, RolloutSchedule). Useful for diagnosing stuck rollouts.

```bash
kuberik events -n production
kuberik events -A
kuberik events -w        # stream
```

### `kuberik completion`

Print a shell completion script.

```bash
# Bash
source <(kuberik completion bash)

# Zsh
kuberik completion zsh > "${fpath[1]}/_kuberik"

# Fish
kuberik completion fish | source

# PowerShell
kuberik completion powershell | Out-String | Invoke-Expression
```

## Use in CI

The [Kuberik GitHub Action](../action/README.md) installs the CLI on a runner:

```yaml
- uses: kuberik/kuberik/action@main
- run: kuberik approve my-app v1.2.3 -n production
```
