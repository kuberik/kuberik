#!/usr/bin/env bash
# Render-level checks for the chart. Needs only the helm binary.
#
# `helm template --kube-version` sets only .Capabilities.KubeVersion; it does
# not add API groups to .Capabilities.APIVersions. The admission template
# gates on APIVersions.Has, so the tests pass --api-versions explicitly to
# simulate a 1.30+ cluster, and omit it to simulate an older one.
set -euo pipefail
CHART="$(cd "$(dirname "$0")/.." && pwd)"
VAP=admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy
fail=0
t() { helm template kuberik "$CHART" "$@"; }
check() { # name, condition-exit-code
  if [ "$2" -eq 0 ]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}
has() { grep -q -- "$2" <<<"$1"; }
count() { grep -cE "$1" || true; }

out=$(t --kube-version 1.29.0)
check "1.29 renders no admission objects" "$([ "$(echo "$out" | count '^kind: ValidatingAdmissionPolic')" = 0 ] && echo 0 || echo 1)"

out=$(t --kube-version 1.30.0 --api-versions "$VAP")
check "1.30 renders 2 policies" "$([ "$(echo "$out" | count '^kind: ValidatingAdmissionPolicy$')" = 2 ] && echo 0 || echo 1)"
check "1.30 renders 2 bindings" "$([ "$(echo "$out" | count '^kind: ValidatingAdmissionPolicyBinding$')" = 2 ] && echo 0 || echo 1)"
for n in kuberik-rolloutapproval-by kuberik-approval-gate-guard; do
  check "policy $n present" "$(has "$out" "^  name: $n$"; echo $?)"
done
check "guard allows controller SA in kuberik-system" "$(has "$out" "system:serviceaccount:kuberik-system:rollout-controller-controller-manager"; echo $?)"

out=$(t --kube-version 1.30.0 --api-versions "$VAP" --set namespace=custom-ns)
check "guard namespace is templated" "$(has "$out" "system:serviceaccount:custom-ns:rollout-controller-controller-manager"; echo $?)"
check "no stale kuberik-system in guard" "$([ "$(echo "$out" | count 'serviceaccount:kuberik-system:')" = 0 ] && echo 0 || echo 1)"

out=$(t --kube-version 1.30.0 --api-versions "$VAP" --set admissionPolicies.enabled=false)
check "enabled=false renders none" "$([ "$(echo "$out" | count '^kind: ValidatingAdmissionPolic')" = 0 ] && echo 0 || echo 1)"

out=$(t --api-versions "$VAP" --set dashboard.enabled=true)
check "dashboard may get the by binding" "$(has "$out" "kuberik-rolloutapproval-by$"; echo $?)"
check "approver role rendered, not aggregated" "$(echo "$out" | awk '/name: kuberik-rollout-approver/{f=1} f&&/aggregate/{bad=1} /^---/{f=0} END{exit bad}' ; echo $?)"
check "approver role rendered" "$(has "$out" "name: kuberik-rollout-approver"; echo $?)"

out=$(t)
for k in ApprovalPolicy ClusterApprovalPolicy RolloutApproval; do
  check "CRD $k rendered" "$(has "$out" "^    kind: $k$"; echo $?)"
done
out=$(t --set crds.install=false)
check "crds.install=false renders no CRDs" "$([ "$(echo "$out" | count '^kind: CustomResourceDefinition')" = 0 ] && echo 0 || echo 1)"

helm lint "$CHART" >/dev/null
check "helm lint" $?
exit $fail
