# Getting Started with Kuberik

This guide walks you through installing Kuberik and deploying your first Rollout.

## Prerequisites

- Kubernetes cluster (1.30+)
- [Flux v2.6 or later](https://fluxcd.io/docs/installation/) with source-controller and kustomize-controller. image-reflector-controller is optional.
- release-controller, which provides the `ReleaseSource` resource (see its [README](https://github.com/kuberik/release-controller) for the install)
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

### 1. Create a ReleaseSource

A `ReleaseSource` lists the tags of your image in the registry and orders them by a policy. release-controller does the scanning itself; no Flux ImageRepository or ImagePolicy is needed.

```yaml
apiVersion: kuberik.com/v1alpha1
kind: ReleaseSource
metadata:
  name: my-app
  namespace: default
spec:
  oci:
    image: ghcr.io/myorg/my-app
    policy:
      semver:
        range: '>=1.0.0'
```

If you already have a Flux `ImagePolicy`, `spec.oci.imagePolicyRef: {name: my-app}` reads the image and policy from it instead.

### 2. Create a Rollout

```yaml
apiVersion: kuberik.com/v1alpha1
kind: Rollout
metadata:
  name: my-app
  namespace: default
spec:
  releaseSourceRef:
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

The rollout controller will automatically detect new releases from the ReleaseSource and promote them through your pipeline.

## Require an approval

An `ApprovalPolicy` makes every matching Rollout wait for a person to approve each version. Approvals are per-version `RolloutApproval` objects, which the dashboard creates when someone presses Approve.

```yaml
apiVersion: kuberik.com/v1alpha1
kind: ApprovalPolicy
metadata:
  name: manual-approval
  namespace: default
spec:
  rolloutSelector:
    matchLabels:
      kuberik.com/approval: required
```

Label the Rollout to opt it in:

```bash
kubectl label rollout my-app kuberik.com/approval=required
```

To approve a version by hand, create the object (see [examples/with-gate](../examples/with-gate)):

```yaml
apiVersion: kuberik.com/v1alpha1
kind: RolloutApproval
metadata:
  name: my-app-1-0-1
  namespace: default
spec:
  rolloutRef:
    name: my-app
  version: "1.0.1"
```

The first deploy of a Rollout with no history is not held. Other gates (schedules, health, dependencies) are described in [Gates](gates.md).

## Next Steps

- [Architecture](architecture.md) - understand how the components fit together
- [Concepts](concepts.md) - deep-dive into Rollout, RolloutGate, HealthCheck, and more
- [Full documentation](https://kuberik.com/docs/)
