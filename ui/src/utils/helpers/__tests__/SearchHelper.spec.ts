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
import { filterSearchItems } from '../SearchHelper';

const entries = [
  { item: 'home', label: 'Dashboard', group: 'Overview' },
  { item: 'mon', label: 'Monitoring', group: 'Overview' },
  { item: 'stor', label: 'Storages', group: 'Storage' },
  { item: 'health', label: 'Storage health', group: 'Storage' },
  { item: 'fail', label: 'Failover', group: 'Data protection' },
];

describe('filterSearchItems', () => {
  it('returns everything for an empty query', () => {
    expect(filterSearchItems('  ', entries)).toHaveLength(5);
  });

  it('matches case-insensitively on the name', () => {
    expect(filterSearchItems('FAIL', entries).map((e) => e.item)).toEqual([
      'fail',
    ]);
  });

  it('requires every word and ranks name matches before group matches', () => {
    expect(filterSearchItems('storage', entries).map((e) => e.item)).toEqual([
      'stor',
      'health',
    ]);
    expect(
      filterSearchItems('storage hea', entries).map((e) => e.item),
    ).toEqual(['health']);
    expect(filterSearchItems('overview', entries).map((e) => e.item)).toEqual([
      'home',
      'mon',
    ]);
  });

  it('finds nothing for unknown text', () => {
    expect(filterSearchItems('zzz', entries)).toEqual([]);
  });
});
