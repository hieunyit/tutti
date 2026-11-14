import { describe, expect, it } from 'vitest';
import { InitProgressHelper as H } from '../InitProgressHelper';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

const repl = (p: Partial<TuttiReplication>) => p as TuttiReplication;

describe('InitProgressHelper', () => {
  it('formats sizes', () => {
    expect(H.formatSize(0)).toBe('0 B');
    expect(H.formatSize(1536)).toBe('1.5 KB');
    expect(H.formatSize(12.4 * 1024 ** 3)).toBe('12.4 GB');
    expect(H.formatSize(812 * 1024 ** 3)).toBe('812 GB');
    expect(H.formatSize(-5)).toBe('0 B');
  });

  it('formats durations and unknown', () => {
    expect(H.formatDuration(null)).toBeNull();
    expect(H.formatDuration(0)).toBeNull();
    expect(H.formatDuration(NaN)).toBeNull();
    expect(H.formatDuration(20)).toBe('< 1 min');
    expect(H.formatDuration(300)).toBe('5 min');
    expect(H.formatDuration(25 * 3600 + 10 * 60)).toBe('25 h 10 min');
    expect(H.formatDuration(3 * 3600)).toBe('3 h');
    expect(H.formatDuration(76 * 3600)).toBe('3 d 4 h');
  });

  it('builds progress from string int64 fields', () => {
    const p = H.fromReplication(
      repl({
        initBytesTotal: '1000',
        initBytesDone: '250',
        initBytesPerSecond: '10',
        initEtaSeconds: '75',
        initListingComplete: true,
      }),
    );

    expect(p).toMatchObject({
      tracked: true,
      percent: 25,
      bytesPerSecond: 10,
      etaSeconds: 75,
      listingComplete: true,
    });
  });

  it('is untracked without bytes and tolerates missing fields', () => {
    const p = H.fromReplication(repl({}));

    expect(p.tracked).toBe(false);
    expect(p.percent).toBeNull();
    expect(p.etaSeconds).toBeNull();
    expect(p.listingComplete).toBe(false);
  });

  it('computes the progress of one running copy', () => {
    expect(H.copyProgress('1000', '330')).toEqual({
      done: 330,
      size: 1000,
      percent: 33,
    });
    expect(H.copyProgress(1000, 5000)?.done).toBe(1000);
    expect(H.copyProgress(1000, undefined)?.percent).toBe(0);
    expect(H.copyProgress(undefined, '10')).toBeNull();
  });

  it('caps percent at 100', () => {
    expect(
      H.fromReplication(repl({ initBytesTotal: '10', initBytesDone: '30' }))
        .percent,
    ).toBe(100);
  });
});
