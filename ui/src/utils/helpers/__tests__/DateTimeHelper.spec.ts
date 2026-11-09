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

import { describe, expect, it, vi } from 'vitest';
import {
  EMPTY_DATE_TEXT,
  formatDateTime,
  formatRelative,
  formatTime,
  parseDateTime,
} from '../DateTimeHelper';

vi.mock('@/i18n', () => ({ i18n: { global: { locale: { value: 'en' } } } }));

describe('DateTimeHelper', () => {
  const local = new Date(2026, 10, 9, 13, 15, 37);

  it('formats absolute date time for EN with 24h time', () => {
    expect(formatDateTime(local, 'en')).toBe('9 Nov 2026, 13:15:37');
  });

  it('formats absolute date time for DE', () => {
    expect(formatDateTime(local, 'de')).toMatch(
      /^9\.\s?Nov\.?\s?2026,?\s13:15:37$/,
    );
  });

  it('formats time only', () => {
    expect(formatTime(local, 'en')).toBe('13:15:37');
  });

  it.each([null, undefined, '', 'garbage', '0001-01-01T00:00:00Z', 0])(
    'returns the placeholder for %s',
    (value) => {
      expect(formatDateTime(value, 'en')).toBe(EMPTY_DATE_TEXT);
      expect(formatRelative(value, 'en')).toBe(EMPTY_DATE_TEXT);
      expect(formatTime(value, 'en')).toBe(EMPTY_DATE_TEXT);
      expect(parseDateTime(value)).toBeNull();
    },
  );

  it('accepts ISO strings and timestamps', () => {
    expect(parseDateTime('2026-11-09T13:15:37Z')).toBeInstanceOf(Date);
    expect(parseDateTime(local.getTime())).toBeInstanceOf(Date);
  });

  it('formats relative past and future times', () => {
    const now = local.getTime();

    expect(formatRelative(now - 3 * 60_000, 'en', now)).toBe('3 minutes ago');
    expect(formatRelative(now - 2 * 3600_000, 'en', now)).toBe('2 hours ago');
    expect(formatRelative(now + 5 * 3600_000, 'en', now)).toBe('in 5 hours');
    expect(formatRelative(now - 3 * 86400_000, 'en', now)).toBe('3 days ago');
  });

  it('formats relative times in German', () => {
    const now = local.getTime();

    expect(formatRelative(now - 3 * 60_000, 'de', now)).toBe('vor 3 Minuten');
  });
});
