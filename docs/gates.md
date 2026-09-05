# Rollout Gates

A `RolloutGate` is a boolean veto on a Rollout. While `spec.passing` is `false`, the referenced Rollout will not promote a new release. When all gates referencing a Rollout pass, the controller is free to promote.

Gates compose: a Rollout can be blocked by many gates, and a single gate set by hand can be combined with gates set automatically by integration controllers.

## Minimal example

```yaml
apiVersion: kuberik.com/v1alpha1
kind: RolloutGate
metadata:
  name: my-app-approval
  namespace: production
spec:
  rolloutRef:
    name: my-app
  passing: false
```

Flip it to passing with the CLI:

```bash
kuberik approve my-app-approval -n production
```

Or directly with `kubectl`:

```bash
kubectl patch rolloutgate my-app-approval -n production \
  --type merge -p '{"spec":{"passing":true}}'
```

## Gate sources

Different controllers create gates for different reasons. The Rollout does not care who set the gate - all it sees is the boolean.

| Source | What it gates on |
| --- | --- |
| Manual | A human (or ChatOps bot) flips `spec.passing` |
| RolloutSchedule | Time-based: business hours, holiday freezes, maintenance windows |
| environment-controller | Promotion order: production gates on staging being deployed (`After` relationship) |
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

`action: Allow` means the gate is open during the window and closed outside it. `action: Deny` means the gate is closed during the window (useful for change-freeze windows).

`ClusterRolloutSchedule` is the cluster-scoped variant: it adds a `namespaceSelector` so the same rules cover Rollouts in multiple namespaces.

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

### Manual approval per environment

One gate per environment, owned by the team that approves into that environment.

```yaml
# production approval
apiVersion: kuberik.com/v1alpha1
kind: RolloutGate
metadata:
  name: production-approval
  namespace: production
spec:
  rolloutRef:
    name: my-app
  passing: false
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

Use environment-controller relationships rather than hand-rolling gates. It will create the right gates for you based on the declared `After` graph between Environments.
