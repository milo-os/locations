# LocationsRemovalBlocked

## Meaning

A Location was deleted at the source more than a day ago, but its published
ServingLocation is still on the federation hub. The publisher's removal guard
refuses the delete, so the publisher keeps the copy and reports it in the
`locations_removal_blocked` and `locations_retained` metrics.

## Impact

Cells still see the location. A cluster still labelled for it keeps serving it,
so customers can keep placing workloads in a location that no longer exists at
the source.

## Diagnosis

1. On the federation hub, read the reason the guard recorded:

   ```shell
   kubectl get servinglocation LOCATION \
     -o jsonpath='{.metadata.annotations.networking\.datumapis\.com/removal-blocked}'
   ```

   Replace `LOCATION` with the alert's `location` label.

1. List the Ready hub clusters and their location labels:

   ```shell
   kubectl get clusters.cluster.karmada.io -L topology.datum.net/location
   ```

The reason is one of the following:

- `ClustersStillServing`: one or more Ready clusters are still labelled
  `topology.datum.net/location=LOCATION`.
- `FleetNotFullyLabelled`: no cluster claims the location, but some Ready
  clusters carry no location label, so the guard can't rule them out.

## Mitigation

- **`ClustersStillServing`**: confirm the location is being retired, then move
  or remove the `topology.datum.net/location` label on each cluster listed in
  the annotation.
- **`FleetNotFullyLabelled`**: label each cluster listed in the annotation with
  the location it serves.

The publisher retries on its own and deletes the copy once the guard allows it.

If the location must go before the fleet is fixed, override the guard on the
hub copy. This removes the location from every cluster that still serves it:

```shell
kubectl annotate servinglocation LOCATION networking.datumapis.com/removal-override=true
```

## Why the rule usually selects no series

`locations_removal_blocked` exists only while a removal is blocked: the
publisher sets it when the guard refuses a delete and removes it once the
removal goes through. A healthy fleet has no series, so the rule carries
`expected_no_match: "true"` to keep the new-rule and dead-rule checks from
reporting it. Check that the operator's metrics are exported with
`locations_retained`, which always exists.
