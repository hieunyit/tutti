/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

export type ApiRole = 'public' | 'viewer' | 'admin';

/**
 * Operations a viewer may call: the allow-list of pkg/auth/interceptor.go
 * (viewerMethods) written as OpenAPI operationIds ("Service_Method"). Every
 * other operation is admin-only, like on the server (fail closed).
 */
const VIEWER_OPERATIONS = new Set([
  'Chorus_GetStorages',
  'Diff_List',
  'Diff_GetReport',
  'Diff_GetReportEntries',
  'Policy_AvailableBuckets',
  'Policy_GetReplication',
  'Policy_ListReplications',
  'Policy_StreamReplication',
  'Policy_GetSwitchStatus',
  'Policy_ListRoutings',
  'Resilience_GetProxyStatus',
  'Resilience_GetStatsHistory',
  'Resilience_ListFailoverPolicies',
  'Resilience_ListFailoverEvents',
  'Resilience_ListAccessPolicies',
  'Resilience_ListProxyAccounts',
]);

/** Reachable without a session (publicMethods / publicMethodPrefixes). */
function isPublicOperation(operationId: string): boolean {
  return (
    operationId === 'Chorus_GetAppVersion' || operationId.startsWith('Webhook_')
  );
}

export function operationRole(operationId: string | undefined): ApiRole {
  if (!operationId) return 'admin';

  if (isPublicOperation(operationId)) return 'public';

  return VIEWER_OPERATIONS.has(operationId) ? 'viewer' : 'admin';
}

export interface SpecOperation {
  method: string;
  path: string;
  operationId?: string;
}

/** Role of the operation at `method path` of an OpenAPI 2.0 document. */
export function roleForPath(
  spec: { paths?: Record<string, Record<string, { operationId?: string }>> },
  method: string,
  path: string,
): ApiRole {
  return operationRole(spec.paths?.[path]?.[method.toLowerCase()]?.operationId);
}

/**
 * The spec has no host or basePath, so Swagger UI would call the origin of the
 * page. Requests are sent to the management API instead: base URL and prefix
 * (VITE_API_BASE_URL, VITE_API_PREFIX) plus the spec path.
 */
export function prefixApiUrl(
  url: string,
  baseUrl: string,
  prefix: string,
  origin: string,
): string {
  const parsed = new URL(url, origin);

  // already pointing somewhere else (e.g. an absolute http(s) host in the spec)
  if (parsed.origin !== origin) return url;

  return `${baseUrl}${prefix}${parsed.pathname}${parsed.search}`;
}
