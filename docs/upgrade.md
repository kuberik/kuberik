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

## Moving Rollouts to ReleaseSources (chart 0.10.0)

Chart 0.10.0 installs the release-controller by default. A Rollout gets its releases from a `ReleaseSource` named in `spec.releaseSourceRef`; `spec.releasesImagePolicy` is ignored, and a Rollout without `releaseSourceRef` reports the condition `ReleaseSourceRequired` and receives no releases. This applies once the chart runs a rollout-controller release that has `releaseSourceRef`.

Migrate before or together with that upgrade:

1. Create one `ReleaseSource` per app, in the Rollout's namespace. To keep reading an existing Flux `ImagePolicy`, reference it:

   ```yaml
   apiVersion: kuberik.com/v1alpha1
   kind: ReleaseSource
   metadata:
     name: my-app
   spec:
     oci:
       imagePolicyRef:
         name: my-app
   ```

   Or describe the image directly with `oci.image` and `oci.policy` (see `examples/basic-rollout/releasesource.yaml`). `imagePolicyRef` cannot be combined with `policy` or `filterTags`.
2. Wait until the `ReleaseSource` is Ready.
3. Point each Rollout at it with `spec.releaseSourceRef: {name: my-app}`.
4. Keep the Flux `ImagePolicy` and `ImageRepository` objects until releases flow through the `ReleaseSource`; only then remove them, unless a source still references them through `imagePolicyRef`.

The rollout-controller no longer reads Flux `ImagePolicy` and `ImageRepository` objects, and its chart role no longer grants access to them; Rollouts read releases from a `ReleaseSource`.

## Chart -> controller version matrix

| Chart version | rollout-controller | release | datadog | openkruise | environment | dashboard |
| --- | --- | --- | --- | --- | --- | --- |
| 0.2.x | v0.7.0 | - | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.3.x | v0.7.0 | - | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.4.x | v0.7.0 | - | v0.1.0 | v0.3.3 | v0.1.5 | v0.7.7 |
| 0.5.0 | v0.8.0 | - | v0.1.0 | v0.4.0 | v0.1.5 | v0.8.0 |
| 0.5.1 | v0.8.0 | - | v0.1.0 | v0.4.0 | v0.1.5 | v0.8.2 |
| 0.6.0 | v0.9.0 | - | v0.1.0 | v0.4.0 | v0.1.5 | v0.9.1 |
| 0.6.1 | v0.9.0 | - | v0.1.0 | v0.4.0 | v0.1.5 | v0.9.1 |
| 0.7.0 | v0.9.2 | - | v0.1.0 | v0.4.0 | v0.1.5 | v0.9.1 |
| 0.10.0 | RELEASE-PLACEHOLDER | RELEASE-PLACEHOLDER | RELEASE-PLACEHOLDER | RELEASE-PLACEHOLDER | RELEASE-PLACEHOLDER | RELEASE-PLACEHOLDER |

0.6.0 shipped rollout-controller v0.9.0 without the `RolloutDependency` CRD and RBAC; use 0.6.1 or newer. 0.7.0 turns the CRDs into chart resources; coming from 0.6.1 or older needs the one-time hand-over above. 0.10.0 is the next release after 0.7.0 (0.8.x and 0.9.x were never published); the versions in its row are filled in when it is released.

## Upgrading to chart 0.10.0

**BREAKING: Flux Kustomization variables written by Kuberik.** For a Rollout that delivers through a Flux Kustomization, rollout-controller now writes the deployed version into `spec.postBuild.substitute` under its own per-Rollout field manager and no longer forces ownership of the field. If the Kustomization you apply from Git also sets that variable (for example `IMAGE_TAG: "1.0.0"` under `postBuild.substitute`), the next deploy is refused and the Rollout reports `KustomizationFieldConflict`. Remove the variable from the Kustomization in Git and give the manifests a default instead, `${IMAGE_TAG:=1.0.0}`. Kuberik fills the variable in on the first deploy and Flux keeps your default until then.

**BREAKING: OCIRepository tags written by Kuberik.** For a Rollout whose source is a Flux OCIRepository, rollout-controller writes only `spec.ref.tag`, under the same per-Rollout field manager, and no longer forces ownership of it. If the OCIRepository you apply from Git also sets `spec.ref.tag`, the next deploy is refused and the Rollout reports `OCIRepositoryFieldConflict`. Remove `spec.ref.tag` from the OCIRepository in Git.

**BREAKING: Environment order comes from the tier name.** The environment-controller orders Environments by `spec.environment`, which must be one of `dev`, `development`, `test`, `qa`, `staging`, `stage`, `preprod`, `prod` or `production`. Any other name closes that Environment's gate (`WaveReady=False`, reason `UnknownTier`), and two names of one tier used as a chain (for example `staging` then `preprod`) block with `InvalidTopology`. Rename such Environments and use `spec.site` and `spec.wave` to split a tier. `spec.relationship` is accepted and ignored (`Deprecated=True`, reason `RelationshipIgnored`); remove it when convenient. Upgrade every cluster that reports to the same GitHub repository, and leave `spec.site` unset on existing Environments until each has reconciled once after the upgrade, so their deployment history carries over as site `default`.

**New RBAC.** The rollout-controller may `get`, `list`, `watch` and `patch` `helmreleases.helm.toolkit.fluxcd.io`, so Rollouts can deliver through a Flux HelmRelease. The chart grants it. If you install the controller from its install manifest instead of the chart, apply the manifest of the matching release. The chart also adds the `kuberik-rollout-approver` ClusterRole, the release catalogue read role for the release-controller, and the approval ValidatingAdmissionPolicies (Kubernetes 1.30+, `admissionPolicies.enabled`).

**New CRDs.** `ApprovalPolicy`, `ClusterApprovalPolicy` and `RolloutApproval` (manual approvals), and `ReleaseSource` and `ReleaseSlice` (release-controller). Helm installs them during `helm upgrade`; they carry `helm.sh/resource-policy: keep`. New fields on `Rollout` and `HealthCheck` (automatic rollback, HelmRelease delivery, health observations) arrive with the same upgrade.

**openkruise-controller.** The chart now defaults to an openkruise-controller release (see the matrix row) that clears stale conditions when a `RolloutTest` moves to `WaitingForStep`. Upgrade it together with the rest of the chart.

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
