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
import { FailoverHelper } from '../FailoverHelper';
import type { ChorusReplication } from '@/utils/types/chorus';
import {
  TuttiPolicyValidationError as E,
  type TuttiFailoverPolicy,
  type TuttiFailoverPolicyForm,
  type TuttiProxyStatus,
} from '@/utils/types/tutti';

const form = (
  over: Partial<TuttiFailoverPolicyForm> = {},
): TuttiFailoverPolicyForm => ({
  source: 'main',
  enabled: true,
  targets: ['b', 'c'],
  after: '60',
  maxEventLag: null,
  ...over,
});

describe('FailoverHelper durations', () => {
  it('parses seconds, units and rejects garbage', () => {
    expect(FailoverHelper.parseDurationSeconds('60')).toBe(60);
    expect(FailoverHelper.parseDurationSeconds('60s')).toBe(60);
    expect(FailoverHelper.parseDurationSeconds('5m')).toBe(300);
    expect(FailoverHelper.parseDurationSeconds('1h')).toBe(3600);
    expect(FailoverHelper.parseDurationSeconds('1.5s')).toBe(1.5);
    expect(FailoverHelper.parseDurationSeconds('-3')).toBeNull();
    expect(FailoverHelper.parseDurationSeconds('abc')).toBeNull();
    expect(FailoverHelper.toProtoDuration(60)).toBe('60s');
    expect(FailoverHelper.fromProtoDuration('90s')).toBe('90');
    expect(FailoverHelper.fromProtoDuration(undefined)).toBe('');
  });
});

describe('FailoverHelper target list', () => {
  it('excludes source and duplicates from available targets', () => {
    expect(
      FailoverHelper.availableTargets(['main', 'b', 'c', 'd'], 'main', ['b']),
    ).toEqual(['c', 'd']);
  });

  it('adds without duplicates and never the source', () => {
    expect(FailoverHelper.addTarget(['b'], 'c', 'main')).toEqual(['b', 'c']);
    expect(FailoverHelper.addTarget(['b'], 'b', 'main')).toEqual(['b']);
    expect(FailoverHelper.addTarget(['b'], 'main', 'main')).toEqual(['b']);
  });

  it('removes and moves targets keeping order', () => {
    expect(FailoverHelper.removeTarget(['a', 'b', 'c'], 'b')).toEqual([
      'a',
      'c',
    ]);
    expect(FailoverHelper.moveTarget(['a', 'b', 'c'], 1, -1)).toEqual([
      'b',
      'a',
      'c',
    ]);
    expect(FailoverHelper.moveTarget(['a', 'b', 'c'], 1, 1)).toEqual([
      'a',
      'c',
      'b',
    ]);
    expect(FailoverHelper.moveTarget(['a', 'b'], 0, -1)).toEqual(['a', 'b']);
    expect(FailoverHelper.moveTarget(['a', 'b'], 1, 1)).toEqual(['a', 'b']);
  });

  it('does not mutate its input', () => {
    const list = ['a', 'b'];

    FailoverHelper.moveTarget(list, 0, 1);
    expect(list).toEqual(['a', 'b']);
  });
});

describe('FailoverHelper validation', () => {
  it('accepts a valid policy', () => {
    expect(FailoverHelper.validate(form())).toEqual([]);
  });

  it('requires a source', () => {
    expect(FailoverHelper.validate(form({ source: null }))).toContain(
      E.SOURCE_REQUIRED,
    );
  });

  it('enabled requires targets, disabled does not', () => {
    expect(FailoverHelper.validate(form({ targets: [] }))).toContain(
      E.TARGETS_REQUIRED,
    );
    expect(
      FailoverHelper.validate(form({ targets: [], enabled: false })),
    ).toEqual([]);
  });

  it('rejects target equal to source and duplicates', () => {
    expect(FailoverHelper.validate(form({ targets: ['main'] }))).toContain(
      E.TARGET_IS_SOURCE,
    );
    expect(FailoverHelper.validate(form({ targets: ['b', 'b'] }))).toContain(
      E.TARGET_DUPLICATE,
    );
  });

  it('rejects negative or malformed numbers, allows empty', () => {
    expect(FailoverHelper.validate(form({ after: '-1' }))).toContain(
      E.AFTER_INVALID,
    );
    expect(FailoverHelper.validate(form({ after: '' }))).toEqual([]);
    expect(FailoverHelper.validate(form({ maxEventLag: -1 }))).toContain(
      E.MAX_EVENT_LAG_INVALID,
    );
    expect(FailoverHelper.validate(form({ maxEventLag: 1.5 }))).toContain(
      E.MAX_EVENT_LAG_INVALID,
    );
    expect(FailoverHelper.validate(form({ maxEventLag: 0 }))).toEqual([]);
  });
});

