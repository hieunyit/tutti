/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import axios from 'axios';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

export abstract class RuntimeStorageHelper {
  static durationSeconds(value: string): number | null {
    if (value.trim() === '0') return 0;

    const input = value.trim();
    const parts = [...input.matchAll(/(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g)];

    if (!parts.length || parts.map((part) => part[0]).join('') !== input)
      return null;

    const units: Record<string, number> = {
      ns: 1e-9,
      us: 1e-6,
      µs: 1e-6,
      ms: 1e-3,
      s: 1,
      m: 60,
      h: 3600,
    };
    const seconds = parts.reduce(
      (sum, part) => sum + Number(part[1]) * (units[part[2]!] ?? 0),
      0,
    );

    return Number.isFinite(seconds) && seconds <= 315576000000 ? seconds : null;
  }

  static copy(storage: RuntimeStorage): RuntimeStorage {
    return JSON.parse(JSON.stringify(storage)) as RuntimeStorage;
  }

  static empty(): RuntimeStorage {
    return {
      name: '',
      address: '',
      provider: 'Minio',
      defaultRegion: 'us-east-1',
      isSecure: true,
      httpTimeout: '60s',
      healthCheckInterval: '10s',
      rateLimit: { enabled: false, rpm: 600, includeMetadataApi: false },
    };
  }

  static validConnection(storage: RuntimeStorage): boolean {
    if (!/^[a-z0-9_-]{1,63}$/.test(storage.name)) return false;

    try {
      const url = new URL(storage.address);

      if (
        !['http:', 'https:'].includes(url.protocol) ||
        url.username ||
        url.password ||
        url.search ||
        url.hash ||
        (url.pathname !== '' && url.pathname !== '/')
      )
        return false;
    } catch {
      return false;
    }

    const timeout = RuntimeStorageHelper.durationSeconds(storage.httpTimeout);
    const interval = RuntimeStorageHelper.durationSeconds(
      storage.healthCheckInterval,
    );

    return (
      timeout !== null &&
      interval !== null &&
      (interval === 0 || interval >= 1) &&
      (!storage.rateLimit?.enabled || storage.rateLimit.rpm > 0)
    );
  }

  static payload(storage: RuntimeStorage): RuntimeStorage {
    const duration = (value: string) =>
      `${(RuntimeStorageHelper.durationSeconds(value) ?? 0).toFixed(9).replace(/\.?0+$/, '') || '0'}s`;

    return {
      name: storage.name,
      address: storage.address.trim(),
      provider: storage.provider,
      defaultRegion: storage.defaultRegion.trim(),
      isSecure: storage.address.trim().startsWith('https://'),
      httpTimeout: duration(storage.httpTimeout),
      healthCheckInterval: duration(storage.healthCheckInterval),
      rateLimit: storage.rateLimit ? { ...storage.rateLimit } : undefined,
    };
  }

  static error(error: unknown, fallback: string): string {
    if (!axios.isAxiosError(error)) return fallback;

    const data = error.response?.data as
      | { message?: string; details?: { reason?: string }[] }
      | undefined;

    return (
      data?.details?.find((detail) => detail.reason)?.reason ??
      data?.message ??
      fallback
    );
  }

  static canDelete(storage: RuntimeStorage): boolean {
    return (
      storage.source === 'managed' &&
      !storage.isMain &&
      (storage.references?.length ?? 0) === 0
    );
  }
}
