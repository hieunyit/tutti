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
import { ProtectionGroupHelper } from '../ProtectionGroupHelper';
import type {
  ProtectionGroup,
  ProtectionGroupForm,
  ProtectionGroupStatus,
} from '@/utils/types/protection';

const baseGroup = (over: Partial<ProtectionGroup> = {}): ProtectionGroup => ({
  id: 'pg-1',
  user: 'alice',
  buckets: [],
  members: ['s1', 's2', 's3'],
  copies: 2,
  mode: 'manual',
  primary: 's1',
  state: 'ready',
  revision: '7',
  ...over,
});

const baseForm = (
  over: Partial<ProtectionGroupForm> = {},
): ProtectionGroupForm => ({
  ...ProtectionGroupHelper.emptyForm(),
  id: 'pg-1',
  user: 'alice',
  members: ['s1', 's2'],
  copies: 2,
  mode: 'manual',
  ...over,
});

describe('ProtectionGroupHelper.displayState & stateVariant', () => {
  it('resolves displayState with error and frozen precedence', () => {
    expect(ProtectionGroupHelper.displayState(baseGroup())).toBe('ready');
    expect(
      ProtectionGroupHelper.displayState(
        baseGroup({ state: 'ready', lastError: 'transient' }),
      ),
    ).toBe('ready');
    expect(
      ProtectionGroupHelper.displayState(
        baseGroup({ state: 'promoting', lastError: 'boom' }),
      ),
    ).toBe('error');
    expect(
      ProtectionGroupHelper.displayState(
        baseGroup({ state: 'initializing', frozen: true }),
      ),
    ).toBe('frozen');
    expect(
      ProtectionGroupHelper.displayState(
        baseGroup({ state: 'initializing', frozen: false }),
      ),
    ).toBe('initializing');
    expect(
      ProtectionGroupHelper.displayState(baseGroup({ state: 'waiting' })),
    ).toBe('waiting');
    expect(
      ProtectionGroupHelper.displayState(baseGroup({ state: 'draining' })),
    ).toBe('draining');
    expect(
      ProtectionGroupHelper.displayState(baseGroup({ state: 'promoting' })),
    ).toBe('promoting');
    expect(
      ProtectionGroupHelper.displayState(
        baseGroup({ state: '', frozen: true }),
      ),
    ).toBe('frozen');
    expect(
      ProtectionGroupHelper.displayState(baseGroup({ state: 'other' })),
    ).toBe('unknown');
  });

  it('maps display states to status variants', () => {
    expect(ProtectionGroupHelper.stateVariant('ready')).toBe('success');
    expect(ProtectionGroupHelper.stateVariant('initializing')).toBe('info');
    expect(ProtectionGroupHelper.stateVariant('waiting')).toBe('info');
    expect(ProtectionGroupHelper.stateVariant('draining')).toBe('warning');
    expect(ProtectionGroupHelper.stateVariant('promoting')).toBe('warning');
    expect(ProtectionGroupHelper.stateVariant('frozen')).toBe('warning');
    expect(ProtectionGroupHelper.stateVariant('error')).toBe('danger');
    expect(ProtectionGroupHelper.stateVariant('unknown')).toBe('neutral');
  });
});

