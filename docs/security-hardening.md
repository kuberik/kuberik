# Security Hardening

Recommended settings for production Kuberik installs. Each item is opt-in via Helm values or a small kubectl change.

## Controller security

The chart already ships hardened defaults: non-root user, read-only root filesystem, dropped capabilities, RuntimeDefault seccomp profile, NonRoot pod security. No changes needed if you use the chart.

If you install via kustomize (`kuberik install`) and want to enforce them, copy the security blocks from `chart/kuberik/templates/rollout-controller-deployment.yaml` into a kustomize patch.

## RBAC scope

The rollout-controller installs cluster-wide RBAC because it reconciles `ClusterRolloutSchedule` and reads release catalogues (`ReleaseSlice`) across namespaces. For multi-tenant clusters where rollout-controller should be scoped to specific namespaces:

1. Don't apply the `clusterrolloutschedules.kuberik.com` CRD.
2. Replace the `rollout-controller-manager` ClusterRole with a namespaced Role per tenant namespace.
3. Use a Role/RoleBinding pair per tenant. Drop the ClusterRoleBinding from the install.

The chart does not ship a "namespaced-only" mode today - this is a manual fork.

## Network policy

Enable the chart's NetworkPolicy:

```bash
helm upgrade kuberik kuberik/kuberik -n kuberik-system \
  --set networkPolicy.enabled=true \
  --set 'networkPolicy.metricsScrapeFrom[0].namespaceSelector.matchLabels.name=monitoring'
```

This allows scraping only from the `monitoring` namespace (label-selected) and restricts egress to DNS + kube-apiserver.

Add Datadog / Prometheus outbound egress as `networkPolicy.extraEgress` entries.

## Audit logging

Kuberik does not store its own audit log - all state changes go through the Kubernetes API, so the cluster audit log is authoritative. Recommend tracking `rolloutgates.kuberik.com` and `rollouts.kuberik.com` annotation changes:

```yaml
# audit-policy.yaml
- level: RequestResponse
  resources:
    - group: kuberik.com
      resources: ["rolloutgates", "rollouts"]
  verbs: ["create", "update", "patch", "delete"]
```

Apply via `--audit-policy-file=audit-policy.yaml` on the kube-apiserver.

## Bypass-gates discipline

The `rollout.kuberik.com/bypass-gates` annotation lets a release skip gates entirely - useful for emergencies, dangerous in steady state. Recommended controls:

- Add an OPA / Kyverno policy that requires a justification annotation alongside `bypass-gates` and matches it to a Slack/PagerDuty incident ID.
- Alert on `kuberik_rollout_promotions_total{result="bypassed"}` (controller metric).
- Code-review the bypass annotation via a GitOps `Kustomization` rather than ad-hoc `kubectl annotate`.

## ServiceAccount scope

The chart's helm test pod uses the rollout-controller's SA by default (it needs list-deployment rights). For long-lived test pods or external CI integrations, create a minimal SA bound to `rollout-viewer-role` only:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kuberik-readonly
  namespace: kuberik-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kuberik-readonly
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: rollout-controller-rollout-viewer-role
subjects:
  - kind: ServiceAccount
    name: kuberik-readonly
    namespace: kuberik-system
```

Then point `tests.serviceAccountName: kuberik-readonly` in values.

## Image provenance

Controllers ship from `ghcr.io/kuberik/`. To enforce only signed images, deploy [cosign](https://docs.sigstore.dev/) policy controllers (cosigned, Kyverno) and pin to `@sha256:...` digests in your values overrides:

```yaml
rolloutController:
  image:
    tag: ""  # Leave blank to let appVersion resolve
    repository: ghcr.io/kuberik/rollout-controller@sha256:<digest>
```

Future versions will publish cosign signatures and SLSA provenance per release.

## Secret-handling discipline

Kuberik resources never hold secret material - HealthCheck and RolloutGate spec/status are public-by-design. Don't put webhook URLs, API tokens, or credentials in them. Pass those to integration controllers via Kubernetes Secrets (e.g. Datadog API key in the Datadog Operator's Secret, not in a Kuberik resource).

## Reporting issues

See [SECURITY.md](../SECURITY.md) for vulnerability reporting.
