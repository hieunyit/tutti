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
