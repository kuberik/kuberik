{{/*
Component image versions: the default tag of each controller image, in one place.

Every value below is RELEASE-PLACEHOLDER on main between releases. The release
process replaces them with the real tags by running, from the repository root:

  hack/set-chart-versions.sh --rollout-controller vX.Y.Z --release-controller vX.Y.Z \
    --dashboard vX.Y.Z --environment-controller vX.Y.Z --datadog-controller vX.Y.Z \
    --openkruise-controller vX.Y.Z

which also sets Chart.yaml appVersion (the rollout-controller version) and the pins
in config/install/kustomization.yaml, and fails if any placeholder remains.
The chart-release workflow refuses to publish while one does
(`hack/set-chart-versions.sh --check`). Do not edit these by hand and do not
put a real tag here outside a release: `image.tag` in values.yaml is the
per-install override.
*/}}
{{- define "kuberik.version.releaseController" -}}RELEASE-PLACEHOLDER{{- end -}}
{{- define "kuberik.version.dashboard" -}}RELEASE-PLACEHOLDER{{- end -}}
{{- define "kuberik.version.environmentController" -}}RELEASE-PLACEHOLDER{{- end -}}
{{- define "kuberik.version.datadogController" -}}RELEASE-PLACEHOLDER{{- end -}}
{{- define "kuberik.version.openkruiseController" -}}RELEASE-PLACEHOLDER{{- end -}}
