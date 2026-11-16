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
import { AxiosError } from 'axios';
import { ReplicationRescanHelper as H } from '../ReplicationRescanHelper';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

const rep = (p: Partial<TuttiReplication>) =>
  ({ isPaused: false, isInitDone: true, ...p }) as TuttiReplication;

describe('ReplicationRescanHelper', () => {
  it('converts minutes to a Duration, off to nothing', () => {
    expect(H.toDuration(0)).toBeUndefined();
    expect(H.toDuration(15)).toBe('900s');
    expect(H.toDuration(1440)).toBe('86400s');
  });

  it('parses durations within 15m..24h only', () => {
    expect(H.parseSeconds('3600s')).toBe(3600);
    expect(H.parseSeconds('0s')).toBe(0);
    expect(H.parseSeconds(undefined)).toBe(0);
    expect(H.parseSeconds('60s')).toBe(0);
    expect(H.parseSeconds('abc')).toBe(0);
  });

  it('formats seconds as minutes or hours', () => {
    expect(H.formatSeconds(900, 'min', 'h')).toBe('15 min');
    expect(H.formatSeconds(3600, 'min', 'h')).toBe('1 h');
    expect(H.formatSeconds(5400, 'min', 'h')).toBe('90 min');
  });

  it('names why Sync now is blocked', () => {
    expect(H.blockedBy(rep({ isPaused: true }))).toBe('paused');
    expect(H.blockedBy(rep({ isInitDone: false }))).toBe('initial');
    expect(H.blockedBy(rep({ rescanRunning: true }))).toBe('running');
    expect(H.blockedBy(rep({}))).toBeNull();
  });

  it('shows time for today and day plus time otherwise', () => {
    const now = new Date(2026, 10, 16, 12, 0);
    const today = new Date(2026, 10, 16, 6, 10).toISOString();
    const older = new Date(2026, 10, 9, 6, 10).toISOString();

    expect(H.formatShort(today, 'en', now)).toBe('06:10');
    expect(H.formatShort(older, 'en', now)).toContain('06:10');
    expect(H.formatShort(older, 'en', now)).toContain('9');
    expect(H.formatShort(undefined, 'en', now)).toBe('');
    expect(H.formatShort('0001-01-01T00:00:00Z', 'en', now)).toBe('');
  });

  it('uses the server message without the gRPC code', () => {
    const error = new AxiosError('x', '409', undefined, undefined, {
      status: 409,
      data: { code: 9, message: 'FailedPrecondition: scan already running' },
    } as never);

    expect(H.errorText(error, 'fallback')).toBe('scan already running');
    expect(H.errorText(new Error('boom'), 'fallback')).toBe('fallback');
  });

  it('prefers the ErrorInfo reason over the bare code in message', () => {
    const error = new AxiosError('x', '400', undefined, undefined, {
      status: 400,
      data: {
        code: 9,
        message: 'FailedPrecondition',
        details: [
          { '@type': 'type.googleapis.com/google.rpc.RequestInfo' },
          {
            '@type': 'type.googleapis.com/google.rpc.ErrorInfo',
            reason:
              'FailedPrecondition: replication is paused, resume it before starting a sync',
          },
        ],
      },
    } as never);

    expect(H.errorText(error, 'fallback')).toBe(
      'replication is paused, resume it before starting a sync',
    );
  });

  it('falls back when only a bare code is known', () => {
    const error = new AxiosError('x', '400', undefined, undefined, {
      status: 400,
      data: { code: 9, message: 'FailedPrecondition', details: [] },
    } as never);

    expect(H.errorText(error, 'fallback')).toBe('fallback');
  });
});
