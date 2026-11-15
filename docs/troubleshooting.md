# Troubleshooting

Common issues with running Kuberik and how to diagnose them.

## A rollout never promotes

Symptoms: `kuberik get rollouts` shows the Rollout but `status.latestRelease` stays empty or stale even though a newer image tag exists.

Check, in order:

1. **The ReleaseSource has found the tags.**
   ```bash
   kubectl get releasesource -n <namespace> <name> -o yaml
   ```
   `status.newest.tag` must be set and `Ready` must be `True`. If empty, release-controller has not seen any tag matching the source's policy. Check `spec.oci.image`, `spec.oci.policy` and your registry credentials (`spec.oci.secretRef`). If the source reads a Flux ImagePolicy through `imagePolicyRef`, check that ImagePolicy and its ImageRepository too.

2. **The Rollout points at the right ReleaseSource.**
   `spec.releaseSourceRef.name` must name a ReleaseSource in the Rollout's namespace. A Rollout without it reports `Ready=False` with reason `ReleaseSourceRequired`, and `ReleaseSourceReady` says which source is missing or not ready.

3. **No gate is blocking the rollout.**
   ```bash
   kuberik get gates -n <namespace>
   kubectl describe rollout <name> -n <namespace>
   ```
   The `Promoting` condition lists the blocking gate by name. If the gate belongs to an `ApprovalPolicy`, approve the version in the dashboard or create a `RolloutApproval` for it (see [Gates](gates.md#approvals)).

4. **No HealthCheck is unhealthy.**
   ```bash
   kuberik get healthchecks -n <namespace>
   ```
   A failing health check during bake will pause promotion. Look at the `message` field for the producing controller's reason.

5. **The target resource has the annotation.**
   The OCIRepository or Kustomization the Rollout patches must carry either:
   - `rollout.kuberik.com/rollout: "<rollout-name>"`, or
   - `rollout.kuberik.com/substitute.<VAR>.from: "<rollout-name>"` (on a Kustomization)
   Without the annotation, the controller has no target to patch.

## `kuberik check` says CRDs are missing

The install did not run, or it ran against a different cluster.

```bash
# Verify current context
kubectl config current-context

# Reapply
kuberik install
```

If `ReleaseSource` is not found, release-controller is not installed (see [installation](installation.md)). Flux's image-reflector-controller is only needed when a ReleaseSource reads an `ImagePolicy` through `imagePolicyRef`.

## A bake fails immediately

If a Rollout enters bake and fails within seconds with no obvious unhealthy HealthCheck:

- Inspect `lastErrorTime` on each HealthCheck referenced by the Rollout. **`lastErrorTime` survives recovery** - if a check went unhealthy at some point and then recovered, `lastErrorTime` is still set and the rollout-controller treats this as a witness of trouble during bake.
- To clear, the producing controller must update the HealthCheck status with a `lastErrorTime` older than the bake start, or you must wait for the bake start to advance past the recorded error.

This is intentional, not a bug. See [docs/healthchecks.md](healthchecks.md).

## Controller pod crashlooping

```bash
kubectl logs -n kuberik-system deploy/rollout-controller --tail=200
```

Common causes:

- **RBAC missing for a CRD.** Usually means an integration controller installed CRDs after the rollout-controller started but the controller's ServiceAccount cannot list them. Restart the rollout-controller pod.
- **Old install bundle.** If you mixed-and-matched per-component install URLs, the version pinning may be inconsistent. Reapply the full bundle:
  ```bash
  kuberik install --all
  ```

## A gate keeps re-blocking

If you flip a gate's `spec.passing` and it is reset right after, a controller owns the gate:

- gates of a `RolloutSchedule` or `ClusterRolloutSchedule` are reconciled back by the rollout-controller (label `gate.kuberik.com/schedule-name`);
- gates of a `RolloutDependency` likewise (label `gate.kuberik.com/dependency-name`).

Two other kinds of gate block without being reset:

- `ghd-*` gates belong to the environment-controller. They block through `spec.allowedVersions`, which the environment-controller sets to the versions the earlier environment (or the previous wave) has deployed; `passing` does not open them. Find their owner through the owner reference, not a label.
- Gates owned by an `ApprovalPolicy` or `ClusterApprovalPolicy`: when the rollout-controller's admission policies are installed, an edit to them is refused outright.

Check who owns a gate:

```bash
kubectl get rolloutgate -n <namespace> \
  -o 'custom-columns=NAME:.metadata.name,PASSING:.spec.passing,OWNER:.metadata.ownerReferences[*].kind'
kubectl get rolloutgate -n <namespace> --show-labels
```

Change the owning object, not the gate. To approve a version, create a `RolloutApproval`, not `passing`.

## Asking for help

- [GitHub Discussions](https://github.com/kuberik/kuberik/discussions) for usage questions.
- [GitHub Issues](https://github.com/kuberik/kuberik/issues) for reproducible bugs.

Include in your report: Kuberik CLI version, rollout-controller image version, Kubernetes version, and the YAML of the Rollout / Gate / HealthCheck you are seeing problems with.
