#!/usr/bin/env bash
# Pin the chart to a rollout-controller release: replaces the rollout-controller
# CRDs in chart/kuberik/files/crds with the ones from that release's install.yaml
# and sets appVersion. Run it only once the release that carries the Rollout
# releaseSourceRef field (rollout-controller #58) is published.
#   hack/bump-rollout-controller.sh v0.12.0
set -euo pipefail
ver="${1:?usage: $0 vX.Y.Z}"
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
curl -fsSL "https://github.com/kuberik/rollout-controller/releases/download/${ver}/install.yaml" -o "$tmp/install.yaml"
python3 - "$tmp/install.yaml" "$root/chart/kuberik/files/crds" <<'PY'
import re, sys
src, out = sys.argv[1:]
for doc in re.split(r"^---\s*$", open(src).read(), flags=re.M):
    if re.search(r"^kind: CustomResourceDefinition\s*$", doc, re.M):
        name = re.search(r"^  name: (\S+)", doc, re.M).group(1)
        open(f"{out}/{name}.yaml", "w").write(doc.lstrip("\n"))
        print("updated", name)
PY
sed -i "s/^appVersion: .*/appVersion: \"${ver#v}\"/" "$root/chart/kuberik/Chart.yaml"
sed -i -E "s#(rollout-controller/releases/download/)v[0-9.]+#\1${ver}#" "$root/config/install/kustomization.yaml"
echo "Next: bump Chart.yaml version, docs/upgrade.md matrix, restore the sample Rollout apply in .github/workflows/ci.yaml, helm lint + template."
