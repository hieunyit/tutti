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
import { AuthHelper } from '../AuthHelper';

describe('AuthHelper', () => {
  it('accepts same-origin paths only as redirect target', () => {
    expect(AuthHelper.getSafeRedirect('/replication?x=1')).toBe(
      '/replication?x=1',
    );
    expect(AuthHelper.getSafeRedirect(['/home'])).toBe('/home');
    expect(AuthHelper.getSafeRedirect('//evil.example')).toBeNull();
    expect(AuthHelper.getSafeRedirect('https://evil.example')).toBeNull();
    expect(AuthHelper.getSafeRedirect('/login')).toBeNull();
    expect(AuthHelper.getSafeRedirect(undefined)).toBeNull();
  });

  it('formats Retry-After rounded up', () => {
    const fmt = {
      seconds: (n: number) => `${n}s`,
      minutes: (n: number) => `${n}m`,
    };

    expect(AuthHelper.formatRetryAfter(0.2, fmt)).toBe('1s');
    expect(AuthHelper.formatRetryAfter(42, fmt)).toBe('42s');
    expect(AuthHelper.formatRetryAfter(61, fmt)).toBe('2m');
  });
});
