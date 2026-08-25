# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Response Style

**Be concise. Always.** Short direct answers, bullets over paragraphs, code over
prose, tables for comparisons. Get to the point immediately.

## Code Comments

**Default to zero comments.** Well-named identifiers and the surrounding code
should convey what the code does. Do not narrate changes, reference tasks/PRs,
or annotate "added for X" / "removed Y".

## What This Is

`locations` is the canonical registry of the places the platform serves from. It
answers "what locations exist" and "what can I use where", and publishes those
answers to the surfaces that need them. See [README.md](README.md) for the
product framing.

It is a Kubebuilder v4 / controller-runtime operator (Go module
`go.miloapis.com/locations`), API group `locations.miloapis.com`.

The service does **not** provision anything. It records a footprint and projects
it — into project control planes as `Location`, and onto cells as
`ServingLocation`.

## API Group

- Group: `locations.miloapis.com`
- Version: `v1alpha1`
- Kinds: `Location` (canonical, cluster-scoped, also the projection into a
  project), `LocationClass` (what backs a location), `ServingLocation` (copy
  delivered to a cell)

## Repo Layout

```
locations/
├── cmd/locations/main.go   # Binary entrypoint
├── api/v1alpha1/           # CRD type definitions
├── internal/
│   ├── config/             # Operator configuration
│   └── controller/         # Reconcilers
├── config/                 # Kustomize manifests (base/components/overlays)
├── ui/                     # Remix UI (React + datum-ui)
├── hack/                   # Scripts and boilerplate
└── test/crd/               # Generated CRDs against envtest
```

## Changing an API

1. Edit `api/v1alpha1/*_types.go` and the kubebuilder markers.
2. `task generate && task manifests` — regenerates deepcopy, CRDs, RBAC, webhooks.
3. Extend the reconciler in `internal/controller/` and add a `_test.go`.
4. `task test`.

Never hand-edit generated files (`zz_generated.*`, `config/base/crd/bases/*`).
RBAC comes from `+kubebuilder:rbac` markers — do not edit `role.yaml` by hand.

## Connecting to the Milo Control Plane

The operator config takes a `kubeconfigPath` pointing at Milo's API server:

```yaml
apiVersion: apiserver.config.miloapis.com/v1alpha1
kind: LocationOperator
metricsServer:
  bindAddress: "0"
kubeconfigPath: /etc/milo/kubeconfig
```

Empty `kubeconfigPath` falls back to in-cluster config — the default for local
kind development.

## Verification Commands

```bash
# Controller
task build                    # Build binary
task test                     # Run tests
task lint                     # Run linter
task generate                 # Run code generation
task manifests                # Generate CRD/RBAC/webhook manifests
task dev:setup                # Bootstrap kind cluster + deploy
task dev:redeploy             # Rebuild and redeploy

# UI
task ui:install               # Install UI dependencies (pnpm)
task ui:dev                   # Start dev server at http://localhost:3000
task ui:build                 # Production build
task ui:type-check            # TypeScript check
```

## Git Commit Message Format

```
<type>: <subject line max 50 chars>

<Body wrapped at 80 chars explaining what and why>
```

Imperative mood, no period in the subject. Types: feat, fix, refactor, docs,
test, chore, perf, style, ci. No watermarks or co-author tags. The 80-char wrap
applies to commit messages only — PR descriptions and issues render as markdown
and should not be hard-wrapped.
