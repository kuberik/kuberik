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
# CI wiring that waits for this bump: the pinned CRD now has releaseSourceRef,
# so apply the sample/example Rollouts, and the kind jobs run the
# release-controller (its image must be published by the time this runs).
python3 - "$root/.github/workflows/ci.yaml" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
s = re.sub(r"[ ]*# releaseController\.enabled=false until[^\n]*\n[ ]*# publishes[^\n]*\n", "", s)
s = re.sub(r"[ ]*--set releaseController\.enabled=false \\\n", "", s)
s = re.sub(r"[ ]*# samples/rollout\.yaml uses spec\.releaseSourceRef[^\n]*\n[ ]*# pinned[^\n]*\n[ ]*# it again[^\n]*\n",
           "", s)
s = s.replace("kubectl apply -f chart/kuberik/samples/releasesource.yaml\n",
              "kubectl apply -f chart/kuberik/samples/releasesource.yaml\n          kubectl apply -f chart/kuberik/samples/rollout.yaml\n")
s = re.sub(r"[ ]*# basic-rollout/rollout\.yaml needs[^\n]*\n[ ]*# pinned[^\n]*\n", "", s)
s = s.replace("kubectl apply -f examples/basic-rollout/releasesource.yaml\n",
              "kubectl apply -f examples/basic-rollout/releasesource.yaml\n          kubectl apply -f examples/basic-rollout/rollout.yaml\n")
s = s.replace("get releasesource,rolloutgate", "get releasesource,rollout,rolloutgate")
s = s.replace("grep -E 'business-hours", "grep -E 'my-app|business-hours")
open(p, "w").write(s)
PY
echo "Done. Still manual: bump Chart.yaml version, add the docs/upgrade.md matrix row, and apply BUMP-FLIP(flux-notes) in chart/kuberik/templates/NOTES.txt if the release no longer needs ImagePolicy. Review git diff."
