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

import MarkdownIt from 'markdown-it';
import type Token from 'markdown-it/lib/token.mjs';

export const GITHUB_BLOB_URL = 'https://github.com/hieunyit/tutti/blob/main/';

export interface DocEntry {
  slug: string;
  /** path in the repository */
  path: string;
}

/** Guides bundled with the UI, in navigation order. */
export const DOCS: DocEntry[] = [
  { slug: 'deployment-guide', path: 'docs/tutti/deployment-guide.md' },
  { slug: 'operations', path: 'docs/tutti/operations.md' },
  { slug: 'configuration', path: 'docs/tutti/configuration.md' },
  { slug: 'storage-accounts', path: 'docs/tutti/storage-accounts.md' },
  { slug: 'resilience-api', path: 'docs/tutti/api.md' },
  {
    slug: 'upgrading-from-chorus',
    path: 'docs/tutti/upgrading-from-chorus.md',
  },
  { slug: 'enterprise-ui', path: 'docs/tutti/ui-enterprise.md' },
  { slug: 'failover', path: 'docs/failover.md' },
];

export interface DocHeading {
  id: string;
  text: string;
  level: number;
}

export interface RenderedDoc {
  html: string;
  headings: DocHeading[];
  /** text of the first level 1 heading */
  title: string;
}

export type ResolvedLink =
  | { kind: 'anchor'; hash: string }
  | { kind: 'doc'; slug: string; hash: string }
  | { kind: 'external'; href: string };

export function findDoc(slug: string | null | undefined): DocEntry | undefined {
  return DOCS.find((doc) => doc.slug === slug);
}

/** GitHub style heading ids: lower case, punctuation dropped, spaces to "-". */
export function slugifyHeading(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s_-]/gu, '')
    .replace(/\s/g, '-');
}

/** Repository path of `href` relative to the document at `fromPath`. */
export function resolveRepoPath(fromPath: string, href: string): string {
  const parts = fromPath.split('/').slice(0, -1);

  href.split('/').forEach((part) => {
    if (part === '..') parts.pop();
    else if (part !== '.' && part !== '') parts.push(part);
  });

  return parts.join('/');
}

/**
 * Where a link of a markdown document points:
 * - "#x" stays on the page
 * - a bundled guide becomes an in-app route
 * - other repository files go to GitHub
 * Returns null for schemes that must not be rendered as links.
 */
export function resolveDocLink(
  fromPath: string,
  href: string,
): ResolvedLink | null {
  const target = href.trim();

  if (target.startsWith('#')) {
    return { kind: 'anchor', hash: target.slice(1) };
  }

  if (/^https?:\/\//i.test(target) || /^mailto:/i.test(target)) {
    return { kind: 'external', href: target };
  }

  if (/^[a-z][a-z0-9+.-]*:/i.test(target) || target.startsWith('//')) {
    return null;
  }

  const [file = '', hash = ''] = target.split('#');
  const path = resolveRepoPath(fromPath, file);
  const doc = DOCS.find((d) => d.path === path);

  if (doc) return { kind: 'doc', slug: doc.slug, hash };

  return {
    kind: 'external',
    href: `${GITHUB_BLOB_URL}${path}${hash ? `#${hash}` : ''}`,
  };
}

const md = new MarkdownIt({ html: false, linkify: false, typographer: false });

function renderLinkOpen(
  tokens: Token[],
  idx: number,
  env: { path: string },
): string {
  const token = tokens[idx]!;
  const href = token.attrGet('href') ?? '';
  const link = resolveDocLink(env.path, href);

  if (!link) {
    // unsafe scheme: keep the text, drop the link
    token.attrSet('href', '#');
    token.attrSet('role', 'presentation');

    return md.renderer.renderToken(tokens, idx, md.options);
  }

  if (link.kind === 'anchor') {
    token.attrSet('href', `#${link.hash}`);
    token.attrSet('data-doc-hash', link.hash);
  } else if (link.kind === 'doc') {
    token.attrSet(
      'href',
      `/docs/${link.slug}${link.hash ? `#${link.hash}` : ''}`,
    );
    token.attrSet('data-doc-slug', link.slug);
    token.attrSet('data-doc-hash', link.hash);
  } else {
    token.attrSet('href', link.href);
    token.attrSet('target', '_blank');
    token.attrSet('rel', 'noopener noreferrer');
  }

  return md.renderer.renderToken(tokens, idx, md.options);
}

