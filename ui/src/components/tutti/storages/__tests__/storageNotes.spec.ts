import { describe, expect, it } from 'vitest';
import { healthTone, lacksAccount, storageNotes } from '../storageNotes';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

const base: RuntimeStorage = {
  name: 'a',
  address: 'https://a',
  provider: 'Minio',
  defaultRegion: '',
  isSecure: true,
  httpTimeout: '30s',
  healthCheckInterval: '10s',
  connectionAccount: { configured: true, accessKeyId: 'k' },
};
const health = { health: { status: 'available' } } as never;

describe('storageNotes', () => {
  it('shows nothing for a healthy storage with an account', () => {
    expect(storageNotes(base, health)).toEqual([]);
  });

  it('ignores test warnings, flags a missing account and a failed test', () => {
    const warned = { ...base, lastTest: { ok: true, hasWarnings: true } };

    expect(storageNotes(warned as RuntimeStorage, health)).toEqual([]);

    const bad = {
      ...base,
      connectionAccount: { configured: false },
      lastTest: { ok: false },
    };

    expect(
      storageNotes(bad as RuntimeStorage, health).map((n) => n.key),
    ).toEqual(['noAccount', 'testFailed']);
    expect(lacksAccount(bad as RuntimeStorage)).toBe(true);
  });

  it('adds the health warning and maps tones', () => {
    const warn = { health: { status: 'available', warning: 'x' } } as never;

    expect(storageNotes(base, warn).map((n) => n.key)).toEqual([
      'healthWarning',
    ]);
    expect(healthTone('unavailable')).toBe('danger');
    expect(healthTone('unknown')).toBe('neutral');
  });
});
