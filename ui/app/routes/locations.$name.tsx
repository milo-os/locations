import { json } from "@remix-run/node";
import type { LoaderFunctionArgs } from "@remix-run/node";
import { useLoaderData } from "@remix-run/react";
import { Badge } from "@datum-cloud/datum-ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@datum-cloud/datum-ui/card";
import { PageTitle } from "@datum-cloud/datum-ui/page-title";
import { fetchK8s } from "~/lib/k8s.server";
import { conditionBadgeProps, relativeAge } from "~/lib/format";
import type { Location } from "~/lib/types";
import { CITY_CODE_KEY, REGION_KEY, readyCondition } from "~/lib/types";
import { locationPath } from "~/lib/api";

interface LoaderData {
  location: Location | null;
  error?: string;
}

export async function loader({ request, params }: LoaderFunctionArgs) {
  const name = params.name ?? "";
  try {
    // Locations are namespaced — list all and find by name for the template.
    // In practice, scope this to a specific namespace via query param or config.
    const location = await fetchK8s<Location>(
      request,
      locationPath(encodeURIComponent(name))
    );
    return json({ location } satisfies LoaderData);
  } catch (e) {
    return json({
      location: null,
      error: e instanceof Error ? e.message : String(e),
    } satisfies LoaderData);
  }
}

export default function LocationDetail() {
  const { location, error } = useLoaderData<typeof loader>() as LoaderData;

  if (error || !location) {
    return (
      <div className="px-6 py-4">
        <Card>
          <CardContent className="py-6">
            <p className="text-sm font-medium">Failed to load location</p>
            <p className="text-sm text-muted-foreground mt-1">{error}</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  const ready = readyCondition(location);
  const badge = ready ? conditionBadgeProps(ready.status) : null;

  return (
    <div className="flex flex-col gap-4 px-6 py-4">
      <div className="flex items-center gap-3">
        <PageTitle title={location.metadata.name} actionsPosition="inline" />
        {badge && ready && (
          <Badge type={badge.type} theme={badge.theme}>
            Ready: {ready.status}
          </Badge>
        )}
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Details</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">City</dt>
                <dd>{location.spec.topology?.[CITY_CODE_KEY] ?? "—"}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Region</dt>
                <dd>{location.spec.topology?.[REGION_KEY] ?? "—"}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Class</dt>
                <dd>{location.spec.locationClassName ?? "—"}</dd>
              </div>
              {location.spec.coordinates && (
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Coordinates</dt>
                  <dd>
                    {location.spec.coordinates.latitude},{" "}
                    {location.spec.coordinates.longitude}
                  </dd>
                </div>
              )}
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Age</dt>
                <dd>{relativeAge(location.metadata.creationTimestamp)}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        {location.status?.conditions && location.status.conditions.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Conditions</CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="space-y-2 text-sm">
                {location.status.conditions.map((c) => (
                  <div key={c.type} className="flex justify-between">
                    <dt className="text-muted-foreground">{c.type}</dt>
                    <dd className="flex items-center gap-2">
                      <span
                        className={
                          c.status === "True"
                            ? "text-green-600 dark:text-green-400"
                            : "text-muted-foreground"
                        }
                      >
                        {c.status}
                      </span>
                      {c.reason && (
                        <span className="text-muted-foreground">({c.reason})</span>
                      )}
                    </dd>
                  </div>
                ))}
              </dl>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
