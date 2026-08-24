import { json } from "@remix-run/node";
import type { LoaderFunctionArgs } from "@remix-run/node";
import { Link, useLoaderData } from "@remix-run/react";
import { useMemo, useState } from "react";
import { Badge } from "@datum-cloud/datum-ui/badge";
import { Card, CardContent } from "@datum-cloud/datum-ui/card";
import { EmptyContent } from "@datum-cloud/datum-ui/empty-content";
import { Input } from "@datum-cloud/datum-ui/input";
import { PageTitle } from "@datum-cloud/datum-ui/page-title";
import { Search } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@datum-cloud/datum-ui/table";
import { fetchK8s } from "~/lib/k8s.server";
import { conditionBadgeProps, relativeAge } from "~/lib/format";
import type { KubeList, Location } from "~/lib/types";
import { CITY_CODE_KEY, REGION_KEY, readyCondition } from "~/lib/types";
import { locationsPath } from "~/lib/api";

interface LoaderData {
  locations: Location[];
  error?: string;
}

export async function loader({ request }: LoaderFunctionArgs) {
  try {
    const result = await fetchK8s<KubeList<Location>>(
      request,
      locationsPath()
    );
    return json({ locations: result.items ?? [] } satisfies LoaderData);
  } catch (e) {
    return json({
      locations: [],
      error: e instanceof Error ? e.message : String(e),
    } satisfies LoaderData);
  }
}

function matchesQuery(location: Location, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  return [
    location.metadata.name,
    location.spec.topology?.[CITY_CODE_KEY] ?? "",
    location.spec.topology?.[REGION_KEY] ?? "",
  ].some((h) =>
    h.toLowerCase().includes(needle)
  );
}

export default function LocationsIndex() {
  const { locations, error } = useLoaderData<typeof loader>() as LoaderData;
  const [query, setQuery] = useState("");

  const filtered = useMemo(
    () => locations.filter((r) => matchesQuery(r, query.trim())),
    [locations, query]
  );

  return (
    <div className="flex flex-col gap-4 px-6 py-4">
      <PageTitle
        title="Locations"
        description="Every place the platform can serve from."
        actionsPosition="inline"
      />

      {error ? (
        <Card>
          <CardContent className="py-6">
            <p className="text-sm font-medium">Failed to load locations</p>
            <p className="text-sm text-muted-foreground mt-1">{error}</p>
            <a href="/locations" className="text-sm text-primary underline mt-2 inline-block">
              Retry
            </a>
          </CardContent>
        </Card>
      ) : locations.length === 0 ? (
        <EmptyContent
          title="no locations found."
          subtitle="Locations will appear here once they are created on the Milo control plane."
          size="lg"
        />
      ) : (
        <>
          <div className="relative max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="search"
              placeholder="Search locations…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9"
            />
          </div>
          {filtered.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center gap-3 py-8">
                <p className="text-sm font-medium">
                  No matches for &ldquo;{query}&rdquo;.
                </p>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>City</TableHead>
                      <TableHead>Class</TableHead>
                      <TableHead>Ready</TableHead>
                      <TableHead>Age</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filtered.map((r) => {
                      const ready = readyCondition(r);
                      const badge = conditionBadgeProps(ready?.status ?? "");
                      return (
                        <TableRow key={r.metadata.name}>
                          <TableCell>
                            <Link
                              to={`/locations/${encodeURIComponent(r.metadata.name)}`}
                              className="text-primary hover:underline"
                            >
                              {r.metadata.name}
                            </Link>
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {r.spec.topology?.[CITY_CODE_KEY] ?? "—"}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {r.spec.locationClassName ?? "—"}
                          </TableCell>
                          <TableCell>
                            {ready ? (
                              <Badge type={badge.type} theme={badge.theme}>
                                {ready.status}
                              </Badge>
                            ) : (
                              "—"
                            )}
                          </TableCell>
                          <TableCell>
                            {relativeAge(r.metadata.creationTimestamp)}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
