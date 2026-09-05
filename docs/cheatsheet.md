# Kuberik Cheatsheet

One-page reference. For depth, see the linked docs.

## Install

```bash
brew install kuberik/tap/kuberik              # CLI
kuberik bootstrap                             # Flux + Kuberik core
kuberik bootstrap --all                       # + all integrations
helm install kuberik kuberik/kuberik \
  -n kuberik-system --create-namespace \
  --set createNamespace=false
```

## CRDs at a glance

| Kind | Group | Scope | Purpose |
| --- | --- | --- | --- |
| `Rollout` | `kuberik.com/v1alpha1` | namespaced | Release pipeline |
| `RolloutGate` | same | namespaced | Boolean veto on a Rollout |
| `HealthCheck` | same | namespaced | Bake-period validation signal |
| `RolloutSchedule` | same | namespaced | Time-based RolloutGate |
| `ClusterRolloutSchedule` | same | cluster | Multi-namespace RolloutSchedule |
| `Environment` | `kuberik.com/v1alpha1` | namespaced | Multi-environment promotion order |

## Annotations the controller reads

```yaml
# On a Flux source the rollout should patch:
rollout.kuberik.com/rollout: "my-app"

# Or, to drive Kustomize substitution:
rollout.kuberik.com/substitute.IMAGE_TAG.from: "my-app"

# On a Rollout, to pause without changing gates:
rollout.kuberik.com/suspended: "true"

# Emergency: skip gates for a specific version (use with care):
rollout.kuberik.com/bypass-gates: "v1.2.3"

# On a DatadogMonitor, mirror its state to a HealthCheck:
kuberik.com/health-check: "true"
```

## CLI

```bash
kuberik install [--all]              # apply install manifests
kuberik uninstall
kuberik bootstrap [--all] [--flux=false]
kuberik check                        # CRDs + controller pods
kuberik get rollouts|gates|healthchecks [-A]
kuberik describe rollout|gate|healthcheck NAME
kuberik tree ROLLOUT                 # gates + healthchecks tree
kuberik approve GATE                 # spec.passing=true
kuberik reject GATE                  # spec.passing=false
kuberik suspend ROLLOUT
kuberik resume ROLLOUT
kuberik logs [-f] [--tail N]
kuberik events [-A] [-w]
kuberik init rollout|gate|schedule --name X
kuberik completion {bash,zsh,fish,powershell}
```

## Common one-liners

```bash
# Why is my rollout stuck?
kuberik tree my-app
kubectl describe rollout my-app -n production

# Bake fail with recovered metric? Check the witness:
kubectl get healthcheck X -o jsonpath='{.status.lastErrorTime}'

# Open all gates for one rollout:
kubectl get rolloutgate -n production -o name | \
  xargs -I{} kubectl patch {} --type=merge -p '{"spec":{"passing":true}}'

# Show recent controller events for a Rollout:
kubectl get events -n production --field-selector \
  involvedObject.name=my-app

# Suspend then resume:
kuberik suspend my-app -n production
kuberik resume  my-app -n production
```

## Metrics worth alerting on

```text
kuberik_rollout_bake_failures_total
kuberik_rollout_gate_blocked_seconds (p95)
kuberik_rollout_promotions_total{result="bypassed"}
controller_runtime_reconcile_errors_total{controller="rollout"}
```

Full alerting examples in [metrics.md](metrics.md).

## Deeper reading

- [Concepts](concepts.md) - what each CRD does and why
- [Gates](gates.md) / [Health Checks](healthchecks.md)
- [Operations Runbook](operations.md)
- [Troubleshooting](troubleshooting.md) / [FAQ](faq.md)
