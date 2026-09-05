# API Reference

The Kuberik CRDs and what they do. Field-level reference is generated from the controller source and lives at [kuberik.com/docs/api](https://kuberik.com/docs/api/).

| Kind | Group / Version | Scope | Owner | Purpose |
| --- | --- | --- | --- | --- |
| `Rollout` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Drives a release pipeline: tracks one ImagePolicy, evaluates gates and health checks, patches the target Flux resource, bakes |
| `RolloutGate` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Boolean veto on a Rollout (`spec.passing`) |
| `HealthCheck` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Observed health signal read during bake; produced by integration controllers |
| `RolloutSchedule` | `kuberik.com/v1alpha1` | Namespaced | rollout-controller | Time-based RolloutGate (e.g. business-hours-only) |
| `ClusterRolloutSchedule` | `kuberik.com/v1alpha1` | Cluster | rollout-controller | Like RolloutSchedule but selects target namespaces |
| `Environment` | `kuberik.com/v1alpha1` | Namespaced | environment-controller | Logical deployment target (staging/production), maps to GitHub Deployments |
| `PrometheusHealthCheck` | `kuberik.com/v1alpha1` | Namespaced | prometheus-controller | PromQL query that produces a HealthCheck |
| `DatadogMonitor` _(annotated)_ | `datadoghq.com/v1alpha1` | Namespaced | Datadog Operator (consumed by datadog-controller) | Datadog monitor; when annotated `kuberik.com/health-check=true`, mirrored to a HealthCheck |

## Annotations the controllers honor

| Annotation | Reads on | Effect |
| --- | --- | --- |
| `rollout.kuberik.com/rollout` | `OCIRepository`, `Kustomization` | Marks the resource as the Rollout's target. The controller patches its image tag when a release is promoted. |
| `rollout.kuberik.com/substitute.<VAR>.from` | `Kustomization` | Patches `spec.postBuild.substitute.<VAR>` with the latest image tag from the named ImagePolicy. |
| `rollout.kuberik.com/suspended` | `Rollout` | When `true`, the controller pauses promotion without disturbing gate/health evaluation. |
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