describe('ProtectionGroupHelper.wantedCopies, isDegraded & summary', () => {
  it('returns explicit copies or falls back to member count', () => {
    expect(ProtectionGroupHelper.wantedCopies(baseGroup({ copies: 2 }))).toBe(
      2,
    );
    expect(
      ProtectionGroupHelper.wantedCopies(
        baseGroup({ copies: undefined, members: ['a', 'b', 'c'] }),
      ),
    ).toBe(3);
  });

  it('computes isDegraded from status healthyCopies vs wantedCopies', () => {
    const group = baseGroup({ copies: 2 });

    expect(ProtectionGroupHelper.isDegraded(group, undefined)).toBeNull();
    expect(
      ProtectionGroupHelper.isDegraded(group, { healthyCopies: 2 }),
    ).toBe(false);
    expect(
      ProtectionGroupHelper.isDegraded(group, { healthyCopies: 1 }),
    ).toBe(true);
  });

  it('summarizes total, healthy, degraded and frozen groups', () => {
    const groups: ProtectionGroup[] = [
      baseGroup({ id: 'g1', copies: 2, state: 'ready' }),
      baseGroup({ id: 'g2', copies: 3, state: 'ready', frozen: true }),
      baseGroup({ id: 'g3', copies: 2, state: 'initializing' }),
    ];
    const statuses: Record<string, ProtectionGroupStatus> = {
      g1: { healthyCopies: 2 },
      g2: { healthyCopies: 1 },
    };

    expect(ProtectionGroupHelper.summary(groups, statuses)).toEqual({
      total: 3,
      healthy: 1,
      degraded: 1,
      frozen: 1,
    });
    expect(ProtectionGroupHelper.attentionCount(groups, statuses)).toBe(1);
  });
});

describe('ProtectionGroupHelper.memberRows', () => {
  it('maps members in stored order with role, health, lag and quarantine flags', () => {
    const group = baseGroup({
      members: ['s1', 's2', 's3', 's4'],
      primary: 's1',
      target: 's2',
      quarantined: ['s4'],
    });
    const status: ProtectionGroupStatus = {
      members: [
        {
          storage: 's1',
          available: true,
          initialized: true,
          current: true,
          eventLag: 0,
        },
        {
          storage: 's2',
          available: true,
          initialized: false,
          current: false,
          eventLag: 12,
        },
        {
          storage: 's3',
          available: true,
          initialized: true,
          current: false,
          eventLag: 5,
        },
        {
          storage: 's4',
          available: false,
          initialized: true,
          current: false,
          eventLag: 99,
        },
      ],
    };

    expect(ProtectionGroupHelper.memberRows(group, status)).toEqual([
      {
        storage: 's1',
        isPrimary: true,
        isTarget: false,
        excluded: false,
        health: 'healthy',
        lag: 0,
        initialized: true,
      },
      {
        storage: 's2',
        isPrimary: false,
        isTarget: true,
        excluded: false,
        health: 'syncing',
        lag: 12,
        initialized: false,
      },
      {
        storage: 's3',
        isPrimary: false,
        isTarget: false,
        excluded: false,
        health: 'lagging',
        lag: 5,
        initialized: true,
      },
      {
        storage: 's4',
        isPrimary: false,
        isTarget: false,
        excluded: true,
        health: 'unavailable',
        lag: 99,
        initialized: true,
      },
    ]);
  });
});

describe('ProtectionGroupHelper switch helpers', () => {
  it('filters primary and quarantined out of switchTargets', () => {
    const group = baseGroup({
      members: ['s1', 's2', 's3'],
      primary: 's1',
      quarantined: ['s3'],
    });

    expect(ProtectionGroupHelper.switchTargets(group)).toEqual(['s2']);
  });

  it('determines canSwitch and canAbort', () => {
    expect(ProtectionGroupHelper.canSwitch(baseGroup())).toBe(true);
    expect(
      ProtectionGroupHelper.canSwitch(baseGroup({ state: 'waiting' })),
    ).toBe(false);
    expect(
      ProtectionGroupHelper.canSwitch(baseGroup({ target: 's2' })),
    ).toBe(false);
    expect(
      ProtectionGroupHelper.canSwitch(
        baseGroup({ members: ['s1', 's2'], primary: 's1', quarantined: ['s2'] }),
      ),
    ).toBe(false);

    expect(ProtectionGroupHelper.canAbort(baseGroup())).toBe(false);
    expect(
      ProtectionGroupHelper.canAbort(
        baseGroup({ state: 'waiting', target: 's2' }),
      ),
    ).toBe(true);
    expect(
      ProtectionGroupHelper.canAbort(
        baseGroup({ state: 'draining', target: 's2' }),
      ),
    ).toBe(true);
    expect(
      ProtectionGroupHelper.canAbort(
        baseGroup({ state: 'promoting', target: 's2' }),
      ),
    ).toBe(false);
  });

  it('derives switchSteps progress from group state and target', () => {
    expect(
      ProtectionGroupHelper.switchSteps(baseGroup()).map((s) => s.status),
    ).toEqual(['pending', 'pending', 'pending', 'pending']);

    expect(
      ProtectionGroupHelper.switchSteps(
        baseGroup({ state: 'waiting', target: 's2' }),
      ).map((s) => s.status),
    ).toEqual(['active', 'pending', 'pending', 'pending']);

    expect(
      ProtectionGroupHelper.switchSteps(
        baseGroup({ state: 'draining', target: 's2' }),
      ).map((s) => s.status),
    ).toEqual(['done', 'active', 'pending', 'pending']);

    expect(
      ProtectionGroupHelper.switchSteps(
        baseGroup({ state: 'promoting', target: 's2' }),
      ).map((s) => s.status),
    ).toEqual(['done', 'done', 'active', 'pending']);

    expect(
      ProtectionGroupHelper.switchSteps(
        baseGroup({ state: 'initializing', frozen: true }),
      ).map((s) => s.status),
    ).toEqual(['done', 'done', 'done', 'active']);
  });
});

