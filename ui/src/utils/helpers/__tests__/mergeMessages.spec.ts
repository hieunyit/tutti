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
import { mergeMessages } from '../mergeMessages';
import i18nReplicationHub from '@/pages/TuttiReplicationPage/i18nReplicationHub';
import i18nReplicationName from '@/components/tutti/replications/i18nReplicationName';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

describe('mergeMessages', () => {
  it('merges the keys of every set per locale', () => {
    const a = { en: { x: 'X', y: 'Y' }, de: { x: 'Xd' } };
    const b = { en: { z: 'Z' }, vi: { w: 'W' } };

    expect(mergeMessages(a, b)).toEqual({
      en: { x: 'X', y: 'Y', z: 'Z' },
      de: { x: 'Xd' },
      vi: { w: 'W' },
    });
  });

  it('keeps the replication list column title available (it rendered as the raw key before)', () => {
    const merged = mergeMessages(
      i18nReplicationHub,
      i18nReplicationName,
    ) as Record<string, Record<string, unknown>>;

    ['en', 'de', 'vi'].forEach((locale) => {
      expect(typeof merged[locale]?.nameColumn).toBe('string');
      expect(typeof merged[locale]?.pause).toBe('string');
    });
  });
});
