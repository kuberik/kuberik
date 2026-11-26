# Operating Kuberik

Day-2 runbook for teams running Kuberik in production. Companion to [troubleshooting](troubleshooting.md), which is for diagnosing one-off incidents; this page is for steady-state operation.

## Daily

### Watch the rollouts dashboard

```bash
kuberik get rollouts -A
kuberik get gates -A
```

Or open the dashboard (`dashboard.enabled=true` in the chart) and watch the per-namespace overview.

Anything stuck for > 30 min that does not match an active gate window warrants investigation.

### Inspect controller health

```bash
kuberik logs --tail 500
kuberik events -A
```

Look for `reconcile error` lines (counted by the `controller_runtime_reconcile_errors_total` metric).

## Weekly

### Check component versions

The `check-component-releases` workflow runs weekly and opens a tracking issue when an integration controller has a newer release than what's pinned. Resolve the issue by:

1. Bumping the version in `config/install/kustomization.yaml`.
2. Bumping the matching `default` tag in `chart/kuberik/templates/integrations-*.yaml` if the chart pins it explicitly.
3. Re-applying or re-deploying through Flux.

### Review long-blocked rollouts

```bash
kubectl get rolloutgate -A -o jsonpath='{range .items[?(@.spec.passing==false)]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}'
```

For each blocked gate, check who owns it (label `owner=` or annotation) and ping the owner if it's been blocked > 1 week without movement.

## Per release

### Promote a release

Most teams let Kuberik auto-promote when gates pass. Where an `ApprovalPolicy` applies, approve the version in the dashboard, or create the approval yourself:

```bash
kubectl apply -f - <<EOF
apiVersion: kuberik.com/v1alpha1
kind: RolloutApproval
metadata:
  name: <rollout>-<version-with-dashes>
  namespace: <namespace>
spec:
  rolloutRef:
    name: <rollout>
  version: "<version>"
EOF
```

### Stop a Rollout from deploying (without disabling gates)

Pin it to the version it runs now:

```bash
kubectl patch rollout <rollout> -n <namespace> \
  --type merge -p '{"spec":{"wantedVersion":"<current version>"}}'
# ... investigate, then remove the pin
kubectl patch rollout <rollout> -n <namespace> \
  --type merge -p '{"spec":{"wantedVersion":null}}'
```

To stop a group of Rollouts for a period, use a `RolloutSchedule` with `action: Deny`.

### Emergency: bypass gates

```bash
kubectl annotate rollout <rollout> -n <ns> \
  rollout.kuberik.com/bypass-gates=v1.2.3 \
  --overwrite
```

Document the incident in the annotation namespace (e.g. `incident.kuberik.com/ref=INC-12345`). Remove the bypass when the emergency clears:

```bash
kubectl annotate rollout <rollout> -n <ns> rollout.kuberik.com/bypass-gates-
```

## Per quarter

### Review SLO posture

Pull these metrics across the past quarter:

- `kuberik_rollout_bake_failures_total{reason=*}` - are bakes failing for new reasons?
- `kuberik_rollout_gate_blocked_seconds` (p95) - which gates eat the most time?
- `kuberik_rollout_promotions_total{result="bypassed"}` - is the bypass annotation getting used? If yes, the gate or process upstream of it may be wrong.

### Audit gate inventory

```bash
kubectl get rolloutgate -A -o yaml | yq '.items[].spec.rolloutRef.name' | sort | uniq -c | sort -rn
```

A single Rollout with > 5 gates is usually a signal the gate model has drifted - consider consolidating.

## Backups

Kuberik state lives entirely in CRD instances:

- `Rollout`, `RolloutGate`, `HealthCheck`, `RolloutSchedule`
- `ClusterRolloutSchedule`
- `Environment` (if environment-controller is installed)

Back them up alongside your other Kubernetes resources (Velero, etcd snapshots, or `kubectl get -o yaml > backup.yaml`). The controllers themselves are stateless; you can reinstall them at any time without losing state.

## Upgrading

See [docs/upgrade.md](upgrade.md). TL;DR: CRDs first, then rollout-controller, then integration controllers, then dashboard.
