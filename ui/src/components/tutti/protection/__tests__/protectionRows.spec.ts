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
import {
  coversText,
  matchesQuery,
  storagesText,
  toProtectionRow,
} from '../protectionRows';

const t = (key: string, values?: Record<string, unknown>) =>
  values ? `${key}:${JSON.stringify(values)}` : key;

describe('protection rows', () => {
  it('prints the primary first, then the other members', () => {
    expect(
      storagesText({ id: 'g', members: ['hcm04', 'hcm02'], primary: 'hcm02' }),
    ).toBe('hcm02 → hcm04');
    expect(storagesText({ id: 'g' })).toBe('–');
  });

  it('summarises the scope', () => {
    expect(coversText({ id: 'g', buckets: ['a'] }, t)).toBe('scopeBucketsOne');
    expect(coversText({ id: 'g', buckets: ['a', 'b'] }, t)).toContain(
      '"count":2',
    );
    expect(coversText({ id: 'g', user: 'alice' }, t)).toContain('alice');
    expect(coversText({ id: 'g' }, t)).toBe('scopeStorage');
  });

  it('builds a row and matches a text query', () => {
    const row = toProtectionRow(
      { id: 'gold', members: ['a', 'b'], primary: 'a', state: 'ready' },
      { healthyCopies: 2 },
      t,
    );

    expect(row.display).toBe('ready');
    expect(row.tone).toBe('success');
    expect(matchesQuery(row, 'GOLD')).toBe(true);
    expect(matchesQuery(row, 'a → b')).toBe(true);
    expect(matchesQuery(row, 'nope')).toBe(false);
  });
});
