# Upgrading

How to upgrade Kuberik components safely. Each Kuberik controller is versioned independently; this page covers the recommended upgrade order and known compatibility constraints.

## Upgrade order

1. **CRDs** (always first). Apply the latest CRD manifests before bumping controller images; a controller does not start while a CRD it watches is missing. The Helm chart (0.7.0+) does this as part of `helm upgrade`. CRD changes are additive across minor versions of `0.x` - new optional fields, new printer columns, no required field renames - so applying a newer CRD against an older controller is safe.
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

Since chart 0.7.0 the CRDs are rendered as chart resources, so `helm upgrade` installs new CRDs and updates existing ones. They carry `helm.sh/resource-policy: keep`, so `helm uninstall` and a chart that stops shipping a CRD leave it in place. Charts up to 0.6.1 shipped CRDs in `crds/`, which Helm creates on install and never touches on upgrade. Two consequences:

**Upgrading from ≤ 0.6.1 to ≥ 0.7.0.** Hand the existing CRDs over to the release once, otherwise Helm refuses with `invalid ownership metadata`:

```bash
kubectl get crd -o name | grep -E 'kuberik\.com$' | xargs -I{} kubectl label --overwrite {} app.kubernetes.io/managed-by=Helm
kubectl get crd -o name | grep -E 'kuberik\.com$' | xargs -I{} kubectl annotate --overwrite {} meta.helm.sh/release-name=kuberik meta.helm.sh/release-namespace=kuberik-system
```

Use your own release name and namespace. Helm 3.17+ can do the same with `helm upgrade --take-ownership`.

**Staying on ≤ 0.6.1.** New CRDs must be applied by hand. Chart 0.6.x runs rollout-controller v0.9, which needs `RolloutDependency`; without it the controller does not start.

```bash
helm pull kuberik/kuberik --version <version> --untar
kubectl apply --server-side -f kuberik/crds/
```

`--server-side` is required: the `RolloutTest` CRD is larger than the 256 KiB annotation limit of client-side `kubectl apply`.

## Chart -> controller version matrix

| Chart version | rollout-controller | datadog | openkruise | environment | dashboard |
| --- | --- | --- | --- | --- | --- |
| 0.2.x | v0.7.0 | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.3.x | v0.7.0 | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.4.x | v0.7.0 | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.5.0 | v0.8.0 | v0.1.0 | v0.4.0 | v0.1.5 | v0.8.0 |
| 0.5.1 | v0.8.0 | v0.1.0 | v0.4.0 | v0.1.5 | v0.8.2 |
| 0.6.0 | v0.9.0 | v0.1.0 | v0.4.0 | v0.1.5 | v0.9.1 |
| 0.6.1 | v0.9.0 | v0.1.0 | v0.4.0 | v0.1.5 | v0.9.1 |
| 0.7.0 | v0.9.2 | v0.1.0 | v0.4.0 | v0.1.5 | v0.9.1 |

0.6.0 shipped rollout-controller v0.9.0 without the `RolloutDependency` CRD and RBAC; use 0.6.1 or newer. 0.7.0 turns the CRDs into chart resources; coming from 0.6.1 or older needs the one-time hand-over above.

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

Helm rollback restores the previous chart artifact, including controller images. Since chart 0.7.0 that includes the CRD schemas: fields the newer release added disappear from the schema and are pruned from objects on their next write, which is safe under the additive-only rule above; a CRD the newer release added stays because of `helm.sh/resource-policy: keep`. Charts up to 0.6.1 never touch CRDs on rollback.

To roll back via the install manifest, re-apply an older release:

```bash
kubectl apply -f https://github.com/kuberik/rollout-controller/releases/download/<old-version>/install.yaml
```

Existing `Rollout`, `RolloutGate`, and `HealthCheck` resources are preserved across rollbacks because they live in CRDs the controller does not own.
