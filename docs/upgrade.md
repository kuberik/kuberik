# Upgrading

How to upgrade Kuberik components safely. Each Kuberik controller is versioned independently; this page covers the recommended upgrade order and known compatibility constraints.

## Upgrade order

1. **CRDs** (always first). Apply the latest CRD manifests before bumping controller images. CRD changes are additive across minor versions of `0.x` - new optional fields, new printer columns, no required field renames - so applying a newer CRD against an older controller is safe.
2. **rollout-controller** (the core). Bump this before integration controllers; they declare API compatibility against the version pinned in the rollout-controller release.
3. **Integration controllers** (datadog, prometheus, openkruise, environment). Bump these any time after the core. Multiple at once is fine.
4. **rollout-dashboard**. Always last - it consumes the controller APIs and benefits from updated CRDs and controllers.

When upgrading with `kuberik install`, the CLI applies the bundled install manifest which already pins compatible versions of each component.

## Helm chart upgrades

For Helm-managed installs:

```bash
helm repo update kuberik
helm upgrade kuberik kuberik/kuberik -n kuberik-system
```

CRDs are stored in `crds/` and **are not upgraded by `helm upgrade`**. This is a Helm convention (CRD upgrades can be lossy if defaults change). When the chart bumps controller versions in a way that requires new CRD fields, you will see a note in the release notes. Apply CRDs manually:

```bash
helm pull kuberik/kuberik --version <new-version> --untar
kubectl apply -f kuberik/crds/
```

## Chart -> controller version matrix

| Chart version | rollout-controller | datadog | openkruise | environment | dashboard |
| --- | --- | --- | --- | --- | --- |
| 0.2.x | v0.7.0 | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.3.x | v0.7.0 | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.4.x | v0.7.0 | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.5.0 | v0.8.0 | v0.1.0 | v0.4.0 | v0.1.5 | v0.8.0 |
| 0.5.1 | v0.8.0 | v0.1.0 | v0.4.0 | v0.1.5 | v0.8.2 |

Always check the release notes of the chart version you are upgrading to for any deviations from this baseline.

## Breaking-change discipline

Pre-1.0, breaking changes are possible between minor versions but they will:

- Be called out in the relevant component's release notes with `BREAKING:` prefix.
- Land with a migration script or a `kuberik` CLI subcommand to perform the migration.
- Be supported in parallel for at least one minor version (the old API will still reconcile while you migrate).

If your environment cannot tolerate breaking changes, pin to patch versions explicitly in your install manifest / Helm values.

## Rolling back

To roll back to a prior chart release:

```bash
helm history kuberik -n kuberik-system
helm rollback kuberik <revision> -n kuberik-system
```

Helm rollback restores the previous chart artifact - including controller images - but **does not** roll back CRDs. CRDs added in the newer release stay; CRD fields added in the newer release stay defined but become harmless if no controller reads them.

To roll back via the install manifest, re-apply an older release:

```bash
kubectl apply -f https://github.com/kuberik/rollout-controller/releases/download/<old-version>/install.yaml
```

Existing `Rollout`, `RolloutGate`, and `HealthCheck` resources are preserved across rollbacks because they live in CRDs the controller does not own.
