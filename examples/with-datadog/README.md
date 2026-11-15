# Rollout with Datadog Health Check

Use Datadog as a health signal for a Kuberik Rollout. While the signal is healthy, the bake period proceeds; if Datadog reports a problem during bake, the rollout fails the bake. The datadog-controller is installed either way (it ships in the install bundle, or apply only the integration controller from its release).

Three modes, pick one:

| File | Mode | Requires Datadog Operator? |
| --- | --- | --- |
| [monitor.yaml](monitor.yaml) | Annotate a `DatadogMonitor` CRD | Yes |
| [healthcheck-datadog-api.yaml](healthcheck-datadog-api.yaml) | Poll monitors via the Datadog API directly (class `datadog`) | No |
| [healthcheck-datadog-incidents.yaml](healthcheck-datadog-incidents.yaml) | Go unhealthy while a matching incident is open (class `datadog-incidents`) | No |

## Credentials

The API modes use one set of Datadog credentials for the whole cluster (keys `api-key`, `app-key`, optional `site`). There is no per-HealthCheck secret. Create the Secret in the controller's namespace:

```bash
kubectl -n kuberik-system create secret generic datadog-credentials \
  --from-literal=api-key=YOUR_API_KEY --from-literal=app-key=YOUR_APP_KEY
```

How the controller gets it depends on how you installed it:

- **Helm chart:** the chart maps the Secret named by `integrations.datadog.existingSecret` (default `datadog-credentials`, in the release namespace) to the controller's environment. If you set `existingSecret: ""`, no credentials are wired and the API modes stay Pending.
- **Install bundle or the controller's own release:** the controller reads the `datadog-credentials` Secret in its own namespace.

## DatadogMonitor CRD

Requires the Datadog Operator and `DatadogMonitor` CRD installed, with the Datadog API key configured for the operator. The controller checks for the CRD at startup, so restart it after installing the Operator.

```bash
kubectl apply -f monitor.yaml
```

The datadog-controller sees the `kuberik.com/health-check: "true"` annotation and creates a `HealthCheck` named `datadog-check-my-app-error-rate` mirroring the monitor's state.

## Direct API Mode

No Datadog Operator needed. The controller polls the Datadog API for the monitors you select by ID or tag.

```bash
kubectl apply -f healthcheck-datadog-api.yaml
```

## Incident-Based Mode

Goes unhealthy while any open Datadog incident matches the search query, independent of any specific monitor.

```bash
kubectl apply -f healthcheck-datadog-incidents.yaml
```

## Wire it to your Rollout

A Rollout only considers HealthChecks that its `spec.healthCheckSelector` matches; with no selector it has none. Add the selector to the Rollout from [examples/basic-rollout](../basic-rollout). The HealthChecks here carry the label `app: my-app`:

```yaml
spec:
  healthCheckSelector:
    selector:
      matchLabels:
        app: my-app
    # HealthChecks in other namespaces: also set namespaceSelector.
```

Without `namespaceSelector`, only HealthChecks in the Rollout's own namespace are considered, so create the Rollout in the same namespace (`production`) as the HealthCheck.

## Inspect

```bash
kuberik get healthchecks -n production
kubectl describe healthcheck my-app-error-rate -n production
```

See [docs/healthchecks.md](../../docs/healthchecks.md) for how `HealthCheck` interacts with bake periods, including the `lastErrorTime` witness semantics.
