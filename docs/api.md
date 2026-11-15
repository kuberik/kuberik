# API Reference

The Kuberik CRDs and what they do. Field-level reference is generated from the controller source and lives at [kuberik.com/docs/api](https://kuberik.com/docs/api/).

| Kind | Group / Version | Scope | Owner | Purpose |
| --- | --- | --- | --- | --- |
| `Rollout` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Drives a release pipeline: reads releases from one ReleaseSource, evaluates gates and health checks, patches the target Flux resource, bakes |
| `RolloutGate` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Boolean veto on a Rollout (`spec.passing`). Owned gates are written by schedules, approval policies and environment-controller; hand-written ones are a kill switch, not an approval |
| `ApprovalPolicy` / `ClusterApprovalPolicy` | `kuberik.com/v1alpha1` | Namespaced / Cluster | rollout-controller | Selects Rollouts that need a per-version approval; owns the gate |
| `RolloutApproval` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | One approval of one version of one Rollout |
| `ReleaseSource` | `kuberik.com/v1alpha1` | Namespaced | release-controller | Lists an image's tags from the registry and keeps the release catalogue a Rollout reads |
| `HealthCheck` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Observed health signal read during bake; produced by integration controllers |
| `RolloutSchedule` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Time-based RolloutGate (e.g. business-hours-only) |
| `ClusterRolloutSchedule` | `kuberik.com/v1alpha1` | Cluster | rollout-controller | Like RolloutSchedule but selects target namespaces |
| `Environment` | `environments.kuberik.com/v1alpha1` | Namespaced | environment-controller | One tier (dev, test, staging, prod) of an app, maps to GitHub Deployments; `site` and `wave` place it in a multi-cluster rollout |
| `DatadogMonitor` _(annotated)_ | `datadoghq.com/v1alpha1` | Namespaced | Datadog Operator (consumed by datadog-controller) | Datadog monitor; when annotated `kuberik.com/health-check=true`, mirrored to a HealthCheck |

## Annotations the controllers honor

| Annotation | Reads on | Effect |
| --- | --- | --- |
| `rollout.kuberik.com/rollout` | `OCIRepository` | Marks the resource as the Rollout's target. The controller patches its `spec.ref.tag` when a release is promoted. |
| `rollout.kuberik.com/substitute.<VAR>.from` | `Kustomization` | Patches `spec.postBuild.substitute.<VAR>` with the release tag. The value is the name of the Rollout. |
| `rollout.kuberik.com/bypass-gates` | `Rollout` | When set to a version, the controller deploys that version without waiting for gates. **Emergency use only.** |
| `kuberik.com/health-check` | `DatadogMonitor` | When `true`, datadog-controller creates a HealthCheck mirroring the monitor. |

## Reading status

Every Kuberik resource sets standard Kubernetes conditions plus type-specific status fields. Useful one-liners:

```bash
# Why is my Rollout stuck?
kubectl get rollout my-app -o jsonpath='{.status.conditions}' | jq

# Current bake state
kubectl get rollout my-app -o jsonpath='{.status.bakingRelease}'

# Last unhealthy time on a HealthCheck (witness that survives recovery)
kubectl get healthcheck my-app-error-rate -o jsonpath='{.status.lastErrorTime}'
```

See also: [Concepts](concepts.md), [Gates](gates.md), [Health Checks](healthchecks.md).
