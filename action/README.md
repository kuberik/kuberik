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

Approvals are per-version `RolloutApproval` objects. `kuberik approve` creates the same one the dashboard's Approve button creates (deterministic name, owned by the Rollout), so `kuberik revoke` can withdraw it. The CLI calls `kubectl`, and the identity running this step needs the `kuberik-rollout-approver` role.

```yaml
- uses: azure/setup-kubectl@v4
- uses: kuberik/kuberik/action@main
- run: kuberik approve my-app "$VERSION" -n production
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
