# Contributing to Kuberik

We welcome contributions from everyone. This file covers what lives where, how to send changes to this repo (the lighthouse / CLI repo), and where to send changes to the controllers.

## Filing issues

Use [GitHub Issues](https://github.com/kuberik/kuberik/issues) for bugs, feature requests, and questions about the CLI, install manifest, or documentation. Issues against a specific controller belong in that controller's repository.

For broader questions or design discussion, start a thread in [GitHub Discussions](https://github.com/kuberik/kuberik/discussions).

## Where the code lives

| Component | Repository |
| --- | --- |
| Core controller (CRDs, reconcilers) | [rollout-controller](https://github.com/kuberik/rollout-controller) |
| Web dashboard | [rollout-dashboard](https://github.com/kuberik/rollout-dashboard) |
| Datadog integration | [datadog-controller](https://github.com/kuberik/datadog-controller) |
| GitHub Deployments integration | [environment-controller](https://github.com/kuberik/environment-controller) |
| OpenKruise integration | [openkruise-controller](https://github.com/kuberik/openkruise-controller) |
| Prometheus integration | [prometheus-controller](https://github.com/kuberik/prometheus-controller) |

This repo (`kuberik/kuberik`) holds the `kuberik` CLI, the GitHub Action, the install bundle, the RFCs, and project-wide documentation.

## Working on the CLI

You need Go 1.24 or newer and `kubectl` on PATH (for end-to-end tests).

```bash
git clone https://github.com/kuberik/kuberik.git
cd kuberik

make build       # compiles bin/kuberik
make test        # runs unit tests with race detector
make vet         # go vet
make lint        # golangci-lint (installs separately)
make tidy        # go mod tidy
```

The `make build` target embeds the version via `-ldflags`. The release pipeline uses goreleaser; see `.goreleaser.yml`.

### Adding a CLI command

1. Add a new file under `cmd/kuberik/` with the command's cobra definition.
2. Register the command on `rootCmd` in the file's `init()`.
3. Add the command to the list in `cmd/kuberik/main_test.go::TestRootCommand`.
4. Document the command in `docs/cli.md`.
5. Update the README install / usage snippets if the command changes the user-facing surface.

CLI conventions:

- Shell out to `kubectl` for cluster operations using the `kubectl()` helper in `cmd/kuberik/kubectl.go`.
- Honor the global `--kubeconfig` and `-n/--namespace` flags (or `--all-namespaces` for list commands).
- Errors must be actionable. Wrap with `%w` so callers can inspect.

## Working on the install bundle

The install bundle pins compatible versions of each controller in `config/install/kustomization.yaml`. Bumping a controller version is a one-line change; please call out the upstream release notes in the PR description.

## Writing docs

User-facing documentation lives in `docs/`. The README is a poster, not a manual - link out to deeper docs rather than expanding the README.

Conventions:

- Show working YAML, not pseudocode.
- Lead with the user problem, not the implementation.
- Avoid marketing prose ("powerful", "robust", "comprehensive").

## Proposing larger changes

Substantial changes (new CRDs, breaking API changes, cross-component behavior changes) go through an [RFC](rfcs/README.md). Open a discussion first to see whether your idea needs one.

## Commit and PR style

- Imperative mood, capitalized, no trailing period.
- Body wrapped at 72 columns, explaining what and why.
- One concern per commit. Drive-by formatting changes belong in their own PR.
- No `@mentions` or `#123` in the subject - put those in the PR description.

Before opening a PR:

```bash
make tidy fmt vet test
```

All four must produce a clean tree.

## Code of conduct

This project follows the [CNCF Code of Conduct](CODE_OF_CONDUCT.md). Be kind. Assume good faith.
