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
import en from '../protectionMessages.en';
import de from '../protectionMessages.de';
import vi from '../protectionMessages.vi';

describe('create failure dialog copy', () => {
  it.each([
    ['en', en],
    ['de', de],
    ['vi', vi],
  ])('says different things in title and body (%s)', (_l, m) => {
    const body = (m.createFailedBody ?? '').replace('{message}', 'reason');

    expect(m.wizSaveFailed).not.toBe(body);
    expect(body).toContain('reason');
    expect(body).not.toContain(m.wizSaveFailed ?? '');
  });

  it('no longer says "not created" and "not saved" together', () => {
    expect(`${en.wizSaveFailed}${en.createFailedBody}`).not.toMatch(
      /not created.*not saved|not saved.*not created/i,
    );
  });
});
