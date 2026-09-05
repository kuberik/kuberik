# Controller Metrics

The Kuberik controllers expose Prometheus metrics on port `8080` at `/metrics` (rollout-controller) and equivalent paths on each integration controller. This page lists the most useful series; see each controller's own README for the complete schema.

## rollout-controller

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `kuberik_rollout_promotions_total` | counter | `rollout`, `namespace`, `result` (`succeeded`/`failed`) | Number of release promotions attempted |
| `kuberik_rollout_bake_failures_total` | counter | `rollout`, `namespace`, `reason` | Bake failures by reason (`unhealthy`, `lastErrorTime`, `manual_reject`) |
| `kuberik_rollout_gate_blocked_seconds` | histogram | `rollout`, `gate`, `namespace` | How long a rollout sat blocked by a specific gate |
| `kuberik_rollout_active` | gauge | `rollout`, `namespace`, `phase` | Phase distribution of in-flight rollouts (`waiting_for_gate`, `baking`, `promoted`) |
| `controller_runtime_reconcile_total` | counter | `controller`, `result` | Standard controller-runtime reconciliation counters |
| `controller_runtime_reconcile_errors_total` | counter | `controller` | Reconcile error counter |

## Integration controllers

Each integration controller exposes:

- `<controller>_healthcheck_updates_total` - counter, label `result`, increments on each update written to a `HealthCheck`.
- `<controller>_source_unavailable_total` - counter, increments when the external source (Datadog API, Prometheus, etc.) is unreachable.
- `controller_runtime_*` metrics from controller-runtime.

## Scraping

The install bundle ships a `ServiceMonitor` per controller for the Prometheus Operator. If you use plain Prometheus scrape configs:

```yaml
- job_name: kuberik-rollout-controller
  kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names: [kuberik-system]
  relabel_configs:
    - source_labels: [__meta_kubernetes_service_label_app_kubernetes_io_name]
      regex: rollout-controller
      action: keep
    - source_labels: [__meta_kubernetes_endpoint_port_name]
      regex: metrics
      action: keep
```

## Suggested alerts

```yaml
groups:
- name: kuberik
  rules:
    - alert: KuberikRolloutBakeFailures
      expr: increase(kuberik_rollout_bake_failures_total[15m]) > 0
      for: 5m
      annotations:
        summary: "Rollout {{ $labels.rollout }} has failing bakes"

    - alert: KuberikRolloutGateBlockedLong
      expr: |
        histogram_quantile(0.95,
          sum(rate(kuberik_rollout_gate_blocked_seconds_bucket[1h])) by (le, gate, rollout)
        ) > 3600
      for: 30m
      annotations:
        summary: "Gate {{ $labels.gate }} blocking {{ $labels.rollout }} >1h"

    - alert: KuberikReconcileErrors
      expr: rate(controller_runtime_reconcile_errors_total{controller="rollout"}[5m]) > 0.1
      for: 10m
      annotations:
        summary: "rollout-controller reconcile error rate elevated"
```

These are starting points - tune the thresholds and `for` durations to your traffic profile.
