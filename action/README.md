# Kuberik GitHub Action

Install the [Kuberik CLI](https://github.com/kuberik/kuberik) on Linux, macOS, or Windows GitHub Actions runners.

## Usage

```yaml
- name: Setup Kuberik CLI
  uses: kuberik/kuberik/action@main
  with:
    version: 'latest'

- name: Use Kuberik
  run: kuberik version
```

## Inputs

| Name | Description | Required |
| --- | --- | --- |
| `version` | CLI version (e.g. `0.1.0`). Defaults to latest stable release. | no |
| `bindir` | Override the install directory. Defaults to a path under `$RUNNER_TOOL_CACHE`. | no |
| `token` | GitHub token for the releases API (avoids rate limits). | no |

## Examples

### Approve a version from CI

Approvals are per-version `RolloutApproval` objects. The identity running this step needs the `kuberik-rollout-approver` role.

```yaml
- uses: azure/setup-kubectl@v4
- run: |
    kubectl apply -f - <<EOF
    apiVersion: kuberik.com/v1alpha1
    kind: RolloutApproval
    metadata:
      name: my-app-${VERSION//./-}
      namespace: production
    spec:
      rolloutRef:
        name: my-app
      version: "${VERSION}"
    EOF
  env:
    VERSION: "1.2.3"
```

### Install Kuberik on a kind cluster for integration tests

```yaml
- uses: helm/kind-action@v1
- uses: kuberik/kuberik/action@main
- run: kuberik install --all
- run: kuberik check
```