md.renderer.rules.link_open = (tokens, idx, _options, env) =>
  renderLinkOpen(tokens, idx, env as { path: string });

// wide tables scroll inside the article
md.renderer.rules.table_open = () => '<div class="docs-table-wrap"><table>\n';
md.renderer.rules.table_close = () => '</table></div>\n';

// images are not bundled: show their alt text
md.renderer.rules.image = (tokens, idx) =>
  md.utils.escapeHtml(tokens[idx]?.content ?? '');

/** Adds ids to headings and collects them. */
md.core.ruler.push('docs_headings', (state) => {
  const headings: DocHeading[] = [];
  const used = new Map<string, number>();

  state.tokens.forEach((token, i) => {
    if (token.type !== 'heading_open') return;

    const text = (state.tokens[i + 1]?.children ?? [])
      .filter((c) => c.type === 'text' || c.type === 'code_inline')
      .map((c) => c.content)
      .join('');
    const base = slugifyHeading(text) || 'section';
    const count = used.get(base) ?? 0;
    const id = count === 0 ? base : `${base}-${count}`;

    used.set(base, count + 1);
    token.attrSet('id', id);
    headings.push({ id, text, level: Number(token.tag.slice(1)) });
  });
  (state.env as { headings: DocHeading[] }).headings = headings;
});

md.renderer.rules.heading_close = (tokens, idx) => {
  const open = tokens
    .slice(0, idx)
    .reverse()
    .find((t) => t.type === 'heading_open');
  const id = open?.attrGet('id') ?? '';

  return `<a class="docs-anchor" href="#${md.utils.escapeHtml(id)}" data-doc-hash="${md.utils.escapeHtml(id)}" aria-label="Link to this section">#</a></${tokens[idx]?.tag}>\n`;
};

const cache = new Map<string, RenderedDoc>();

/** Renders a guide to sanitised HTML (raw HTML in markdown is escaped). */
export function renderDoc(markdown: string, path: string): RenderedDoc {
  const cached = cache.get(path);

  if (cached) return cached;

  const env: { path: string; headings: DocHeading[] } = {
    path,
    headings: [],
  };
  const html = md.render(markdown, env);
  const rendered = {
    html,
    headings: env.headings,
    title: env.headings.find((h) => h.level === 1)?.text ?? '',
  };

  cache.set(path, rendered);

  return rendered;
}

/** Headings (below level 1) of a rendered doc, for navigation and search. */
export function sectionHeadings(doc: RenderedDoc): DocHeading[] {
  return doc.headings.filter((h) => h.level > 1);
}

export interface DocSearchResult {
  slug: string;
  titleMatch: boolean;
  headings: DocHeading[];
}

/**
 * Filters guides by a query: a guide matches by its title or by a heading.
 * Without a query every guide matches with no headings listed.
 */
export function searchDocs(
  query: string,
  titles: Record<string, string>,
  headings: Record<string, DocHeading[]>,
): DocSearchResult[] {
  const q = query.trim().toLowerCase();

  return DOCS.flatMap((doc) => {
    if (!q) return [{ slug: doc.slug, titleMatch: true, headings: [] }];

    const titleMatch = (titles[doc.slug] ?? '').toLowerCase().includes(q);
    const matched = (headings[doc.slug] ?? []).filter((h) =>
      h.text.toLowerCase().includes(q),
    );

    return titleMatch || matched.length > 0
      ? [{ slug: doc.slug, titleMatch, headings: matched }]
      : [];
  });
}
