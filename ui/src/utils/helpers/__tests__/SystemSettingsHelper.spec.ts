import { describe, expect, it } from 'vitest';
import {
  toProtoDuration,
  validateSystemSettings,
} from '../SystemSettingsHelper';

describe('system settings API contract', () => {
  it.each([
    ['1m', '60s'],
    ['1m30s', '90s'],
    ['1h2m3.5s', '3723.5s'],
    ['500ms', '0.5s'],
    ['1ns', '0.000000001s'],
    [' 60s ', '60s'],
    ['9223372036.854775807s', '9223372036.854775807s'],
  ])('normalizes %s without losing nanosecond precision', (input, expected) => {
    expect(toProtoDuration(input)).toBe(expected);
  });

  it.each([
    '',
    '0s',
    '-1s',
    '5',
    'foo1s',
    '1s junk',
    '1m 30s',
    '0.1ns',
    '9223372036.854775808s',
    '1.1234567890s',
  ])('rejects invalid duration %s', (input) => {
    expect(toProtoDuration(input)).toBeNull();
  });

  it.each(['-1', '1.5', '1e6', '9223372036854775808', ''])(
    'rejects invalid byte budgets %s',
    (value) => {
      expect(
        validateSystemSettings({ worker: { bufferBudgetBytes: value } })
          .bufferBudgetBytes,
      ).toBe('invalidBytes');
    },
  );

  it('preserves int64 values above JavaScript safe integer and allows zero for unlimited', () => {
    expect(
      validateSystemSettings({
        worker: {
          bufferBudgetBytes: '9223372036854775807',
          bandwidthLimitBytes: '0',
        },
      }),
    ).not.toHaveProperty('bufferBudgetBytes');
    expect(
      validateSystemSettings({ worker: { bandwidthLimitBytes: '0' } }),
    ).not.toHaveProperty('bandwidthLimitBytes');
  });

  it.each([
    'javascript:alert(1)',
    'ftp://example.com',
    'https://user:secret@example.com',
    'not a URL',
  ])('rejects unsafe webhook URL %s', (baseUrl) => {
    expect(
      validateSystemSettings({ webhook: { baseUrl } }).webhookBaseUrl,
    ).toBe('invalidUrl');
  });
});
