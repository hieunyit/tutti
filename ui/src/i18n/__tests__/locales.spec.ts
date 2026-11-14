import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, expect, it, vi } from 'vitest';
import { i18n } from '@/i18n';
import { toKitLocale } from '@/i18n/locales';
import { useI18nStore } from '@/stores/i18nStore';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

it('offers vi, applies and persists it, and maps it to en for the ui-kit', () => {
  const store = useI18nStore();

  expect(store.localeList).toContain('vi');
  store.setLocale('vi');
  expect(i18n.global.locale.value).toBe('vi');
  expect(localStorage.getItem('tutti-locale')).toContain('vi');
  expect(toKitLocale('vi')).toBe('en');
  expect(toKitLocale('de')).toBe('de');

  (i18n.global.locale as { value: string }).value = 'en';
  store.initLocale();
  expect(store.locale).toBe('vi');
});

it('migrates the legacy locale key once', () => {
  localStorage.setItem('chorus-locale', 'de');
  const store = useI18nStore();

  store.initLocale();

  expect(store.locale).toBe('de');
  expect(localStorage.getItem('tutti-locale')).toContain('de');
  expect(localStorage.getItem('chorus-locale')).toBeNull();
});
