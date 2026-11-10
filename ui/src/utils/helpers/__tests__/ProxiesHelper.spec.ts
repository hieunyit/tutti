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
import { ProxiesHelper } from '../ProxiesHelper';
import type { TuttiProxyStatus } from '@/utils/types/tutti';

const NOW = Date.parse('2026-11-10T12:00:00Z');

function proxy(
  id: string,
  patch: Partial<TuttiProxyStatus> = {},
): TuttiProxyStatus {
  return {
    appId: id,
    updatedAt: '2026-11-10T11:59:58Z',
    settings: {
      failover: { enabled: true },
      replication: { sync: { enabled: true, writeQuorum: 0 } },
      health: { failureThreshold: 3, openDuration: '30s' },
    },
    ...patch,
  };
}

describe('ProxiesHelper.settingsDrift', () => {
  it('is empty with identical settings or a single proxy', () => {
    expect(ProxiesHelper.settingsDrift([proxy('a'), proxy('b')], NOW)).toEqual(
      [],
    );
    expect(ProxiesHelper.settingsDrift([proxy('a')], NOW)).toEqual([]);
  });

  it('detects failover and quorum differences and names the outlier', () => {
    const b = proxy('b', {
      settings: {
        failover: { enabled: false },
        replication: { sync: { enabled: true, writeQuorum: 1 } },
        health: { failureThreshold: 3, openDuration: '30s' },
      },
    });
    const drift = ProxiesHelper.settingsDrift([proxy('a'), proxy('c'), b], NOW);

    expect(drift.map((d) => d.field)).toEqual([
      'failoverEnabled',
      'syncQuorum',
    ]);
    expect(drift[0]!.groups[0]).toEqual({ value: 'on', proxies: ['a', 'c'] });
    expect([...ProxiesHelper.outliers(drift)]).toEqual(['b']);
  });

  it('detects health threshold drift and ignores stale proxies', () => {
    const b = proxy('b', {
      settings: {
        failover: { enabled: true },
        replication: { sync: { enabled: true, writeQuorum: 0 } },
        health: { failureThreshold: 5, openDuration: '30s' },
      },
    });
    const stale = proxy('s', { updatedAt: '2026-11-10T10:00:00Z' });

    expect(ProxiesHelper.settingsDrift([proxy('a'), b], NOW)[0]!.field).toBe(
      'healthThreshold',
    );
    expect(ProxiesHelper.settingsDrift([proxy('a'), stale], NOW)).toEqual([]);
  });
});

describe('ProxiesHelper.uptime', () => {
  it('formats durations', () => {
    expect(ProxiesHelper.uptime('2026-11-07T08:00:00Z', NOW)).toBe('3 d 4 h');
    expect(ProxiesHelper.uptime('2026-11-10T11:48:00Z', NOW)).toBe('12 min');
    expect(ProxiesHelper.uptime(undefined, NOW)).toBeNull();
  });
});

describe('ProxiesHelper.effectiveGroup & configSyncStatus', () => {
  it('defaults empty group to default', () => {
    expect(ProxiesHelper.effectiveGroup(proxy('a'))).toBe('default');
    expect(ProxiesHelper.effectiveGroup(proxy('b', { group: 'canary' }))).toBe(
      'canary',
    );
  });

  it('computes configSyncStatus', () => {
    expect(ProxiesHelper.configSyncStatus(proxy('a'), 0)).toEqual({
      state: 'unconfigured',
      targetVersion: 0,
      appliedVersion: 0,
    });
    expect(
      ProxiesHelper.configSyncStatus(
        proxy('a', { appliedConfigVersion: 2 }),
        2,
      ),
    ).toEqual({
      state: 'applied',
      targetVersion: 2,
      appliedVersion: 2,
    });
    expect(
      ProxiesHelper.configSyncStatus(
        proxy('a', { appliedConfigVersion: 1 }),
        2,
      ),
    ).toEqual({
      state: 'applying',
      targetVersion: 2,
      appliedVersion: 1,
    });
  });
});

describe('ProxiesHelper.isFleetConflict', () => {
  it('detects a FailedPrecondition (code 9) fleet config conflict', () => {
    expect(
      ProxiesHelper.isFleetConflict({ response: { data: { code: 9 } } }),
    ).toBe(true);
  });

  it('ignores other errors', () => {
    expect(
      ProxiesHelper.isFleetConflict({ response: { data: { code: 3 } } }),
    ).toBe(false);
    expect(ProxiesHelper.isFleetConflict(new Error('x'))).toBe(false);
    expect(ProxiesHelper.isFleetConflict(null)).toBe(false);
  });
});
