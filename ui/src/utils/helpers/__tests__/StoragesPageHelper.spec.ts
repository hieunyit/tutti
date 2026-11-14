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
  StoragesPageHelper,
  type StoragesSources,
} from '../StoragesPageHelper';
import type { TuttiReplication, TuttiStorage } from '@/utils/types/tuttiCore';
import type { TuttiProxyStatus } from '@/utils/types/tutti';
import type { S3User } from '@/utils/types/users';

const NOW = Date.parse('2026-11-10T13:05:00Z');

const storages = [
  { name: 'main', isMain: true, address: 'http://a:9000', provider: 'S3' },
  { name: 'follower', isMain: false, address: 'http://b:9000', provider: 'S3' },
] as TuttiStorage[];

function proxy(id: string, mainUp: boolean): TuttiProxyStatus {
  return {
    appId: id,
    updatedAt: '2026-11-10T13:04:58Z',
    storages: [
      {
        name: 'main',
        available: mainUp,
        consecutiveFailures: mainUp ? 0 : 14,
        lastError: mainUp ? '' : 'connection refused',
        lastChange: '2026-11-10T13:04:40Z',
      },
      { name: 'follower', available: true, consecutiveFailures: 0 },
    ],
    settings: { health: { failureThreshold: 3, openDuration: '30s' } },
    rates: {
      requestsPerSec: 4,
      status5xxPerSec: 0,
      requestsByStorage: { follower: 4 },
    },
  };
}

const users: S3User[] = [
  {
    name: 'user1',
    complete: true,
    storages: [
      { storage: 'main', configured: true },
      { storage: 'follower', configured: true },
    ],
  },
  {
    name: 'user3',
    complete: false,
    storages: [
      { storage: 'main', configured: true },
      { storage: 'follower', configured: false },
    ],
  },
];

const replication = (
  over: Partial<TuttiReplication> & { id: TuttiReplication['id'] },
) =>
  ({
    isArchived: false,
    isPaused: false,
    isInitDone: true,
    eventLag: '0',
    ...over,
  }) as TuttiReplication;

function sources(): StoragesSources {
  return {
    storages,
    proxies: [proxy('a', false), proxy('b', false)],
    users,
    replications: [
      replication({
        id: { user: 'user1', fromStorage: 'main', toStorage: 'follower' },
      }),
      replication({
        id: { user: 'user2', fromStorage: 'main', toStorage: 'follower' },
        isPaused: true,
        eventLag: '7',
      }),
    ],
    policies: [
      { source: 'main', enabled: true, targets: ['follower'], after: '30s' },
    ],
    events: [],
  };
}

describe('StoragesPageHelper', () => {
  it('derives worst health, breaker and traffic per storage', () => {
    const [main, follower] = StoragesPageHelper.rows(sources(), NOW);

    expect(main!.health.status).toBe('unavailable');
    expect(main!.health.unavailableOn).toBe(2);
    expect(main!.breaker).toMatchObject({ state: 'open', failures: 14 });
    // opened 20 s ago, breaker stays open 30 s
    expect(main!.breaker.retryInSeconds).toBe(10);
    expect(main!.requestsPerSecond).toBe(0);
    expect(follower!.health.status).toBe('available');
    expect(follower!.requestsPerSecond).toBe(8);
    // all traffic goes to the follower: the 5xx share is attributable
    expect(follower!.errorPercent).toBe(0);
    expect(main!.errorPercent).toBeNull();
  });

  it('counts replication and failover facts', () => {
    const [main, follower] = StoragesPageHelper.rows(sources(), NOW);

    expect(main!.replication).toMatchObject({
      sourceOf: 2,
      targetOf: 0,
      paused: 1,
    });
    expect(follower!.replication).toMatchObject({
      sourceOf: 0,
      targetOf: 2,
      lag: 7,
    });
    expect(main!.failover).toEqual({ target: 'follower', afterSeconds: 30 });
    expect(follower!.failover).toBeNull();
  });

  it('computes user coverage per storage and in the summary', () => {
    const rows = StoragesPageHelper.rows(sources(), NOW);

    expect(rows[1]!.users).toEqual({ count: 1, missing: ['user3'] });
    expect(rows[0]!.users).toEqual({ count: 2, missing: [] });

    const summary = StoragesPageHelper.summary(rows, sources(), NOW);

    expect(summary).toMatchObject({
      total: 2,
      available: 1,
      usersTotal: 2,
      usersComplete: 1,
      proxiesTotal: 2,
      proxiesReporting: 2,
    });
    expect(summary.worst?.name).toBe('main');
    expect(summary.firstIncomplete).toEqual({
      name: 'user3',
      missing: ['follower'],
    });
  });

  it('reports unknown health without proxies', () => {
    const rows = StoragesPageHelper.rows({ ...sources(), proxies: [] }, NOW);

    expect(rows[0]!.health.status).toBe('unknown');
    expect(rows[0]!.breaker.state).toBe('unknown');
  });
});
