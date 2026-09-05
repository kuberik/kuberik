# Architecture

Kuberik is a set of Kubernetes controllers that together implement progressive delivery. Each component is a separate controller with its own CRDs and can be installed independently.

## Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         Cluster                             │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                  rollout-controller                  │   │
│  │                                                      │   │
│  │  Rollout  ──▶  RolloutGate  ──▶  HealthCheck         │   │
│  │     │              │                                 │   │
│  │     ▼              ▼                                 │   │
│  │  ImagePolicy   RolloutSchedule                       │   │
│  └──────────────────────────────────────────────────────┘   │
│           │                    │                            │
│           ▼                    ▼                            │
│  ┌─────────────┐    ┌──────────────────────┐               │
│  │  OCIRepo /  │    │  Integration         │               │
│  │  Kustomize  │    │  Controllers         │               │
│  │  (Flux)     │    │  (Datadog, Prom,     │               │
│  └─────────────┘    │   OpenKruise, Env)   │               │
│                     └──────────────────────┘               │
│                                                             │
│  ┌──────────────────────────────┐                          │
│  │       rollout-dashboard      │                          │
│  └──────────────────────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

## rollout-controller

The core of Kuberik. It owns the following CRDs:

- **Rollout** - defines a release pipeline: what image to track, how long to bake, which gates must pass
- **RolloutGate** - a boolean condition that must be `passing: true` for a rollout to proceed
- **HealthCheck** - records the health state of a workload; created by integration controllers
- **RolloutSchedule / ClusterRolloutSchedule** - time-based gates; automatically creates RolloutGate resources on a schedule

The controller integrates with Flux's `ImagePolicy` to discover new releases and patches `OCIRepository` or `Kustomization` resources when a release is promoted.

Source: [kuberik/rollout-controller](https://github.com/kuberik/rollout-controller)

## Integration Controllers

Integration controllers extend Kuberik by producing `HealthCheck` or `RolloutGate` resources that the rollout-controller consumes.

### datadog-controller

Watches `DatadogMonitor` resources. When a monitor has the `kuberik.com/health-check: "true"` annotation, the controller creates and keeps a `HealthCheck` resource in sync with the monitor's health state.

Source: [kuberik/datadog-controller](https://github.com/kuberik/datadog-controller)

### prometheus-controller

Creates `HealthCheck` resources from Prometheus alert and query results.

Source: [kuberik/prometheus-controller](https://github.com/kuberik/prometheus-controller)

### environment-controller

Watches `Environment` resources and reports deployment status to the GitHub Deployments API. Also manages environment relationships (After, Parallel) and automatically creates `RolloutGate` resources.

Source: [kuberik/environment-controller](https://github.com/kuberik/environment-controller)

### openkruise-controller

Integrates OpenKruise advanced rollout strategies (canary, blue-green) with the Kuberik gate system, creating `RolloutGate` resources that reflect strategy progress.

Source: [kuberik/openkruise-controller](https://github.com/kuberik/openkruise-controller)

## rollout-dashboard

A web UI that reads rollout status across namespaces and presents a unified view of in-flight deployments, gate states, and health checks.

Source: [kuberik/rollout-dashboard](https://github.com/kuberik/rollout-dashboard)

## Data Flow

1. Flux's `ImagePolicy` detects a new image tag matching the semver range.
2. The rollout-controller reads the `latestRef` from the `ImagePolicy` status.
3. The controller evaluates all `RolloutGate` resources referenced by the `Rollout`. If any gate is not passing, the rollout waits.
4. When all gates pass, the controller patches the annotated `OCIRepository` or `Kustomization` with the new tag.
5. During the bake period, the controller monitors `HealthCheck` resources. If any check becomes unhealthy, the rollout can be blocked or rolled back.
6. After bake time completes with all health checks passing, the rollout records the release in its version history.

## CRD Ownership

| CRD | Owner |
| --- | --- |
| `Rollout`, `RolloutGate`, `HealthCheck`, `RolloutSchedule`, `ClusterRolloutSchedule` | rollout-controller |
| `Environment` | environment-controller |
