# Installation

Kuberik is a collection of independently versioned Kubernetes controllers. You can install them in any combination - only the **core rollout-controller** is required. The integration controllers are optional and pull in their own CRDs.

## Quick start

Install everything (recommended for getting started):

```bash
kubectl apply -k https://github.com/kuberik/kuberik/config/install
```

This applies a [kustomize bundle](../config/install/kustomization.yaml) that pins compatible versions of:

- rollout-controller (core, required)
- openkruise-controller
- datadog-controller
- environment-controller

## Components

| Component | Install URL | Required? |
| --- | --- | --- |
| rollout-controller | `https://github.com/kuberik/rollout-controller/releases/latest/download/install.yaml` | yes |
| rollout-dashboard | `https://github.com/kuberik/rollout-dashboard/releases/latest/download/install.yaml` | no |
| datadog-controller | `https://github.com/kuberik/datadog-controller/releases/latest/download/install.yaml` | no |
| prometheus-controller | `https://github.com/kuberik/prometheus-controller/releases/latest/download/install.yaml` | no |
| openkruise-controller | `https://github.com/kuberik/openkruise-controller/releases/latest/download/install.yaml` | no |
| environment-controller | `https://github.com/kuberik/environment-controller/releases/latest/download/install.yaml` | no |

Apply only what you need:

```bash
# Core only
kubectl apply -f https://github.com/kuberik/rollout-controller/releases/latest/download/install.yaml

# Add Prometheus integration
kubectl apply -f https://github.com/kuberik/prometheus-controller/releases/latest/download/install.yaml
```

## Prerequisites

- Kubernetes 1.25 or newer
- [Flux v2](https://fluxcd.io/docs/installation/) with the `image-reflector-controller` component (required by rollout-controller for ImagePolicy tracking)

## Installing the CLI

See [docs/cli.md](cli.md) for CLI installation. The CLI is optional - everything Kuberik does is reachable through `kubectl` - but it streamlines common operations.

## Verifying

After install:

```bash
kuberik check
# or
kubectl get crd | grep kuberik.com
kubectl get pods -n kuberik-system
```

You should see:

- the Kuberik CRDs (`rollouts.kuberik.com`, `rolloutgates.kuberik.com`, `healthchecks.kuberik.com`, `rolloutschedules.kuberik.com`)
- a running rollout-controller pod in the `kuberik-system` namespace

## Uninstalling

```bash
kuberik uninstall
# or
kubectl delete -k https://github.com/kuberik/kuberik/config/install
```

Deleting the CRDs removes all `Rollout`, `RolloutGate`, and `HealthCheck` resources in the cluster. Back up these resources first if you need to preserve them.

## Pinning versions

The kustomize bundle in `config/install/kustomization.yaml` pins specific component versions. Fork or vendor this file if you need to pin a specific combination, or apply individual component releases directly.

## Air-gapped / private registries

Each component publishes container images to `ghcr.io/kuberik/`. Mirror these images to your private registry and patch the install manifests with your registry prefix. The kustomize bundle is designed to make this easy via a top-level `images:` patch.
