/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { describe, expect, it } from 'vitest';
import { StorageTestHelper } from '../StorageTestHelper';
import type { StorageTestResult } from '@/utils/types/runtimeStorage';

const step = (number: number, status: string) => ({
  id: `s${number}`,
  number,
  name: `step ${number}`,
  status,
});
const ok: StorageTestResult = {
  ok: true,
  steps: [step(1, 'pass'), step(2, 'warn'), step(3, 'skip')],
  hasWarnings: true,
};
const bad: StorageTestResult = {
  ok: false,
  steps: [step(9, 'fail'), step(1, 'pass')],
};

describe('StorageTestHelper steps', () => {
  it('maps a status to variant and icon, unknown statuses are skipped', () => {
    expect(StorageTestHelper.stepMeta('pass')).toMatchObject({
      variant: 'success',
      icon: '✓',
    });
    expect(StorageTestHelper.stepMeta('fail').variant).toBe('danger');
    expect(StorageTestHelper.stepMeta('warn').variant).toBe('warning');
    expect(StorageTestHelper.stepMeta('skip').variant).toBe('neutral');
    expect(StorageTestHelper.stepMeta('weird').status).toBe('skip');
    expect(StorageTestHelper.stepMeta(undefined).status).toBe('skip');
  });

  it('orders by number, counts statuses and lists failures', () => {
    expect(StorageTestHelper.orderedSteps(bad).map((s) => s.number)).toEqual([
      1, 9,
    ]);
    expect(StorageTestHelper.counts(ok)).toEqual({
      pass: 1,
      fail: 0,
      warn: 1,
      skip: 1,
    });
    expect(StorageTestHelper.failing(bad).map((s) => s.id)).toEqual(['s9']);
    expect(StorageTestHelper.counts(null)).toEqual({
      pass: 0,
      fail: 0,
      warn: 0,
      skip: 0,
    });
  });

  it('parses int64 latencies', () => {
    expect(StorageTestHelper.latency('12')).toBe(12);
    expect(StorageTestHelper.latency(7)).toBe(7);
    expect(StorageTestHelper.latency(undefined)).toBeNull();
    expect(StorageTestHelper.latency('x')).toBeNull();
  });

  it('flags results with failures or warnings as a problem', () => {
    expect(StorageTestHelper.hasProblem(bad)).toBe(true);
    expect(StorageTestHelper.hasProblem(ok)).toBe(true);
    expect(
      StorageTestHelper.hasProblem({ ok: true, steps: [step(1, 'pass')] }),
    ).toBe(false);
    expect(StorageTestHelper.hasProblem(undefined)).toBe(false);
  });
});

describe('StorageTestHelper force-save flow', () => {
  it('needs a test first', () => {
    expect(StorageTestHelper.saveGate(null, false)).toBe('untested');
    expect(StorageTestHelper.canSave(null, true, true)).toBe(false);
  });

  it('saves a passing test (warnings allowed) without force', () => {
    expect(StorageTestHelper.saveGate(ok, false)).toBe('ok');
    expect(StorageTestHelper.canSave(ok, false, false)).toBe(true);
    expect(StorageTestHelper.forceFlag(ok, true, true)).toBe(false);
  });

  it('blocks failing steps, then needs "save anyway" and a confirmation', () => {
    expect(StorageTestHelper.saveGate(bad, false)).toBe('blocked');
    expect(StorageTestHelper.canSave(bad, false, true)).toBe(false);
    expect(StorageTestHelper.saveGate(bad, true)).toBe('force');
    expect(StorageTestHelper.canSave(bad, true, false)).toBe(false);
    expect(StorageTestHelper.canSave(bad, true, true)).toBe(true);
    expect(StorageTestHelper.forceFlag(bad, true, true)).toBe(true);
    expect(StorageTestHelper.forceFlag(bad, true, false)).toBe(false);
  });

  it('treats ok=true with a failing step as failing', () => {
    const odd: StorageTestResult = { ok: true, steps: [step(5, 'fail')] };

    expect(StorageTestHelper.saveGate(odd, false)).toBe('blocked');
  });

  it('builds API options and omits empty fields', () => {
    const form = StorageTestHelper.emptyOptions();

    expect(StorageTestHelper.toOptions(form)).toEqual({
      features: {
        versioning: true,
        tagging: false,
        acl: false,
        lifecycle: false,
        policy: false,
      },
    });
    expect(
      StorageTestHelper.toOptions({
        ...form,
        probeBucketPrefix: ' app-tutti-probe- ',
        throughput: true,
      }),
    ).toMatchObject({
      probeBucketPrefix: 'app-tutti-probe-',
      throughput: true,
    });
  });
});
