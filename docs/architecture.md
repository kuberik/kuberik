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
│  │  ReleaseSource  RolloutSchedule                      │   │
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

- **Rollout** - defines a release pipeline: which ReleaseSource to read, how long to bake, which gates must pass
- **RolloutGate** - a boolean condition that must be `passing: true` for a rollout to proceed
- **ApprovalPolicy / ClusterApprovalPolicy / RolloutApproval** - per-version manual approval; a policy owns the gate and each approval is one object
- **HealthCheck** - records the health state of a workload; created by integration controllers
- **RolloutSchedule / ClusterRolloutSchedule** - time-based gates; automatically creates RolloutGate resources on a schedule

The controller reads releases from a `ReleaseSource` and patches `OCIRepository` or `Kustomization` resources when a release is promoted.

Source: [kuberik/rollout-controller](https://github.com/kuberik/rollout-controller)

## release-controller

Owns `ReleaseSource`. It lists an image's tags in the registry itself, orders them by the source's policy, and keeps the catalogue the rollout-controller reads. Flux's image-reflector-controller is not required; a source can optionally read an existing `ImagePolicy` through `spec.oci.imagePolicyRef`.

Source: [kuberik/release-controller](https://github.com/kuberik/release-controller)

## Integration Controllers

Integration controllers extend Kuberik by producing `HealthCheck` or `RolloutGate` resources that the rollout-controller consumes.

### datadog-controller

Watches `DatadogMonitor` resources. When a monitor has the `kuberik.com/health-check: "true"` annotation, the controller creates and keeps a `HealthCheck` resource in sync with the monitor's health state.

Source: [kuberik/datadog-controller](https://github.com/kuberik/datadog-controller)

### prometheus-controller

Evaluates `HealthCheck` resources of class `prometheus-alert`: a check is unhealthy while a Prometheus alert matching its labels fires.

Source: [kuberik/prometheus-controller](https://github.com/kuberik/prometheus-controller)

### environment-controller

Watches `Environment` resources and reports deployment status to the GitHub Deployments API. Orders tiers (dev, test, staging, prod) and sites in waves by what GitHub records, and automatically creates `RolloutGate` resources. Clusters do not talk to each other; promotion is per cluster, coordinated through GitHub Deployments.

Source: [kuberik/environment-controller](https://github.com/kuberik/environment-controller)

### openkruise-controller

Integrates OpenKruise advanced rollout strategies (canary, blue-green) with the Kuberik gate system, creating `RolloutGate` resources that reflect strategy progress.

Source: [kuberik/openkruise-controller](https://github.com/kuberik/openkruise-controller)

## rollout-dashboard

A web UI that reads rollout status across namespaces and presents a unified view of in-flight deployments, gate states, and health checks.

Source: [kuberik/rollout-dashboard](https://github.com/kuberik/rollout-dashboard)

## Data Flow

1. release-controller's `ReleaseSource` finds a new image tag matching its policy and adds it to the catalogue.
2. The rollout-controller reads the new release from that catalogue.
3. The controller evaluates all `RolloutGate` resources referenced by the `Rollout`. If any gate is not passing, the rollout waits.
4. When all gates pass, the controller patches the annotated `OCIRepository` or `Kustomization` with the new tag.
5. During the bake period, the controller monitors `HealthCheck` resources. If any check becomes unhealthy, the rollout can be blocked or rolled back.
6. After bake time completes with all health checks passing, the rollout records the release in its version history.

## CRD Ownership

| CRD | Owner |
| --- | --- |
| `Rollout`, `RolloutGate`, `HealthCheck`, `RolloutSchedule`, `ClusterRolloutSchedule`, `ApprovalPolicy`, `ClusterApprovalPolicy`, `RolloutApproval` | rollout-controller |
| `ReleaseSource` | release-controller |
| `Environment` | environment-controller |
