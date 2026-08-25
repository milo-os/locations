# Locations

**Every location. One answer.**

`locations` is the canonical registry of the places a platform can serve from,
and the source every other surface reads to answer two questions:

- **What locations exist?**
- **What can I run or use in each one?**

Without a registry, those answers get re-derived independently by the console,
the CLI, the docs, and each service's own config — and they drift. A customer
picks a city the console offers, the deployment fails, and a support ticket
follows. This service exists so there is one authoritative answer, published
once and consumed everywhere.

## What a Location is

A `Location` is a place the platform serves traffic or runs workloads from —
typically a city-level presence rather than a cloud vendor's region. Each one
carries:

- **Topology** — where in the world it is (`city-code`, `region`, and any other
  keys you need). Workloads placed at a location inherit these, and placement
  requests that name a city are answered from them.
- **Class** — which `LocationClass` backs it, naming the controller that brings
  the location up and pointing at the capacity behind it. The reference carries
  an optional project, so a location can name a class in its own control plane
  or one in the provider's.
- **Coordinates** — latitude and longitude, for anything that needs to plot the
  location on a map.

Locations are declared once by whoever operates the footprint. Everything
downstream is a projection of that declaration, never an independent copy.

### Classes, and whose capacity is behind a location

A `LocationClass` lives in the control plane of whoever owns the capacity it
describes, and that placement is what tells you who the location belongs to:

| Where the class lives | What it means | Who manages the class |
|---|---|---|
| The provider's control plane | The provider's own footprint, offered to you | The provider |
| Your control plane | Capacity you brought — your own cloud account, your own hardware | You |

So there is no separate field saying who runs a location. **Whose capacity is
this** is answered by which control plane holds the class, and **who operates
it** is answered by `spec.controllerName` on that class: only the controller
named there acts on locations of that class, so several providers can serve
locations side by side without contending for the same objects.

Bringing your own capacity means declaring a class in your own control plane
and pointing your `Location` at it — the same two objects the platform uses for
its own footprint, no special case.

## How locations reach the people and systems that need them

| Resource | Who reads it | What it is |
|---|---|---|
| `Location` | Platform operators | The canonical record. The only object anyone edits. |
| `LocationClass` | Platform operators, capacity providers | A kind of location: the controller that serves it and the configuration for the capacity behind it. Lives in the control plane of whoever owns that capacity. |
| `Location` | Consumers, in their own control plane | The same kind, projected into a project — present only once the location is ready and the service is actually offered there. |
| `ServingLocation` | A cell, in its own cluster | A read-only copy delivered to a cluster telling it which location it serves. |

The distinction that matters: **a `Location` existing does not mean a consumer
can use it.** A `Location` appearing in a project's control plane is the
statement that they can. That gap is deliberate — it's where availability,
readiness, and rollout live.

A **cell** — a cluster running workloads at one physical location — cannot work
out where it is on its own. It's told, via a `ServingLocation` delivered to it.
Everything the cell does that depends on where it sits, such as claiming network
addresses, resolves through that one object. A cell that has been delivered two
of them refuses to guess between them.

## Relationship to other Milo services

- **[`inventory`](https://github.com/milo-os/inventory)** records the physical
  estate — regions, sites, racks, nodes, links. It answers *"what hardware do we
  have and where is it?"*
- **`locations`** records the consumer-facing footprint. It answers *"where can
  I run this, and what's offered there?"*

A location may be backed by inventory assets, several of them, or none at all.
The two models are deliberately separate: the estate changes for operational
reasons, the footprint changes for product reasons, and neither should force the
other.

## Usage

Locations are served by the `locations.miloapis.com` API group. Operators
declare them on the platform control plane:

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: locations.miloapis.com/v1alpha1
kind: Location
metadata:
  name: us-east-1
spec:
  locationClassRef:
    name: shared-edge
  topology:
    topology.datum.net/city-code: IAD
    topology.datum.net/region: us-east-1
  coordinates:
    latitude: "38.9445"
    longitude: "-77.4558"
EOF
```

Consumers list what is available to them, in their own control plane:

```bash
datumctl auth update-kubeconfig --project your-project
datumctl get locations
```

## Development

```bash
task build       # Build the binary
task test        # Run tests
task lint        # Run the linter
task generate    # Run code generation
task manifests   # Generate CRD, RBAC, and webhook manifests
task dev:setup   # Bring up a kind cluster, build, load, and deploy
```

## License

AGPL-3.0-only. See [LICENSE](LICENSE).
