export interface KubeMeta {
  name: string;
  namespace?: string;
  creationTimestamp: string;
}

export interface KubeList<T> {
  items: T[];
}

export interface KubeCondition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
}

export interface Coordinates {
  latitude: string;
  longitude: string;
}

export interface Location {
  metadata: KubeMeta;
  spec: {
    locationClassName?: string;
    topology: Record<string, string>;
    provider?: {
      gcp?: {
        projectId?: string;
        region?: string;
        zone?: string;
      };
    };
    coordinates?: Coordinates;
  };
  status?: {
    conditions?: KubeCondition[];
  };
}

export const CITY_CODE_KEY = "topology.datum.net/city-code";
export const REGION_KEY = "topology.datum.net/region";

export function readyCondition(location: Location): KubeCondition | undefined {
  return location.status?.conditions?.find((c) => c.type === "Ready");
}
