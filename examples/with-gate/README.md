# Rollout with Manual Approval

Builds on [examples/basic-rollout](../basic-rollout). An `ApprovalPolicy` selects Rollouts that need a person to approve each version. An approval is a `RolloutApproval` object for one Rollout and one version.

## Apply

```bash
# Apply the base rollout first
kubectl apply -f ../basic-rollout/

# Opt the Rollout in to the policy, then add the policy
kubectl label rollout my-app kuberik.com/approval=required
kubectl apply -f approval-policy.yaml
```

The policy creates and owns a `RolloutGate` for each matching Rollout. A version that has no `RolloutApproval` is not allowed through that gate.

A Rollout with no deployment history deploys its newest release even when no gate allows it, so the first deploy is not held. Approvals apply from the second deploy on.

## Approve a version

Approve in the dashboard, or create the object yourself:

```bash
kubectl apply -f approval.yaml
```

Creating a `RolloutApproval` needs the `kuberik-rollout-approver` ClusterRole, which is not part of `edit` or `admin`. Bind it to the people who may approve, for example:

```bash
kubectl create rolebinding my-app-approvers -n default \
  --clusterrole=kuberik-rollout-approver --group=my-app-oncall
```

Once version `1.0.1` is approved, the controller patches the target OCIRepository and starts the bake period. Each later version needs its own approval.

## Old manual gates

A plain `RolloutGate` with `spec.passing: false` still works as a veto, but it is not an approval: it records nobody and no version. See [docs/gates.md](../../docs/gates.md).
