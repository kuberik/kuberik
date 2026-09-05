# Rollout with Datadog Health Check

Use Datadog as a health signal for a Kuberik Rollout. While the signal is healthy, the bake period proceeds; if Datadog reports a problem during bake, the rollout is paused. The datadog-controller is installed either way (it ships in the install bundle, or apply only the integration controller from its release).

Three modes, pick one:

| File | Mode | Requires Datadog Operator? |
| --- | --- | --- |
| [monitor.yaml](monitor.yaml) | Annotate a `DatadogMonitor` CRD | Yes |
| [healthcheck-datadog-api.yaml](healthcheck-datadog-api.yaml) | Poll a monitor via the Datadog API directly | No |
| [healthcheck-datadog-incidents.yaml](healthcheck-datadog-incidents.yaml) | Go unhealthy while a labeled incident is open | No |

## DatadogMonitor CRD

Requires the Datadog Operator and `DatadogMonitor` CRD installed, with the Datadog API key configured for the operator.

```bash
kubectl apply -f monitor.yaml
```

The datadog-controller sees the `kuberik.com/health-check: "true"` annotation and creates a `HealthCheck` resource named `my-app-error-rate` mirroring the monitor's state.

## Direct API Mode

No Datadog Operator needed - the controller polls the Datadog API for an existing monitor's status. Requires a secret with your `api-key`/`app-key` (see file header).

```bash
kubectl apply -f healthcheck-datadog-api.yaml
```

## Incident-Based Mode

Goes unhealthy while any open Datadog incident matches the given labels, independent of any specific monitor. Same credentials secret as direct API mode.

```bash
kubectl apply -f healthcheck-datadog-incidents.yaml
```

## Wire it to your Rollout

The HealthCheck must reference the Rollout. The datadog-controller does this automatically based on label selection - any Rollout labeled to match the HealthCheck's `selector` will pick it up. Adjust labels on your Rollout accordingly.

## Inspect

```bash
kuberik get healthchecks -n production
kubectl describe healthcheck my-app-error-rate -n production
```

See [docs/healthchecks.md](../../docs/healthchecks.md) for how `HealthCheck` interacts with bake periods, including the `lastErrorTime` witness semantics.
