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

import { describe, expect, it } from 'vitest';
import { operationRole, prefixApiUrl, roleForPath } from '../ApiDocsHelper';
import { DOCS } from '../DocsHelper';

describe('DocsHelper', () => {
  it('has unique slugs and no guide for the API reference route', () => {
    const slugs = DOCS.map((d) => d.slug);

    expect(new Set(slugs).size).toBe(slugs.length);
    // "api" is the route of the API reference, not a guide
    expect(slugs).not.toContain('api');
  });
});

describe('ApiDocsHelper', () => {
  it('marks viewer, public and admin operations', () => {
    expect(operationRole('Resilience_GetProxyStatus')).toBe('viewer');
    expect(operationRole('Resilience_GetStatsHistory')).toBe('viewer');
    expect(operationRole('Tutti_GetAppVersion')).toBe('public');
    expect(operationRole('Webhook_S3Notifications')).toBe('public');
    expect(operationRole('Resilience_SetFailoverPolicy')).toBe('admin');
    // secret keys are never viewer-readable
    expect(operationRole('Tutti_GetProxyCredentials')).toBe('admin');
    expect(operationRole(undefined)).toBe('admin');
  });

  it('finds the role of a method and path in the spec', () => {
    const spec = {
      paths: {
        '/tutti/proxies': { get: { operationId: 'Resilience_GetProxyStatus' } },
      },
    };

    expect(roleForPath(spec, 'GET', '/tutti/proxies')).toBe('viewer');
    expect(roleForPath(spec, 'PUT', '/tutti/proxies')).toBe('admin');
  });

  it('prefixes requests with the API base and keeps the query', () => {
    const origin = 'http://localhost:5173';

    expect(
      prefixApiUrl(
        `${origin}/tutti/stats/history?range=1h`,
        '',
        '/api',
        origin,
      ),
    ).toBe('/api/tutti/stats/history?range=1h');
    expect(
      prefixApiUrl(`${origin}/version`, 'https://w.example', '', origin),
    ).toBe('https://w.example/version');
    expect(prefixApiUrl('https://other.example/x', '', '/api', origin)).toBe(
      'https://other.example/x',
    );
  });
});
