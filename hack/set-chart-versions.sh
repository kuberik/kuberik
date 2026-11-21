#!/usr/bin/env bash
# Release-time step: pin the chart to the component releases being published.
#
#   hack/set-chart-versions.sh --rollout-controller v0.12.0 --release-controller v0.1.0 \
#     --dashboard v0.10.0 --environment-controller v0.2.0 --datadog-controller v0.2.0 \
#     --openkruise-controller v0.4.2
#   hack/set-chart-versions.sh --check      # fail while any RELEASE-PLACEHOLDER remains
#
# Between releases every component version in the chart is the literal
# RELEASE-PLACEHOLDER (Chart.yaml appVersion and chart/kuberik/templates/_versions.tpl).
# This script replaces them all, runs hack/bump-rollout-controller.sh for the
# rollout-controller (CRDs, appVersion, config/install pin), pins the other
# controllers in config/install/kustomization.yaml, then runs --check.
# The chart-release workflow runs --check and refuses to publish on failure.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
PH=RELEASE-PLACEHOLDER
chart="$root/chart/kuberik"

check() {
  local bad
  bad="$(grep -nE "^appVersion: \"$PH\"|-\}\}$PH\{\{-" "$chart/Chart.yaml" "$chart/templates/_versions.tpl" || true)"
  if [ -n "$bad" ]; then
    echo "ERROR: $PH remains; run hack/set-chart-versions.sh with every component tag:" >&2
    echo "$bad" >&2
    exit 1
  fi
  echo "ok: no $PH in Chart.yaml appVersion or templates/_versions.tpl"
}

if [ "${1:-}" = "--check" ]; then check; exit 0; fi

declare -A V=()
while [ $# -gt 0 ]; do
  case "$1" in
    --rollout-controller|--release-controller|--dashboard|--environment-controller|--datadog-controller|--openkruise-controller)
      V["${1#--}"]="${2:?missing value for $1}"; shift 2 ;;
    *) echo "unknown argument $1" >&2; exit 2 ;;
  esac
done
for k in rollout-controller release-controller dashboard environment-controller datadog-controller openkruise-controller; do
  v="${V[$k]:-}"
  [[ "$v" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$ ]] || { echo "--$k needs a tag like v1.2.3 (got '${v}')" >&2; exit 2; }
done

"$root/hack/bump-rollout-controller.sh" "${V[rollout-controller]}"

tpl="$chart/templates/_versions.tpl"
set_tpl() { # define-name tag
  sed -i -E "s#(define \"kuberik\.version\.$1\" -}})[^{]*(\{\{- end -\}\})#\1$2\2#" "$tpl"
}
set_tpl releaseController "${V[release-controller]}"
set_tpl dashboard "${V[dashboard]}"
set_tpl environmentController "${V[environment-controller]}"
set_tpl datadogController "${V[datadog-controller]}"
set_tpl openkruiseController "${V[openkruise-controller]}"

kz="$root/config/install/kustomization.yaml"
for c in openkruise-controller datadog-controller environment-controller; do
  sed -i -E "s#(kuberik/$c/releases/download/)v[0-9][^/]*#\1${V[$c]}#" "$kz"
done
# release-controller: uncomment/refresh the pin once its install.yaml exists
sed -i -E "s#^(\s*)\# - (https://github.com/kuberik/release-controller/releases/download/)[^/]+(/install.yaml)#\1- \2${V[release-controller]}\3#; s#(kuberik/release-controller/releases/download/)v[0-9][^/]*#\1${V[release-controller]}#" "$kz"

check
echo "Pinned. Review git diff, then run QA against the published images."
