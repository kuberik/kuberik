# Kuberik Helm Chart

[![helm](https://img.shields.io/badge/helm-kuberik.github.io%2Fkuberik-blue.svg)](https://kuberik.github.io/kuberik)
[![oci](https://img.shields.io/badge/oci-ghcr.io%2Fkuberik%2Fcharts%2Fkuberik-blueviolet.svg)](https://github.com/kuberik/kuberik/pkgs/container/charts%2Fkuberik)

A Helm chart that installs the Kuberik rollout-controller and optionally the integration controllers. Alternative to the [kustomize bundle](../../config/install/kustomization.yaml).

The chart installs:

- 11 CRDs, rendered as chart resources (`templates/crds.yaml` reads `files/crds/`) so `helm upgrade` installs new ones and updates existing ones: Kuberik core (incl. `RolloutDependency`, v0.9, and the manual-approval kinds `ApprovalPolicy`, `ClusterApprovalPolicy`, `RolloutApproval`) + openkruise + environment. They carry `helm.sh/resource-policy: keep`.
- rollout-controller: ServiceAccount, leader-election Role/RoleBinding, 11 ClusterRoles (incl. `kuberik-rollout-approver`) and 3 ClusterRoleBindings, metrics Service, Deployment.
- On Kubernetes 1.30+, two ValidatingAdmissionPolicies with bindings (see [Manual approvals](#manual-approvals)).
- Optional integration controllers (toggle in `values.yaml`):
  - **datadog-controller** - templated
  - **openkruise-controller** - templated
  - **environment-controller** - templated
  - **prometheus-controller** - no upstream release yet; NOTES tells you to build from source
- Dashboard toggle prints the kubectl apply line in NOTES until templates are added.

## Install

### gh-pages helm repo

```bash
helm repo add kuberik https://kuberik.github.io/kuberik
helm repo update
helm install kuberik kuberik/kuberik \
  --namespace kuberik-system --create-namespace \
  --set createNamespace=false
```

### OCI registry (Helm 3.8+)

```bash
helm install kuberik oci://ghcr.io/kuberik/charts/kuberik \
  --namespace kuberik-system --create-namespace \
  --set createNamespace=false
```

### From this directory directly

```bash
helm install kuberik ./chart/kuberik \
  --namespace kuberik-system
```

## Values

| Key | Default | Description |
| --- | --- | --- |
| `namespace` | `kuberik-system` | Namespace the controllers run in |
| `crds.install` | `true` | Render the CRDs. Set false if you manage them outside the chart |
| `crds.keep` | `true` | Annotate the CRDs `helm.sh/resource-policy: keep` so `helm uninstall` leaves them (and every CR) in place |
| `admissionPolicies.enabled` | `true` | Render the approval ValidatingAdmissionPolicies (only on clusters serving `ValidatingAdmissionPolicy`, Kubernetes 1.30+) |
| `rolloutController.enabled` | `true` | Install the core controller |
| `rolloutController.image.repository` | `ghcr.io/kuberik/rollout-controller` | Controller image |
| `rolloutController.image.tag` | _appVersion_ | Image tag override |
| `rolloutController.replicas` | `1` | Number of controller replicas |
| `rolloutController.logLevel` | `info` | Controller log level |
| `integrations.datadog.enabled` | `false` | Install datadog-controller |
| `integrations.prometheus.enabled` | `false` | Install prometheus-controller |
| `integrations.openkruise.enabled` | `false` | Install openkruise-controller |
| `integrations.environment.enabled` | `false` | Install environment-controller |
| `dashboard.enabled` | `false` | Install rollout-dashboard |
| `dashboard.ingress.enabled` | `false` | Expose the dashboard via a `networking.k8s.io` Ingress |
| `dashboard.ingress.host` | `""` | Required when ingress is enabled |
| `dashboard.gateway.enabled` | `false` | Expose the dashboard via a Gateway API HTTPRoute (alternative to `ingress`) |
| `dashboard.gateway.parentRef.name` | `""` | Gateway the HTTPRoute attaches to |
| `dashboard.gateway.hostname` | `""` | Hostname the dashboard serves at |
| `dashboard.gateway.auth` | `false` | Gate the HTTPRoute with the shared `auth` block via a SecurityPolicy |
| `auth.enabled` | `false` | Install the cluster-level oauth2-proxy auth gate in its own namespace |
| `auth.oidc.issuerUrl` | `""` | OIDC discovery URL of your IdP |
| `auth.oidc.clientId` | `kuberik-cluster` | OAuth2 client id; also configure as kube-apiserver `--oidc-client-id` |
| `auth.canonicalHost` | `""` | Host that owns the OAuth2 callback (`/oauth2/callback`) |
| `auth.cookieDomain` | _canonicalHost_ | Cookie scope; set a parent domain to share session across subdomains |
| `auth.gateway.name` | `""` | Gateway the `/oauth2/*` HTTPRoute attaches to |
| `auth.apiRoutes` | `["^/api/"]` | Paths answered `401` instead of redirected to the IdP, so the dashboard's polling/SSE requests don't start background OIDC flows |
| `metrics.serviceMonitor.enabled` | `false` | Prometheus Operator ServiceMonitor for the rollout-controller |
| `networkPolicy.enabled` | `false` | Restrict ingress/egress for the controller pods |
| `rolloutController.podDisruptionBudget.enabled` | `false` | Emit a PDB (requires replicas > 1) |
| `createNamespace` | `true` | Have the chart create the Namespace. Set false when using `helm install --create-namespace` |

See [values.yaml](values.yaml) for the full schema.

## Manual approvals

The chart ships the `ApprovalPolicy`, `ClusterApprovalPolicy` and `RolloutApproval` CRDs and the controller RBAC for them. It also creates the ClusterRole `kuberik-rollout-approver` (`get/list/watch/create/delete` on `rolloutapprovals`). The role is not aggregated into `edit` or `admin`; bind it per namespace, for example:

```bash
kubectl -n shop create rolebinding shop-approvers \
  --clusterrole=kuberik-rollout-approver --group=shop-oncall
```

On Kubernetes 1.30+ (where `ValidatingAdmissionPolicy` is GA) the chart also renders two policies, each with a `Deny` binding:

- `kuberik-rolloutapproval-by` - `approval.kuberik.com/by` on a RolloutApproval must equal the requesting user and is immutable.
- `kuberik-approval-gate-guard` - only the rollout-controller service account (`system:serviceaccount:<namespace>:rollout-controller-controller-manager`, namespace from `namespace`), the garbage collector and the namespace controller may create, change or delete a RolloutGate owned by an approval policy.

On older clusters, or with `admissionPolicies.enabled=false`, approvals still work but `by` is unverified, policy-owned gates can be edited by anyone who may update RolloutGates, and the dashboard does not show `by` (it checks for the `kuberik-rolloutapproval-by` binding at startup).

## Common scenarios

### Production (HA, hardened, metrics, ingress)

A complete opinionated overrides file ships with the chart at [values-production.yaml](values-production.yaml). Use as-is or as a starting point:

```bash
helm install kuberik oci://ghcr.io/kuberik/charts/kuberik \
  --namespace kuberik-system --create-namespace \
  -f values-production.yaml
```

The file enables: 3-replica rollout-controller with PDB and topology spread, `system-cluster-critical` priority, restricted PSA on the namespace, ServiceMonitor + NetworkPolicy, dashboard with TLS Ingress, Datadog and environment-controller integrations.

## Upgrade

```bash
helm repo update kuberik
helm upgrade kuberik kuberik/kuberik -n kuberik-system
```

Since chart 0.7.0 the CRDs are ordinary chart resources, so `helm upgrade` installs new CRDs and updates existing ones. Charts up to 0.6.1 shipped them in `crds/`, which Helm creates on install and never touches again. Upgrading from a chart ≤ 0.6.1 therefore needs a one-time hand-over of the existing CRDs to the release, otherwise Helm refuses with `invalid ownership metadata`:

```bash
kubectl get crd -o name | grep -E 'kuberik\.com$' | xargs -I{} kubectl label --overwrite {} app.kubernetes.io/managed-by=Helm
kubectl get crd -o name | grep -E 'kuberik\.com$' | xargs -I{} kubectl annotate --overwrite {} meta.helm.sh/release-name=kuberik meta.helm.sh/release-namespace=kuberik-system
```

Use your own release name and namespace. Helm 3.17+ can do the same with `helm upgrade --take-ownership`. See [docs/upgrade.md](../../docs/upgrade.md) for the chart → controller version matrix.

## Uninstall

```bash
helm uninstall kuberik -n kuberik-system
```

CRDs are not removed by `helm uninstall` (they carry `helm.sh/resource-policy: keep`). To delete them:

```bash
kubectl delete crd \
  rollouts.kuberik.com \
  rolloutgates.kuberik.com \
  healthchecks.kuberik.com \
  rolloutschedules.kuberik.com \
  clusterrolloutschedules.kuberik.com \
  rolloutdependencies.kuberik.com \
  rollouttests.rollout.kuberik.com \
  environments.environments.kuberik.com
```

This also deletes every `Rollout`, `RolloutGate`, `HealthCheck`, `RolloutTest`, and `Environment` in the cluster.

### OIDC-gated dashboard (Gateway API)

Put the dashboard behind your OIDC provider via a shared oauth2-proxy in `auth-system`. Same audience works for kube-apiserver, so every dashboard action runs as the logged-in user under RBAC.

```yaml {filename="auth-values.yaml"}
dashboard:
  enabled: true
  gateway:
    enabled: true
    parentRef:
      name: eg
      namespace: envoy-gateway-system
    hostname: dashboard.kuberik.example.com
    auth: true

auth:
  enabled: true
  oidc:
    issuerUrl: https://idp.example.com
    clientId: kuberik-cluster
    clientSecret: <your-oidc-client-secret>  # or set existingSecret
  canonicalHost: dashboard.kuberik.example.com
  cookieDomain: .example.com
  gateway:
    name: eg
    namespace: envoy-gateway-system
```

Set kube-apiserver flags:
`--oidc-issuer-url=https://idp.example.com --oidc-client-id=kuberik-cluster --oidc-username-claim=email`

To gate another service later, add its namespace to `auth.allowedConsumerNamespaces` and apply a `SecurityPolicy` in that namespace pointing at `oauth2-proxy.auth-system:4180`.

## Enabling everything

```bash
helm install kuberik ./chart/kuberik \
  --namespace kuberik-system --create-namespace \
  --set integrations.datadog.enabled=true \
  --set integrations.openkruise.enabled=true \
  --set integrations.environment.enabled=true
```

## Troubleshooting

### `namespaces "kuberik-system" already exists` during install

You used `helm install --create-namespace` while the chart also tried to create the namespace. Pick one:

```bash
# A) Let helm own the namespace
helm install kuberik ./chart/kuberik \
  --namespace kuberik-system --create-namespace \
  --set createNamespace=false

# B) Let the chart own the namespace
helm install kuberik ./chart/kuberik \
  --namespace kuberik-system
```

### `helm test` pod can't list deployments

The default ServiceAccount in the namespace usually cannot list deployments. The chart's test pod uses `rollout-controller-controller-manager` by default, which has the necessary permissions. If you override `tests.serviceAccountName`, make sure it can `get`/`list`/`watch` Deployments.

### Image pull failures for `rollout-controller`

The chart defaults `rolloutController.image.tag` to `v` + `Chart.AppVersion`. If you are testing a fork with a non-`v`-prefixed tag, set the tag explicitly:

```bash
--set rolloutController.image.tag=0.7.0
```

### ServiceMonitor not picked up by Prometheus

The ServiceMonitor lives in `.Values.namespace` by default. If your Prometheus Operator scrapes a different namespace, set:

```bash
--set metrics.serviceMonitor.namespace=monitoring
```
