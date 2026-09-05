# Health Checks

A `HealthCheck` records the observed health of something - a workload, a metric, an external dependency. The rollout-controller reads `HealthCheck` status during the bake period to decide whether a promotion is stable.

`HealthCheck` resources are normally created by integration controllers, not by hand. Your Rollout does not care who produced them; it just reads the resulting status.

## Anatomy

```yaml
apiVersion: kuberik.com/v1alpha1
kind: HealthCheck
metadata:
  name: my-app-error-rate
  namespace: production
  labels:
    app: my-app          # Rollout's spec.healthCheckSelector matches on these labels
spec:
  class: datadog         # optional, names the producing integration
status:
  healthy: true
  lastUpdateTime: "2026-05-17T12:34:56Z"
  lastErrorTime: "2026-05-15T03:11:00Z"
  message: "p99 error rate 0.12% (threshold 1%)"
```

Key status fields:

- `healthy`: current state - true means the rollout-controller treats this signal as good.
- `lastUpdateTime`: when the source controller last evaluated the signal.
- `lastErrorTime`: a witness of the last unhealthy observation. **It survives recovery** - it is not cleared when the check goes healthy again. This lets the rollout-controller and step-gate logic catch transient failures inside the bake window that they might otherwise miss.
- `message`: human-readable detail. Shown by `kubectl describe` and the dashboard.

## Producers

| Controller | What it watches |
| --- | --- |
| datadog-controller | `DatadogMonitor` resources annotated `kuberik.com/health-check: "true"` |
| prometheus-controller | PromQL queries or alert states |
| Your custom controller | Anything you can compute - synthetic probes, smoke test job results, dependency dashboards |

## Bake-period semantics

When a Rollout promotes a release, it enters the bake period (`spec.bakeTime`). During bake:

1. The controller watches every `HealthCheck` whose labels match the Rollout's `spec.healthCheckSelector`.
2. If any `HealthCheck.status.healthy` is `false` _or_ `lastErrorTime` is more recent than the bake start, the bake is failing.
3. A failing bake either pauses the Rollout (default) or triggers a rollback (if `spec.rollbackOnFailedBake` is true).
4. Once bake time elapses with no errors witnessed, the release is recorded in `status.versionHistory` and the next promotion can begin.

Because `lastErrorTime` survives recovery, a brief flap mid-bake will fail the bake even if the metric recovers before the controller re-reads it. This is intentional - "saw an error during bake" is a stronger signal than "currently healthy."

## Writing a custom HealthCheck producer

The minimum is a controller that:

1. Watches some external signal (a metric, a probe, an SLO burn rate).
2. Creates or updates a `HealthCheck` resource carrying labels that match the target Rollout's `spec.healthCheckSelector`.
3. Sets `status.healthy`, `status.lastUpdateTime`, and (when something goes wrong) `status.lastErrorTime`.

Use the existing integration controllers as a reference - they are small Go programs that wrap a single external system in this shape.

## Inspecting

```bash
kuberik get healthchecks -A
kubectl describe healthcheck my-app-error-rate -n production
```

The Rollout's status conditions list which health check, if any, is blocking the current bake:

```bash
kubectl describe rollout my-app
```
