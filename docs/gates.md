# Rollout Gates

A `RolloutGate` is a boolean veto on a Rollout. While `spec.passing` is `false`, the referenced Rollout will not promote a new release. When all gates referencing a Rollout pass, the controller is free to promote.

Gates compose: a Rollout can be blocked by many gates, and a gate written by hand can be combined with gates created by controllers.

## Minimal example

A hand-written gate is a veto. This one stops all promotions of `my-app` while `passing` is `false`:

```yaml
apiVersion: kuberik.com/v1alpha1
kind: RolloutGate
metadata:
  name: my-app-killswitch
  namespace: production
spec:
  rolloutRef:
    name: my-app
  passing: false
```

Flip it back with `kubectl`:

```bash
kubectl patch rolloutgate my-app-killswitch -n production \
  --type merge -p '{"spec":{"passing":true}}'
```

Flipping `passing` is not an approval. It records nobody and no version. Approvals are `RolloutApproval` objects, one per version, and an `ApprovalPolicy` owns the gate that enforces them; see [Approvals](#approvals).

## Gate sources

Different controllers create gates for different reasons. The Rollout does not care who set the gate - all it sees is the boolean.

| Source | What it gates on |
| --- | --- |
| ApprovalPolicy | A person approves each version with a `RolloutApproval` |
| Hand-written | A human or bot flips `spec.passing`; use it as a kill switch |
| RolloutSchedule | Time-based: business hours, holiday freezes, maintenance windows |
| environment-controller | Promotion order: a tier gates on the nearest earlier tier (dev, test, staging, prod) having deployed and baked the version |
| openkruise-controller | Strategy progress: canary at 50% before allowing full rollout |
| Your custom controller | Any signal you can express as `passing: true/false` |

## Schedule-based gates

A `RolloutSchedule` creates and manages gates on a time-based rule set. Rollouts opt in by carrying a matching label.

```yaml
apiVersion: kuberik.com/v1alpha1
kind: RolloutSchedule
metadata:
  name: business-hours-only
  namespace: production
spec:
  rolloutSelector:
    matchLabels:
      kuberik.com/schedule: business-hours-only
  action: Allow
  timezone: "America/New_York"
  rules:
    - daysOfWeek: [Monday, Tuesday, Wednesday, Thursday, Friday]
      timeRange:
        start: "09:00"
        end: "17:00"
```

`action: Allow` means the gate is open during the window and closed outside it. `action: Deny` means the gate is closed during the window (useful for change-freeze windows). `action` defaults to `Deny`, so a schedule without it blocks promotions while a rule matches.

`ClusterRolloutSchedule` is the cluster-scoped variant: it adds a `namespaceSelector` so the same rules cover Rollouts in multiple namespaces.

## Approvals

An approval is a `RolloutApproval` for one Rollout and one version. Its `spec` is immutable and names the Rollout and the release tag. See [examples/with-gate](https://github.com/kuberik/kuberik/tree/main/examples/with-gate) for the policy and an approval. Creating one needs the `kuberik-rollout-approver` ClusterRole. A Rollout with no deployment history deploys its first release without approval.

## Inspecting gate state

```bash
kuberik get gates -A
```

A non-passing gate that holds up promotion will show on the Rollout's status conditions:

```bash
kubectl describe rollout my-app
```

The `Promoting` condition will list which gate is currently blocking.

## Patterns

### Manual approval

Select the Rollouts that need approval with an `ApprovalPolicy`. Each version then needs its own `RolloutApproval`, created in the dashboard or by hand. A policy can select Rollouts by label, so one policy covers every production Rollout in a namespace; `ClusterApprovalPolicy` covers namespaces by selector.

```yaml
apiVersion: kuberik.com/v1alpha1
kind: ApprovalPolicy
metadata:
  name: production-approval
  namespace: production
spec:
  rolloutSelector:
    matchLabels:
      kuberik.com/approval: required
```

### Pause everything

A namespace-wide kill switch: one gate everyone agrees to flip when something is on fire.

```yaml
apiVersion: kuberik.com/v1alpha1
kind: RolloutGate
metadata:
  name: kill-switch
  namespace: production
spec:
  rolloutRef:
    name: my-app
  passing: true  # default open; flip to false to stop all promotions
```

### Promotion order across environments

Use environment-controller rather than hand-rolling gates. Give each tier's Environment the same `spec.name` and its own `spec.environment` (`staging`, `production`); the controller creates the gates from that fixed tier order. The `relationship` field is ignored.
