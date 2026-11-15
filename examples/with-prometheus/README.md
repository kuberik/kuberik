# Rollout with Prometheus Health Check

Gate the bake period on a Prometheus alert. The prometheus-controller polls Prometheus for firing alerts and updates a `HealthCheck` of class `prometheus-alert`: it is `Unhealthy` while a matching alert fires.

## Prerequisites

- prometheus-controller is installed. It has no published release yet; build it from source (`make deploy` in the [prometheus-controller](https://github.com/kuberik/prometheus-controller) repo).
- Prometheus is reachable from the prometheus-controller pod.

## Files

- `healthcheck.yaml` - the `HealthCheck` that goes unhealthy while the alert `HighP99Latency` fires for `my-app`.

The alert itself lives in Prometheus. As a Prometheus Operator `PrometheusRule` it looks like this; any way of defining an alert works, only its labels matter to the HealthCheck:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: my-app-latency
  namespace: monitoring
spec:
  groups:
    - name: my-app
      rules:
        - alert: HighP99Latency
          expr: |
            histogram_quantile(0.99,
              sum(rate(http_request_duration_seconds_bucket{service="my-app"}[5m]))
              by (le)
            ) > 0.5
          for: 1m
          labels:
            service: my-app
```

## Apply

```bash
kubectl apply -f healthcheck.yaml
```

## Wire it to your Rollout

A Rollout only considers HealthChecks that its `spec.healthCheckSelector` matches. The HealthCheck carries the label `app: my-app`:

```yaml
spec:
  healthCheckSelector:
    selector:
      matchLabels:
        app: my-app
```

Without `namespaceSelector`, only HealthChecks in the Rollout's own namespace are considered, so create the Rollout in `production` too.

## Annotations

- `healthcheck.kuberik.com/prometheus-url` (required): the Prometheus server.
- `healthcheck.kuberik.com/prometheus-alert-labels` (required): comma-separated `key=value` labels an alert must carry to count.
- `healthcheck.kuberik.com/prometheus-credentials-secret` (optional): Secret with a bearer `token`, or `username` and `password`.
- `healthcheck.kuberik.com/requeue-interval` (optional): poll interval, default 30s, minimum 5s.

## Inspect

```bash
kuberik get healthchecks -n production
kubectl describe healthcheck my-app-latency -n production
```
