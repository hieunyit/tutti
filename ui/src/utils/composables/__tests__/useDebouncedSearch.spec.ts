/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { effectScope, nextTick } from 'vue';
import { useDebouncedSearch } from '../useDebouncedSearch';

// GeneralHelper pulls the i18n store in; the real lodash debounce is kept
vi.mock('@/stores/i18nStore', () => ({ useI18nStore: vi.fn() }));

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useDebouncedSearch', () => {
  it('lags the debounced value by the delay and keeps only the last keystroke', async () => {
    const { search, debounced } = useDebouncedSearch(250);

    search.value = 'a';
    await nextTick();
    vi.advanceTimersByTime(100);
    search.value = 'ab';
    await nextTick();
    vi.advanceTimersByTime(200);
    expect(debounced.value).toBe('');

    vi.advanceTimersByTime(60);
    expect(debounced.value).toBe('ab');
  });

  it('clears at once and cancels a pending value', async () => {
    const { search, debounced } = useDebouncedSearch(250);

    search.value = 'abc';
    await nextTick();
    vi.advanceTimersByTime(250);
    expect(debounced.value).toBe('abc');

    search.value = 'abcd';
    await nextTick();
    search.value = '';
    await nextTick();
    expect(debounced.value).toBe('');

    vi.advanceTimersByTime(1000);
    expect(debounced.value).toBe('');
  });

  it('does not apply a pending value after the scope is disposed', async () => {
    const scope = effectScope();
    const { search, debounced } = scope.run(() => useDebouncedSearch(250))!;

    search.value = 'late';
    await nextTick();
    scope.stop();
    vi.advanceTimersByTime(1000);

    expect(debounced.value).toBe('');
  });

  it('uses 250 ms by default', async () => {
    const { search, debounced } = useDebouncedSearch();

    search.value = 'x';
    await nextTick();
    vi.advanceTimersByTime(249);
    expect(debounced.value).toBe('');
    vi.advanceTimersByTime(2);
    expect(debounced.value).toBe('x');
  });
});
