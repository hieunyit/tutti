/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { describe, expect, it } from 'vitest';
import { RuntimeStorageHelper } from '../RuntimeStorageHelper';

const storage = () => ({
  ...RuntimeStorageHelper.empty(),
  name: 'archive',
  address: 'https://s3.example.com:9000',
});

describe('RuntimeStorageHelper', () => {
  it('converts supported Go durations to protobuf seconds without scientific notation', () => {
    const value = {
      ...storage(),
      httpTimeout: '1m30s',
      healthCheckInterval: '1h',
    };

    expect(RuntimeStorageHelper.payload(value).httpTimeout).toBe('90s');
    expect(RuntimeStorageHelper.payload(value).healthCheckInterval).toBe(
      '3600s',
    );
    expect(
      RuntimeStorageHelper.payload({ ...value, httpTimeout: '1ns' })
        .httpTimeout,
    ).toBe('0.000000001s');
    expect(RuntimeStorageHelper.validConnection(value)).toBe(true);
  });

  it('rejects invalid endpoint/name/duration and sub-second health probes', () => {
    for (const overrides of [
      { name: 'UPPER' },
      { address: 'file:///tmp/key' },
      { address: 'https://user:secret@example.com' },
      { address: 'https://example.com/path' },
      { httpTimeout: '-1s' },
      { healthCheckInterval: '500ms' },
      { httpTimeout: 'banana' },
    ]) {
      expect(
        RuntimeStorageHelper.validConnection({ ...storage(), ...overrides }),
      ).toBe(false);
    }
  });

  it('derives TLS from the endpoint and omits read-only fields from mutations', () => {
    const payload = RuntimeStorageHelper.payload({
      ...storage(),
      address: 'http://localhost:9000',
      isSecure: true,
      source: 'config',
      references: ['replication'],
      users: [{ user: 'u', accessKeyId: 'key', source: 'dynamic' }],
    });

    expect(payload.isSecure).toBe(false);
    expect(payload).not.toHaveProperty('users');
    expect(payload).not.toHaveProperty('references');
    expect(payload).not.toHaveProperty('source');
  });

  it('only enables deletion of unreferenced non-main managed storages', () => {
    expect(
      RuntimeStorageHelper.canDelete({
        ...storage(),
        source: 'managed',
        references: [],
      }),
    ).toBe(true);
    expect(
      RuntimeStorageHelper.canDelete({ ...storage(), source: 'config' }),
    ).toBe(false);
    expect(
      RuntimeStorageHelper.canDelete({
        ...storage(),
        source: 'managed',
        isMain: true,
      }),
    ).toBe(false);
    expect(
      RuntimeStorageHelper.canDelete({
        ...storage(),
        source: 'managed',
        references: ['routing'],
      }),
    ).toBe(false);
  });
});
