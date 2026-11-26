# Concepts

## Rollout

A `Rollout` defines a release pipeline for one workload. It references a `ReleaseSource` (`spec.releaseSourceRef`) to discover new releases and specifies how to promote them: which Flux resources to patch, how long to observe health (bake time), and which gates must pass before proceeding. The rollout-controller reconciles the `Rollout` continuously, advancing through versions as conditions are met.

## RolloutGate

A `RolloutGate` is a named boolean condition scoped to a namespace. When `spec.passing` is `false`, any `Rollout` that references this gate will not advance to a new release. Gates are created automatically by integration controllers (e.g., environment-controller) or by a `RolloutSchedule` or `ApprovalPolicy`, or written by hand as a kill switch. A hand-flipped `passing` is not an approval; see below. Multiple gates can block a single rollout, and all must pass before promotion proceeds.

## ReleaseSource

A `ReleaseSource` (managed by release-controller) lists the tags of an OCI image in the registry, orders them by a policy such as a semver range, and keeps the catalogue of releases a `Rollout` reads. It scans the registry itself. `spec.oci.imagePolicyRef` can instead read the image and policy from an existing Flux `ImagePolicy`, but that is optional.

## ApprovalPolicy / RolloutApproval

An `ApprovalPolicy` (or cluster-wide `ClusterApprovalPolicy`) selects Rollouts that need a person to approve each version, and owns the gate that enforces it. An approval is a `RolloutApproval` for one Rollout and one version; the dashboard creates them. A Rollout with no deployment history deploys its first release without approval.

## HealthCheck

A `HealthCheck` records the observed health state of a workload component. It is typically created and updated by an integration controller (datadog-controller, prometheus-controller) rather than by hand. The rollout-controller reads `HealthCheck` status during the bake period to determine whether a deployment is stable. An unhealthy `HealthCheck` during bake time can pause or block the current release.

## Environment

An `Environment` (managed by environment-controller) represents a logical deployment target - for example, `staging` or `production`. It links a `Rollout` to a backend like the GitHub Deployments API, so deployment events are recorded in GitHub's environment tracking UI. Environments of one app share `spec.name` and differ by `spec.environment`, a tier with a fixed order (dev, test, staging, prod); a tier is gated on the nearest earlier tier. The `relationship` field is ignored. `site` and `wave` place an Environment in a multi-cluster rollout.

## RolloutSchedule / ClusterRolloutSchedule

A `RolloutSchedule` creates and manages `RolloutGate` resources on a time-based schedule. The `action` field controls the direction: `Allow` means gates pass only during the schedule window (block outside it), `Deny` means gates are blocked during the window (allow outside it). `ClusterRolloutSchedule` applies across multiple namespaces via a `namespaceSelector`. Common uses: business-hours-only deployments, holiday freezes, maintenance windows.