describe('ProtectionGroupHelper.moveMember', () => {
  it('moves members within bounds without mutating input', () => {
    const orig = ['s1', 's2', 's3'];

    expect(ProtectionGroupHelper.moveMember(orig, 0, 1)).toEqual([
      's2',
      's1',
      's3',
    ]);
    expect(ProtectionGroupHelper.moveMember(orig, 2, -1)).toEqual([
      's1',
      's3',
      's2',
    ]);
    expect(ProtectionGroupHelper.moveMember(orig, 0, -1)).toEqual(orig);
    expect(ProtectionGroupHelper.moveMember(orig, 2, 1)).toEqual(orig);
    expect(orig).toEqual(['s1', 's2', 's3']);
  });
});

describe('ProtectionGroupHelper.validate', () => {
  it('accepts valid create and edit forms', () => {
    expect(ProtectionGroupHelper.validate(baseForm(), false)).toEqual([]);
    expect(
      ProtectionGroupHelper.validate(
        baseForm({
          mode: 'auto',
          electorate: ['p1', 'p2', 'p3'],
          afterSeconds: 30,
          maxEventLag: 0,
          maxSwitchesPerHour: 3,
        }),
        true,
      ),
    ).toEqual([]);
  });

  it('reports create-only validation errors when isEdit is false', () => {
    expect(
      ProtectionGroupHelper.validate(baseForm({ id: '  ' }), false),
    ).toContain('ID_REQUIRED');
    expect(
      ProtectionGroupHelper.validate(baseForm({ id: 'bad:id' }), false),
    ).toContain('ID_INVALID');
    expect(
      ProtectionGroupHelper.validate(baseForm({ id: 'bad/id' }), false),
    ).toContain('ID_INVALID');
    expect(
      ProtectionGroupHelper.validate(baseForm({ members: ['s1'] }), false),
    ).toContain('MEMBERS_MIN');
    expect(
      ProtectionGroupHelper.validate(
        baseForm({ buckets: ['b1', 'b1'] }),
        false,
      ),
    ).toContain('BUCKET_INVALID');
    expect(
      ProtectionGroupHelper.validate(baseForm({ buckets: ['b*'] }), false),
    ).toContain('BUCKET_INVALID');
  });

  it('skips immutable field checks when isEdit is true', () => {
    expect(
      ProtectionGroupHelper.validate(
        baseForm({ id: '', user: null, buckets: ['b*'] }),
        true,
      ),
    ).toEqual([]);
  });

  it('validates copies range, lag, after, switches and auto electorate', () => {
    expect(
      ProtectionGroupHelper.validate(
        baseForm({ members: ['s1', 's2'], copies: 1 }),
        true,
      ),
    ).toContain('COPIES_RANGE');
    expect(
      ProtectionGroupHelper.validate(
        baseForm({ members: ['s1', 's2'], copies: 3 }),
        true,
      ),
    ).toContain('COPIES_RANGE');
    expect(
      ProtectionGroupHelper.validate(baseForm({ maxEventLag: -1 }), true),
    ).toContain('LAG_NEGATIVE');
    expect(
      ProtectionGroupHelper.validate(baseForm({ afterSeconds: -5 }), true),
    ).toContain('AFTER_NEGATIVE');
    expect(
      ProtectionGroupHelper.validate(
        baseForm({ maxSwitchesPerHour: 0 }),
        true,
      ),
    ).toContain('SWITCHES_POSITIVE');
    expect(
      ProtectionGroupHelper.validate(
        baseForm({ mode: 'auto', electorate: ['p1', 'p2'] }),
        true,
      ),
    ).toContain('ELECTORATE_MIN');
  });
});

