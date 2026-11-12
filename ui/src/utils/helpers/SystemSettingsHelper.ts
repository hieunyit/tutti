import type { TuttiUpdateSystemSettingsRequest } from '@/utils/types/tutti';

export const WORKER_DURATION_FIELDS = [
  'shutdownTimeout',
  'queueUpdateInterval',
  'taskCheckInterval',
  'delayedTaskCheckInterval',
  'swiftRetryInterval',
  'pauseRetryInterval',
  'switchRetryInterval',
] as const;

const MAX_INT64 = 9223372036854775807n;
const UNITS: Record<string, bigint> = {
  h: 3600000000000n,
  m: 60000000000n,
  s: 1000000000n,
  ms: 1000000n,
  us: 1000n,
  µs: 1000n,
  ns: 1n,
};

/** Accept familiar Go durations; send the seconds-only protobuf JSON format. */
export function toProtoDuration(value: string): string | null {
  const input = value.trim();
  const parts = [...input.matchAll(/(\d+(?:\.\d{1,9})?)(ms|us|µs|ns|h|m|s)/g)];

  if (!parts.length || parts.map((part) => part[0]).join('') !== input)
    return null;

  let total = 0n;

  for (const part of parts) {
    const [whole, fraction = ''] = part[1]!.split('.');
    const scale = 10n ** BigInt(fraction.length);
    const nanos = BigInt(whole! + fraction) * UNITS[part[2]!]!;

    if (nanos % scale !== 0n) return null;

    total += nanos / scale;
  }

  if (total <= 0n || total > MAX_INT64) return null;

  const fraction = (total % 1000000000n)
    .toString()
    .padStart(9, '0')
    .replace(/0+$/, '');

  return `${total / 1000000000n}${fraction ? `.${fraction}` : ''}s`;
}

export function validateSystemSettings(
  draft: TuttiUpdateSystemSettingsRequest,
) {
  const errors: Record<string, string> = {};
  const worker = draft.worker ?? {};

  if (
    !Number.isInteger(worker.concurrency) ||
    worker.concurrency! < 1 ||
    worker.concurrency! > 1000
  ) {
    errors.concurrency = 'invalidConcurrency';
  }

  for (const key of WORKER_DURATION_FIELDS) {
    if (!toProtoDuration(worker[key] ?? '')) errors[key] = 'invalidDuration';
  }

  if (!toProtoDuration(draft.lock?.overlap ?? ''))
    errors.lockOverlap = 'invalidDuration';

  for (const key of ['backlogHighWatermark', 'backlogLowWatermark'] as const) {
    if (
      !Number.isInteger(worker[key]) ||
      worker[key]! < 1 ||
      worker[key]! > 2147483647
    ) {
      errors[key] = 'invalidWatermark';
    }
  }

  if (worker.backlogLowWatermark! >= worker.backlogHighWatermark!) {
    errors.backlogLowWatermark = 'invalidWatermarkOrder';
  }

  for (const key of ['bandwidthLimitBytes', 'bufferBudgetBytes'] as const) {
    const value = worker[key] ?? '';

    if (!/^\d{1,19}$/.test(value) || BigInt(value) > MAX_INT64)
      errors[key] = 'invalidBytes';
  }

  const baseUrl = draft.webhook?.baseUrl?.trim();

  if (baseUrl) {
    try {
      const url = new URL(baseUrl);

      if (
        !['http:', 'https:'].includes(url.protocol) ||
        url.username ||
        url.password
      ) {
        errors.webhookBaseUrl = 'invalidUrl';
      }
    } catch {
      errors.webhookBaseUrl = 'invalidUrl';
    }
  }

  return errors;
}
