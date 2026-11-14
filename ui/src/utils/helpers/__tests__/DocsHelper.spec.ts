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
import {
  DOCS,
  findDoc,
  renderDoc,
  resolveDocLink,
  resolveRepoPath,
  searchDocs,
  slugifyHeading,
} from '../DocsHelper';

describe('DocsHelper', () => {
  describe('slug and link resolution', () => {
    it('builds GitHub style heading slugs', () => {
      expect(slugifyHeading('Circuit breaker and health checks')).toBe(
        'circuit-breaker-and-health-checks',
      );
      expect(slugifyHeading('Replication event recording (503)!')).toBe(
        'replication-event-recording-503',
      );
    });

    it('has unique slugs and finds guides by slug', () => {
      const slugs = DOCS.map((d) => d.slug);

      expect(new Set(slugs).size).toBe(slugs.length);
      // "api" is the route of the API reference, not a guide
      expect(slugs).not.toContain('api');
      expect(findDoc('failover')?.path).toBe('docs/failover.md');
      expect(findDoc('nope')).toBeUndefined();
    });

    it('resolves paths relative to the document', () => {
      expect(resolveRepoPath('docs/tutti/api.md', './configuration.md')).toBe(
        'docs/tutti/configuration.md',
      );
      expect(resolveRepoPath('docs/tutti/api.md', '../dynamic-creds.md')).toBe(
        'docs/dynamic-creds.md',
      );
      expect(resolveRepoPath('docs/failover.md', './tutti/api.md')).toBe(
        'docs/tutti/api.md',
      );
    });

    it('keeps in-page anchors on the page', () => {
      expect(resolveDocLink('docs/tutti/api.md', '#policies')).toEqual({
        kind: 'anchor',
        hash: 'policies',
      });
    });

    it('opens bundled guides inside the page, with the section', () => {
      expect(
        resolveDocLink('docs/tutti/api.md', './configuration.md#status'),
      ).toEqual({ kind: 'doc', slug: 'configuration', hash: 'status' });
      expect(
        resolveDocLink(
          'docs/failover.md',
          './tutti/operations.md#automatic-failover',
        ),
      ).toEqual({
        kind: 'doc',
        slug: 'operations',
        hash: 'automatic-failover',
      });
      expect(
        resolveDocLink('docs/tutti/api.md', 'upgrading-from-chorus.md'),
      ).toEqual({
        kind: 'doc',
        slug: 'upgrading-from-chorus',
        hash: '',
      });
    });

    it('sends other repository files to GitHub', () => {
      expect(
        resolveDocLink(
          'docs/tutti/api.md',
          '../dynamic-creds.md#runtime-managed',
        ),
      ).toEqual({
        kind: 'external',
        href: 'https://github.com/hieunyit/tutti/blob/main/docs/dynamic-creds.md#runtime-managed',
      });
      expect(
        resolveDocLink(
          'docs/tutti/configuration.md',
          '../../service/proxy/config.yaml',
        ),
      ).toEqual({
        kind: 'external',
        href: 'https://github.com/hieunyit/tutti/blob/main/service/proxy/config.yaml',
      });
    });

    it('passes http(s) links and rejects script schemes', () => {
      expect(
        resolveDocLink('docs/failover.md', 'https://example.com/x'),
      ).toEqual({
        kind: 'external',
        href: 'https://example.com/x',
      });
      expect(
        resolveDocLink('docs/failover.md', 'javascript:alert(1)'),
      ).toBeNull();
      expect(resolveDocLink('docs/failover.md', 'data:text/html,x')).toBeNull();
      expect(resolveDocLink('docs/failover.md', '//evil.example/x')).toBeNull();
    });
  });

  describe('renderDoc', () => {
    const md = [
      '# Title',
      '',
      '## Same',
      '',
      '## Same',
      '',
      'See [config](./configuration.md#status), [top](#same) and [repo](../../ui).',
      '',
      '| A | B |',
      '|---|---|',
      '| 1 | 2 |',
      '',
      '```yaml',
      'failover:',
      '  enabled: true',
      '```',
    ].join('\n');

    it('renders headings with unique anchors and collects them', () => {
      const doc = renderDoc(md, 'docs/tutti/api.md');

      expect(doc.title).toBe('Title');
      expect(doc.headings.map((h) => h.id)).toEqual([
        'title',
        'same',
        'same-1',
      ]);
      expect(doc.html).toContain('<h2 id="same-1">');
      expect(doc.html).toContain('class="docs-anchor"');
    });

    it('rewrites links: in-page navigation and GitHub', () => {
      const { html } = renderDoc(md, 'docs/tutti/api.md');

      expect(html).toContain(
        'href="/docs/configuration#status" data-doc-slug="configuration" data-doc-hash="status"',
      );
      expect(html).toContain('href="#same" data-doc-hash="same"');
      expect(html).toContain(
        'href="https://github.com/hieunyit/tutti/blob/main/ui" target="_blank" rel="noopener noreferrer"',
      );
    });

    it('wraps tables and code blocks', () => {
      const { html } = renderDoc(md, 'docs/tutti/other.md');

      expect(html).toContain('<div class="docs-table-wrap"><table>');
      expect(html).toContain('<pre><code class="language-yaml">');
    });

    it('does not let raw HTML or script links through', () => {
      const { html } = renderDoc(
        '<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\n[bad](javascript:alert(1))',
        'docs/tutti/xss.md',
      );

      expect(html).not.toContain('<script');
      expect(html).not.toContain('<img');
      expect(html).not.toContain('<a ');
      expect(html).toContain('&lt;script&gt;');
    });
  });

  describe('searchDocs', () => {
    const titles = Object.fromEntries(DOCS.map((d) => [d.slug, `T ${d.slug}`]));
    const headings = {
      operations: [
        { id: 'automatic-failover', text: 'Automatic failover', level: 2 },
      ],
      configuration: [{ id: 'status', text: 'Status', level: 2 }],
    };

    it('lists every guide without a query', () => {
      expect(searchDocs('  ', titles, headings)).toHaveLength(DOCS.length);
    });

    it('matches titles and headings', () => {
      const byHeading = searchDocs('FAILOVER', titles, headings);

      expect(byHeading.map((r) => r.slug)).toEqual(['operations', 'failover']);
      expect(byHeading[0]?.headings).toHaveLength(1);
      expect(byHeading[1]?.titleMatch).toBe(true);
      expect(searchDocs('zzz', titles, headings)).toEqual([]);
    });
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
