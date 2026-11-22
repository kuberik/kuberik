# Getting Started with Kuberik

This guide walks you through installing Kuberik and deploying your first Rollout.

## Prerequisites

- Kubernetes cluster (1.25+)
- [Flux v2](https://fluxcd.io/docs/installation/) installed with the image-reflector-controller component
- `kubectl` configured to access your cluster

## Install

Install the core rollout-controller and its CRDs:

```bash
kubectl apply -f https://github.com/kuberik/rollout-controller/releases/latest/download/install.yaml
```

To install with all optional integrations at once:

```bash
kubectl apply -k https://github.com/kuberik/kuberik/config/install
```

Verify the controller is running:

```bash
kubectl get pods -n kuberik-system
```

## Deploy Your First Rollout

### 1. Set up Flux image tracking

Kuberik uses Flux's `ImageRepository` and `ImagePolicy` to discover new releases. If you already have these configured, skip to step 2.

```yaml
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: my-app
  namespace: flux-system
spec:
  image: ghcr.io/myorg/my-app
  interval: 1m
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: my-app
  namespace: flux-system
spec:
  imageRepositoryRef:
    name: my-app
  policy:
    semver:
      range: '>=1.0.0'
```

### 2. Create a Rollout

```yaml
apiVersion: kuberik.com/v1alpha1
kind: Rollout
metadata:
  name: my-app
  namespace: default
spec:
  releasesImagePolicy:
    name: my-app
  versionHistoryLimit: 10
  bakeTime: "10m"
```

Apply it:

```bash
kubectl apply -f rollout.yaml
```

### 3. Annotate your Flux resources

Tell Kuberik which Flux resources to update when a new release is ready.

For an `OCIRepository`:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: my-app
  annotations:
    rollout.kuberik.com/rollout: "my-app"
spec:
  url: oci://ghcr.io/myorg/my-app
```

Leave `spec.ref` out. The Rollout writes `spec.ref.tag` at its first deploy, and if Git sets it the deploy is refused with `OCIRepositoryFieldConflict`. Until that first deploy Flux fetches the `latest` tag, so the OCIRepository may report not ready if the artifact has none; that is expected.

For a `Kustomization` with substitution:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-app
  annotations:
    rollout.kuberik.com/substitute.IMAGE_TAG.from: "my-app"
spec:
  postBuild: {}
```

Do not set `IMAGE_TAG` under `postBuild.substitute` in Git: Kuberik writes that variable itself, and a value you also set there makes the next deploy fail with `KustomizationFieldConflict`. Give the manifests a default instead, which Flux uses until the first deploy:

```yaml
image: ghcr.io/myorg/my-app:${IMAGE_TAG:=1.0.0}
```

### 4. Check rollout status

```bash
kubectl get rollouts
kubectl describe rollout my-app
```

The rollout controller will automatically detect new releases from the ImagePolicy and promote them through your pipeline.

## Add a Deployment Gate

Gates block a rollout from proceeding until a condition is met. Create a manual approval gate:

```yaml
apiVersion: kuberik.com/v1alpha1
kind: RolloutGate
metadata:
  name: my-app-approval
  namespace: default
spec:
  rolloutRef:
    name: my-app
  passing: false
```

Set `passing: true` when you are ready to approve:

```bash
kubectl patch rolloutgate my-app-approval -p '{"spec":{"passing":true}}'
```

## Next Steps

- [Architecture](architecture.md) - understand how the components fit together
- [Concepts](concepts.md) - deep-dive into Rollout, RolloutGate, HealthCheck, and more
- [Full documentation](https://kuberik.com/docs/)
