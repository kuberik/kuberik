# Basic Rollout Example

A minimal end-to-end Kuberik setup: one image tracked by a ReleaseSource, one Rollout, one OCIRepository patched on promotion.

## Files

- `releasesource.yaml` - the `ReleaseSource` that lists the image's tags and orders them with a `policy` (reconciled by the release-controller)
- `rollout.yaml` - the Kuberik `Rollout` that drives promotion
- `oci-target.yaml` - the Flux `OCIRepository` that gets patched when a release is approved

## Prerequisites

- rollout-controller and release-controller installed (the kuberik chart installs both by default).
- Flux v2.6 or later with source-controller and kustomize-controller. image-reflector-controller is optional.

## Apply

```bash
kubectl apply -f releasesource.yaml
kubectl apply -f rollout.yaml
kubectl apply -f oci-target.yaml
```

## Watch

```bash
kuberik get rollouts
kubectl describe rollout my-app
```

The kuberik chart installs the release-controller by default. When a new image tag matching the semver range appears in `ghcr.io/myorg/my-app`, the Rollout will pick it up, wait for any gates to pass, patch the OCIRepository tag, and bake for 10 minutes before recording the release in its version history.

A Rollout without `spec.releaseSourceRef` gets no releases and reports `Ready=False` with reason `ReleaseSourceRequired`.

## Already using a Flux ImagePolicy?

You do not need one. If you have one, point the ReleaseSource at it instead of repeating the image and policy. `image`, `secretRef`, `filterTags` and `policy` are then read from the ImagePolicy and its ImageRepository, and must not be set on the ReleaseSource:

```yaml
spec:
  oci:
    imagePolicyRef:
      name: my-app
```

## Next steps

- Add a manual approval gate - see [examples/with-gate](../with-gate)
- Add a Datadog health check - see [examples/with-datadog](../with-datadog)
