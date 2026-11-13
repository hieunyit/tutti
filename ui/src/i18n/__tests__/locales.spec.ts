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
  expect(localStorage.getItem('chorus-locale')).toContain('vi');
  expect(toKitLocale('vi')).toBe('en');
  expect(toKitLocale('de')).toBe('de');

  (i18n.global.locale as { value: string }).value = 'en';
  store.initLocale();
  expect(store.locale).toBe('vi');
});
