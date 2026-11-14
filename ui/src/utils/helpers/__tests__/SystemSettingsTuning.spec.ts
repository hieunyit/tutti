import { describe, expect, it } from 'vitest';
import {
  recommendSettings,
  validateSystemSettings,
} from '../SystemSettingsHelper';
import { diffSettings } from '@/components/tutti/settings/settingsMeta';
import type { TuttiUpdateSystemSettingsRequest } from '@/utils/types/tutti';

function draft(
  worker: Record<string, number | undefined>,
): TuttiUpdateSystemSettingsRequest {
  return {
    worker: {
      concurrency: 10,
      shutdownTimeout: '5s',
      queueUpdateInterval: '3s',
      taskCheckInterval: '1s',
      delayedTaskCheckInterval: '3s',
      swiftRetryInterval: '1m',
      pauseRetryInterval: '30s',
      switchRetryInterval: '30s',
      backlogHighWatermark: 10,
      backlogLowWatermark: 5,
      bandwidthLimitBytes: '0',
      bufferBudgetBytes: '0',
      ...worker,
    },
    lock: { overlap: '2s' },
  };
}

describe('worker tuning validation', () => {
  it.each([
    ['bulkConcurrencyPercent', 0, true],
    ['bulkConcurrencyPercent', 5, true],
    ['bulkConcurrencyPercent', 95, true],
    ['bulkConcurrencyPercent', 4, false],
    ['bulkConcurrencyPercent', 96, false],
    ['bulkConcurrencyPercent', 5.5, false],
    ['multipartThreads', 0, true],
    ['multipartThreads', 1, true],
    ['multipartThreads', 16, true],
    ['multipartThreads', 17, false],
    ['multipartThreads', -1, false],
    ['multipartPartSizeMib', 0, true],
    ['multipartPartSizeMib', 5, true],
    ['multipartPartSizeMib', 512, true],
    ['multipartPartSizeMib', 4, false],
    ['multipartPartSizeMib', 513, false],
  ])('%s=%s valid=%s', (key, value, valid) => {
    const errors = validateSystemSettings(draft({ [key]: value }));

    expect(Boolean(errors[key])).toBe(!valid);
  });

  it('treats a missing value as the default', () => {
    expect(validateSystemSettings(draft({}))).toEqual({});
  });

  it('includes the new fields in the diff with restart flags', () => {
    const changes = diffSettings(
      draft({}),
      draft({ bulkConcurrencyPercent: 80, multipartThreads: 4 }),
    );

    expect(changes).toEqual([
      { key: 'bulkConcurrencyPercent', from: '0', to: '80', restart: true },
      { key: 'multipartThreads', from: '0', to: '4', restart: false },
    ]);
  });
});

describe('recommendSettings', () => {
  const GIB = 1024 ** 3;

  it.each([
    [1, 1, 10, 20, 256 * 1024 ** 2],
    [2, 4, 20, 20, GIB],
    [4, 32, 40, 30, 8 * GIB],
    [16, 64, 128, 40, 8 * GIB],
  ])('%i cpu / %i GB', (cpu, ram, conc, bulk, buf) => {
    expect(recommendSettings(cpu, ram)).toEqual({
      concurrency: conc,
      bufferBudgetBytes: String(buf),
      multipartThreads: 1,
      bulkConcurrencyPercent: bulk,
    });
  });

  it('floors at the historical defaults on nonsense input', () => {
    const r = recommendSettings(0, -5);

    expect(r.concurrency).toBe(10);
    expect(r.bufferBudgetBytes).toBe(String(256 * 1024 ** 2));
  });

  it('accepts concurrency 0 (auto) in validation', () => {
    expect(validateSystemSettings(draft({ concurrency: 0 })).concurrency).toBe(
      undefined,
    );
  });
});
