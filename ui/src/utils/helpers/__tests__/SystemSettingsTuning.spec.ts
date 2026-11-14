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
    ['multipartPartSizeMiB', 0, true],
    ['multipartPartSizeMiB', 5, true],
    ['multipartPartSizeMiB', 512, true],
    ['multipartPartSizeMiB', 4, false],
    ['multipartPartSizeMiB', 513, false],
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
  it('uses the defaults of 4 cores / 32 GB', () => {
    expect(recommendSettings(4, 32)).toEqual({
      concurrency: 64,
      bufferBudgetBytes: String(8 * 1024 ** 3),
      multipartThreads: 4,
      bulkConcurrencyPercent: 80,
    });
  });

  it('caps concurrency at 256 and the buffer at 8 GiB', () => {
    const r = recommendSettings(64, 512);

    expect(r.concurrency).toBe(256);
    expect(r.bufferBudgetBytes).toBe(String(8 * 1024 ** 3));
  });

  it('takes 25% of RAM below the cap', () => {
    expect(recommendSettings(2, 8).bufferBudgetBytes).toBe(
      String(2 * 1024 ** 3),
    );
    expect(recommendSettings(2, 8).concurrency).toBe(32);
  });

  it('guards against nonsense input', () => {
    const r = recommendSettings(0, -5);

    expect(r.concurrency).toBe(16);
    expect(r.bufferBudgetBytes).toBe('0');
  });
});
