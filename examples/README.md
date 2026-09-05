# Examples

Copy-pasteable manifests showing common Kuberik configurations. Each subdirectory is a self-contained example.

| Example | What it shows |
| --- | --- |
| [basic-rollout](basic-rollout) | A minimal Rollout tracking one image with a bake period |
| [with-gate](with-gate) | Adding a manual approval gate to a Rollout |
| [with-schedule](with-schedule) | Business-hours-only promotions and holiday change freezes |
| [with-datadog](with-datadog) | A Datadog monitor that gates the bake period via a HealthCheck |
| [with-prometheus](with-prometheus) | A PromQL query that gates the bake period via a HealthCheck |
| [with-environments](with-environments) | Multi-environment promotion order (staging → production) |

Full conceptual docs at [kuberik.com/docs](https://kuberik.com/docs/).