describe('FailoverHelper form <-> policy', () => {
  it('converts a form to a policy with proto duration', () => {
    expect(
      FailoverHelper.formToPolicy(form({ after: '5m', maxEventLag: 3 })),
    ).toEqual({
      source: 'main',
      enabled: true,
      targets: ['b', 'c'],
      after: '300s',
      maxEventLag: 3,
    });
  });

  it('omits after/maxEventLag when empty', () => {
    const policy = FailoverHelper.formToPolicy(form({ after: '' }));

    expect(policy).not.toHaveProperty('after');
    expect(policy).not.toHaveProperty('maxEventLag');
  });

  it('round-trips a policy', () => {
    const form1 = FailoverHelper.policyToForm({
      source: 's',
      targets: ['x'],
      after: '30s',
      maxEventLag: 0,
    });

    expect(form1).toEqual({
      source: 's',
      enabled: false,
      targets: ['x'],
      after: '30',
      maxEventLag: 0,
    });
  });
});

describe('FailoverHelper readiness and trigger evaluation', () => {
  const NOW = Date.parse('2026-11-10T10:00:00Z');
  const proxy = (enabled: boolean, appId = 'p1'): TuttiProxyStatus => ({
    appId,
    address: 'http://p',
    updatedAt: '2026-11-10T09:59:55Z',
    settings: { failover: { enabled, checkInterval: '5s', maxEventLag: 0 } },
  });
  const policy = (over: Partial<TuttiFailoverPolicy> = {}) => ({
    source: 'main',
    enabled: true,
    targets: ['follower'],
    ...over,
  });
  const repl = (over: Record<string, unknown> = {}) =>
    ({
      id: { user: 'u', fromStorage: 'main', toStorage: 'follower' },
      isPaused: false,
      isArchived: false,
      eventLag: '0',
      ...over,
    }) as unknown as ChorusReplication;

  it('says no when a reporting proxy has failover off', () => {
    expect(
      FailoverHelper.evaluatePolicy(
        policy(),
        [proxy(true), proxy(false, 'p2')],
        [repl()],
        NOW,
      ),
    ).toEqual({ state: 'no', reason: 'proxiesOff' });
  });

  it('says no when target replication is paused or lagging', () => {
    expect(
      FailoverHelper.evaluatePolicy(
        policy(),
        [proxy(true)],
        [repl({ isPaused: true })],
        NOW,
      ),
    ).toEqual({ state: 'no', reason: 'replication' });
    expect(
      FailoverHelper.evaluatePolicy(
        policy({ maxEventLag: 2 }),
        [proxy(true)],
        [repl({ eventLag: '5' })],
        NOW,
      ),
    ).toEqual({ state: 'no', reason: 'replication' });
    expect(
      FailoverHelper.evaluatePolicy(
        policy({ maxEventLag: 5 }),
        [proxy(true)],
        [repl({ eventLag: '5' })],
        NOW,
      ),
    ).toEqual({ state: 'yes' });
  });

  it('uses the proxy default lag when the policy has none', () => {
    expect(
      FailoverHelper.evaluatePolicy(
        policy(),
        [proxy(true)],
        [repl({ eventLag: '1' })],
        NOW,
      ),
    ).toEqual({ state: 'no', reason: 'replication' });
  });

  it('says yes with a healthy replication, disabled for disabled policies', () => {
    expect(
      FailoverHelper.evaluatePolicy(policy(), [proxy(true)], [repl()], NOW),
    ).toEqual({ state: 'yes' });
    expect(
      FailoverHelper.evaluatePolicy(
        policy({ enabled: false }),
        [proxy(false)],
        [],
        NOW,
      ),
    ).toEqual({ state: 'disabled' });
    expect(
      FailoverHelper.evaluatePolicy(policy(), [proxy(true)], null, NOW),
    ).toEqual({ state: 'yes' });
  });

  it('builds the readiness row and the banner flag', () => {
    const r = FailoverHelper.getReadiness(
      [policy(), policy({ source: 'x', enabled: false })],
      [proxy(true), proxy(false, 'p2')],
      [repl({ isPaused: true })],
      NOW,
    );

    expect(r.total).toBe(2);
    expect(r.on).toBe(1);
    expect(r.off.map((p) => p.appId)).toEqual(['p2']);
    expect(r.enabledPolicies).toBe(1);
    expect(r.replication).toBe('paused');
    expect(r.willNotTrigger).toBe(true);
  });

  it('shows no banner without enabled policies or when all proxies are on', () => {
    expect(
      FailoverHelper.getReadiness([], [proxy(false)], [], NOW).willNotTrigger,
    ).toBe(false);
    expect(
      FailoverHelper.getReadiness([policy()], [proxy(true)], [repl()], NOW)
        .willNotTrigger,
    ).toBe(false);
    expect(
      FailoverHelper.getReadiness([policy()], [proxy(true)], [repl()], NOW)
        .replication,
    ).toBe('inSync');
  });
});
