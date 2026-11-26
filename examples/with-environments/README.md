# Multi-Environment Promotion

Use `Environment` resources to model promotion order across environments. Production waits for staging to deploy and bake successfully before it considers a version.

## Prerequisites

- environment-controller is installed (`integrations.environment.enabled=true` in the Helm chart, or `kuberik install --all`).
- You have a Rollout named `my-app` in both the `staging` and `production` namespaces (see [examples/basic-rollout](../basic-rollout)).
- A `github-token` Secret in each Environment's namespace with a GitHub PAT that can write Deployments to `myorg/my-app`.
- The images carry the OCI annotation `org.opencontainers.image.revision` set to the git SHA. Promotion compares git revisions recorded in GitHub and maps them back to tags, so an image without a revision cannot be promoted.

## Apply

```bash
kubectl apply -f environments.yaml
```

## How the order is decided

Both Environments have `spec.name: my-app`. `spec.environment` is the tier, and tiers have a fixed order: `dev` < `test` < `staging` < `prod`. The aliases `development`, `qa`, `stage`, `preprod` and `production` are accepted and count as their tier.

The environment-controller:

1. Gates each Environment on the nearest earlier tier that exists in GitHub for the same `spec.name`. Tiers that are absent are skipped, so `production` is gated on `staging` here.
2. Writes a `RolloutGate` for the Rollout. Its `allowedVersions` holds the versions that completed the upstream tier.
3. Reports deployment events for each Environment to the GitHub Deployments API.

The old `spec.relationship` field is ignored. A manifest that sets it still applies, and the Environment reports `Deprecated=True`.

## Inspect

```bash
kuberik get gates -A
kubectl describe environment my-app-production -n production
```

## Adding tiers

Add another Environment with the same `spec.name` and the tier you want, for example `environment: dev` (no upstream) or `environment: test` (between dev and staging). No existing manifest changes.

## Several clusters or regions

To run one tier in several clusters, give each cluster's Environment a `site` and a `wave`:

```yaml
spec:
  name: my-app
  environment: production
  site: eu-west-1
  wave: 2
```

Waves of one environment roll out in order: wave N starts a version after every site of wave N-1 baked it. The bake time of a site is its Rollout's `bakeTime`. Clusters do not connect to each other; they coordinate through GitHub Deployments. Only one Environment may own a given name, tier and site.
