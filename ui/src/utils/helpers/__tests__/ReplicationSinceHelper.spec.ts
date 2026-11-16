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
  ReplicationSinceHelper as H,
  type SinceChoice,
} from '../ReplicationSinceHelper';

const NOW = Date.UTC(2026, 9, 17, 14, 3, 27, 500);
const DAY = 24 * 60 * 60 * 1000;

function choice(partial: Partial<SinceChoice>): SinceChoice {
  return { ...H.defaultChoice(), ...partial };
}

describe('ReplicationSinceHelper', () => {
  it('sends nothing for all objects', () => {
    expect(H.resolve(choice({ mode: 'all' }), NOW)).toBeUndefined();
    expect(H.getProblem(choice({ mode: 'all', amount: -4 }), NOW)).toBeNull();
  });

  it('turns relative days and hours into an absolute UTC timestamp', () => {
    expect(
      H.resolve(choice({ mode: 'recent', amount: 30, unit: 'days' }), NOW),
    ).toBe('2026-09-17T14:03:27Z');
    expect(
      H.resolve(choice({ mode: 'recent', amount: 12, unit: 'hours' }), NOW),
    ).toBe('2026-10-17T02:03:27Z');
  });

  it('counts back from the moment of the call, not from typing time', () => {
    const c = choice({ mode: 'recent', amount: 1, unit: 'days' });

    expect(H.resolve(c, NOW)).not.toBe(H.resolve(c, NOW + 3600_000));
  });

  it('passes a picked date through', () => {
    const date = Date.UTC(2026, 0, 5, 8, 0, 0);

    expect(H.resolve(choice({ mode: 'date', date }), NOW)).toBe(
      '2026-01-05T08:00:00Z',
    );
  });

  it.each([null, 0, -1, 1.5, Number.NaN])('rejects amount %s', (amount) => {
    expect(H.getProblem(choice({ mode: 'recent', amount }), NOW)).toBe(
      'amount',
    );
    expect(H.resolve(choice({ mode: 'recent', amount }), NOW)).toBeUndefined();
  });

  it('asks for a date when none is picked', () => {
    expect(H.getProblem(choice({ mode: 'date', date: null }), NOW)).toBe(
      'dateMissing',
    );
  });

  it('rejects a future date and accepts "now"', () => {
    expect(H.getProblem(choice({ mode: 'date', date: NOW + 1 }), NOW)).toBe(
      'future',
    );
    expect(H.getProblem(choice({ mode: 'date', date: NOW }), NOW)).toBeNull();
  });

  it('rejects anything before year 2000, accepts 2000-01-01', () => {
    expect(
      H.getProblem(
        choice({ mode: 'date', date: Date.UTC(1999, 11, 31, 23, 59) }),
        NOW,
      ),
    ).toBe('tooOld');
    expect(
      H.getProblem(choice({ mode: 'date', date: Date.UTC(2000, 0, 1) }), NOW),
    ).toBeNull();
    expect(
      H.getProblem(
        choice({ mode: 'recent', amount: 40000, unit: 'days' }),
        NOW,
      ),
    ).toBe('tooOld');
  });

  it('accepts one day when it is in range', () => {
    expect(
      H.getProblem(choice({ mode: 'recent', amount: 1, unit: 'days' }), NOW),
    ).toBeNull();
    expect(H.momentOf(choice({ mode: 'recent', amount: 1 }), NOW)).toBe(
      NOW - DAY,
    );
  });

  it('formats in the user locale and time zone', () => {
    const date = new Date(2026, 9, 17, 14, 3);

    expect(H.formatDay(date, 'en')).toBe('17 Oct 2026');
    expect(H.formatMoment(date, 'en')).toContain('17 Oct 2026');
    expect(H.formatMoment(date, 'en')).toContain('14:03');
    expect(H.formatDay(date, 'de')).toMatch(/17\. Okt\.? 2026/);
  });

  it('reads the stored cutoff and ignores empty or zero values', () => {
    expect(H.parseStored(undefined)).toBeNull();
    expect(H.parseStored('')).toBeNull();
    expect(H.parseStored('0001-01-01T00:00:00Z')).toBeNull();
    expect(H.parseStored('nope')).toBeNull();
    expect(H.parseStored('2026-10-17T14:03:00Z')?.getTime()).toBe(
      Date.UTC(2026, 9, 17, 14, 3),
    );
  });
});
