export const API_GROUP = "locations.miloapis.com";
export const API_VERSION = "v1alpha1";
export const RESOURCES_BASE = `/apis/${API_GROUP}/${API_VERSION}`;

export function locationsPath() {
  return `${RESOURCES_BASE}/locations`;
}

export function locationPath(name: string) {
  return `${RESOURCES_BASE}/locations/${name}`;
}
