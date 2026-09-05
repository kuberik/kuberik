# Migrating to Kuberik

How Kuberik maps to concepts from the other tools you may already be using.

## From Argo Rollouts

Argo Rollouts owns the deployment strategy (canary, blue-green) and traffic shifting. Kuberik owns promotion control: when a new image is allowed to enter the deployment, under what conditions, and how to react during bake.

These two tools are not direct replacements - many teams run both. A typical layout:

| Argo Rollouts concept | Kuberik analog |
| --- | --- |
| `Rollout` (with strategy.canary/.blueGreen) | Continue using - drives traffic shifting and pod orchestration |
| `AnalysisTemplate` | `HealthCheck` produced by an integration controller |
| `AnalysisRun` | `HealthCheck.status` during the bake window |
| Promote via `kubectl-argo-rollouts promote` | `kuberik approve <gate>` |
| Pause/Resume | `kuberik suspend` / `kuberik resume` |
| Metric provider (Datadog, Prometheus, Web, Job) | datadog-controller / prometheus-controller / your own |

Kuberik does not implement traffic-shifting itself. If you already use Argo Rollouts for canary traffic management, keep it - Kuberik provides the higher-level "should this version even start to roll out" decision.

## From Flagger

Flagger automates progressive delivery on a single workload: it shifts traffic, runs metric analysis, and rolls back on failure. Kuberik separates these concerns into independent controllers and CRDs:

| Flagger concept | Kuberik analog |
| --- | --- |
| `Canary` resource | Combination of `Rollout` + `HealthCheck` + strategy (e.g. OpenKruise) |
| Webhook checks | `RolloutGate` produced by a custom controller |
| Metric template (PromQL / Datadog) | `HealthCheck` from prometheus-controller / datadog-controller |
| Pre/post-rollout hooks | `RolloutGate` or `HealthCheck` that produces the signal |
| Manual gating | `RolloutGate` with `spec.passing: false` |

Flagger is monolithic; Kuberik decomposes into Rollout + Gate + HealthCheck so teams can mix-and-match producers (e.g. one team's Datadog monitor and another team's manual approval gate the same Rollout).

## From "we manually edit YAML and run kubectl apply"

The minimum migration:

1. Install Kuberik + Flux image-reflector-controller.
2. For each application, add an `ImageRepository` + `ImagePolicy` pointing at the image registry. (You may already have these if you use Flux for image automation.)
3. Add a `Rollout` referencing the `ImagePolicy`.
4. Annotate your existing Flux `OCIRepository` / `Kustomization` with `rollout.kuberik.com/rollout: "<rollout-name>"` so the controller knows which resource to patch.
5. Optionally add gates and health checks.

The controller takes over auto-promotion from the moment the Rollout exists. Existing deployments are unaffected until a new image tag matches the ImagePolicy's range.

## From "we have a CI pipeline that runs kubectl apply"

You can keep the pipeline. Kuberik does not require you to remove other automation:

- The CI pipeline can still set initial image tags via Kustomize overlays or values files.
- Kuberik takes over once the Flux source is reconciled and a new image policy match is seen.
- For change-freeze windows or release-train cadences, add a `RolloutSchedule` instead of gating the CI pipeline.

A common pattern: CI pushes manifests to a git repo, Flux applies them, Kuberik controls when each version is promoted from one environment to the next.
