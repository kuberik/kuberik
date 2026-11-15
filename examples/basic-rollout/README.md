# Basic Rollout Example

A minimal end-to-end Kuberik setup: one image listed by a `ReleaseSource`, one Rollout, one OCIRepository patched on promotion.

## Files

- `release-source.yaml` - the `ReleaseSource` (release-controller) that lists the image's tags from the registry
- `rollout.yaml` - the Kuberik `Rollout` that reads the ReleaseSource and drives promotion
- `oci-target.yaml` - the Flux `OCIRepository` that gets patched when a release is promoted

## Prerequisites

- rollout-controller and release-controller installed (release-controller has its own install, see its README).
- Flux v2.6 or later with source-controller and kustomize-controller. image-reflector-controller is optional.

## Apply

```bash
kubectl apply -f release-source.yaml
kubectl apply -f rollout.yaml
kubectl apply -f oci-target.yaml
```

## Watch

```bash
kuberik get rollouts
kubectl describe rollout my-app
```

When a new image tag matching the semver range appears in `ghcr.io/myorg/my-app`, the ReleaseSource adds it to its catalogue, the Rollout picks it up, waits for any gates to pass, patches the OCIRepository tag, and bakes for 10 minutes before recording the release in its version history.

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

- Add a manual approval - see [examples/with-gate](../with-gate)
- Add a Datadog health check - see [examples/with-datadog](../with-datadog)
