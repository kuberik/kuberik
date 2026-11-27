# Basic Rollout Example

A minimal end-to-end Kuberik setup: one image tracked by a ReleaseSource, one Rollout, one OCIRepository patched on promotion.

## Files

- `releasesource.yaml` - the `ReleaseSource` that lists the image's tags and orders them with a `policy` (reconciled by the release-controller)
- `rollout.yaml` - the Kuberik `Rollout` that drives promotion
- `oci-target.yaml` - the Flux `OCIRepository` that gets patched when a release is approved

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

## Next steps

- Add a manual approval gate - see [examples/with-gate](../with-gate)
- Add a Datadog health check - see [examples/with-datadog](../with-datadog)