describe('ProtectionGroupHelper.buildCreateRequest & buildUpdateRequest', () => {
  it('builds create request with trimmed id and optional mutable fields', () => {
    const req = ProtectionGroupHelper.buildCreateRequest(
      baseForm({
        id: '  pg-new ',
        user: 'alice',
        buckets: ['orders'],
        members: ['s1', 's2', 's3'],
        copies: 2,
        mode: 'auto',
        electorate: ['p1', 'p2', 'p3'],
        afterSeconds: 45,
        maxEventLag: 5,
        maxSwitchesPerHour: 4,
        adoptExisting: true,
      }),
    );

    expect(req).toEqual({
      group: {
        id: 'pg-new',
        user: 'alice',
        buckets: ['orders'],
        members: ['s1', 's2', 's3'],
        copies: 2,
        mode: 'auto',
        electorate: ['p1', 'p2', 'p3'],
        after: '45s',
        maxEventLag: 5,
        maxSwitchesPerHour: 4,
      },
      adoptExisting: true,
    });
  });

  it('defaults user to "default" in buildCreateRequest when user is not provided', () => {
    const req = ProtectionGroupHelper.buildCreateRequest(
      baseForm({
        id: 'pg-1',
        user: null,
      }),
    );
    expect(req.group.user).toBe('default');
  });

  it('builds update request preserving stored scope, members and revision', () => {
    const current = baseGroup({
      id: 'pg-1',
      user: 'alice',
      buckets: ['b1'],
      members: ['s1', 's2', 's3'],
      revision: '12',
    });
    const updated = ProtectionGroupHelper.buildUpdateRequest(
      current,
      baseForm({
        id: 'ignored',
        user: 'ignored',
        buckets: [],
        members: ['s1', 's2', 's3'],
        copies: 3,
        mode: 'manual',
        electorate: ['p1', 'p2', 'p3'],
        afterSeconds: null,
        maxEventLag: null,
        maxSwitchesPerHour: null,
      }),
    );

    expect(updated).toEqual({
      id: 'pg-1',
      user: 'alice',
      buckets: ['b1'],
      members: ['s1', 's2', 's3'],
      revision: '12',
      copies: 3,
      mode: 'manual',
      electorate: [],
    });
  });
});

describe('ProtectionGroupHelper.parseWriter', () => {
  it('parses writer records including owners containing colons', () => {
    const parsed = ProtectionGroupHelper.parseWriter(
      'proxy:eu-1:4:inst-abc:1700000000000',
    );

    expect(parsed.owner).toBe('proxy:eu-1');
    expect(parsed.generation).toBe(4);
    expect(parsed.instance).toBe('inst-abc');
    expect(parsed.admittedAt?.getTime()).toBe(1700000000000);
  });

  it('handles malformed writer records gracefully', () => {
    expect(ProtectionGroupHelper.parseWriter('short:record')).toEqual({
      owner: 'short:record',
      generation: 0,
      instance: '',
      admittedAt: null,
    });
  });
});
