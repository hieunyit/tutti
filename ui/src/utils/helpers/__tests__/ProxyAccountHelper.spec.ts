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

import { AxiosError } from 'axios';
import { describe, expect, it } from 'vitest';
import { ProxyAccountHelper } from '../ProxyAccountHelper';

function apiError(data: unknown) {
  return new AxiosError('fail', '400', undefined, undefined, {
    data,
    status: 400,
    statusText: 'Bad Request',
    headers: {},
    config: {} as never,
  });
}

describe('ProxyAccountHelper', () => {
  it('maps grace options to duration strings', () => {
    expect(ProxyAccountHelper.GRACE_OPTIONS.map((o) => o.value)).toEqual([
      '0s',
      '3600s',
      '86400s',
      '604800s',
    ]);
    expect(ProxyAccountHelper.getGraceValue('immediate')).toBe('0s');
    expect(ProxyAccountHelper.getGraceValue('week')).toBe('604800s');
    expect(ProxyAccountHelper.getGraceValue('bogus')).toBe('3600s');
    expect(
      ProxyAccountHelper.getGraceValue(ProxyAccountHelper.DEFAULT_GRACE_ID),
    ).toBe('3600s');
  });

  it('detects FailedPrecondition', () => {
    expect(ProxyAccountHelper.isNotConfigured(apiError({ code: 9 }))).toBe(
      true,
    );
    expect(ProxyAccountHelper.isNotConfigured(apiError({ code: 3 }))).toBe(
      false,
    );
    expect(ProxyAccountHelper.isNotConfigured(new Error('x'))).toBe(false);
    expect(ProxyAccountHelper.getErrorCode(apiError({}))).toBeNull();
  });

  it('extracts backend messages', () => {
    expect(
      ProxyAccountHelper.getMessage(apiError({ message: 'conflict' })),
    ).toBe('conflict');
    expect(ProxyAccountHelper.getMessage(apiError({}))).toBeNull();
    expect(ProxyAccountHelper.getMessage('x')).toBeNull();
  });

  it('separates static from dynamic keys', () => {
    const keys = ProxyAccountHelper.getStaticKeys({
      user: 'u',
      accessKeyIds: ['A', 'B', 'C'],
      sources: ['custom', 'dynamic', 'storage:s3'],
      dynamicKeys: [{ accessKeyId: 'B', status: 'active' }],
    });

    expect(keys).toEqual([
      { accessKeyId: 'A', source: 'custom' },
      { accessKeyId: 'C', source: 'storage:s3' },
    ]);
  });

  it('checks timestamps', () => {
    expect(ProxyAccountHelper.hasTime(undefined)).toBe(false);
    expect(ProxyAccountHelper.hasTime('0001-01-01T00:00:00Z')).toBe(false);
    expect(ProxyAccountHelper.hasTime('2026-01-01T00:00:00Z')).toBe(true);
  });
});
