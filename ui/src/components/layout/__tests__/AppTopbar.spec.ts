/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import AppTopbar from '../AppTopbar.vue';
import CompactNotice from '../CompactNotice.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';

// the real kit pulls in a stylesheet that node cannot import
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de', VI: 'vi' },
  ColorScheme: { LIGHT: 'light', DARK: 'dark' },
}));

vi.mock('@/stores/i18nStore', async () => {
  const { defineStore } = await import('pinia');

  return {
    useI18nStore: defineStore('i18n-spec', {
      state: () => ({ locale: 'en', localeList: ['en'] }),
      actions: { setLocale() {} },
    }),
  };
});

// the colour scheme store reads matchMedia while it is imported
vi.hoisted(() => {
  window.matchMedia = (() => ({
    matches: false,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  })) as never;
});

tolerateNaiveStyles();

const stubs = {
  AppBreadcrumbs: { template: '<nav data-test="crumbs" />' },
  AppQuickSearch: { template: '<div data-test="search" />' },
  AppThemeToggle: { template: '<div data-test="theme" />' },
  HelpMenu: { template: '<div data-test="help" />' },
  TopbarOverflow: {
    template: '<button data-test="overflow" aria-label="More options" />',
  },
  AuthUserMenu: { template: '<div data-test="user" />' },
  NSelect: { template: '<div data-test="lang" />' },
};

function mockWidth(phone: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation((query: string) => ({
      matches: phone && query.includes('640px'),
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
}

async function mountBar(phone: boolean) {
  mockWidth(phone);
  setActivePinia(createPinia());

  const wrapper = mount(AppTopbar, {
    props: { isMobile: true },
    global: {
      stubs,
      plugins: [
        createI18n({ legacy: false, locale: 'en', messages: { en: {} } }),
      ],
    },
  });

  await nextTick();

  return wrapper;
}

afterEach(() => {
  vi.unstubAllGlobals();
  sessionStorage.clear();
});

describe('AppTopbar', () => {
  it('is one compact row on phones: no quick search, one overflow menu', async () => {
    const wrapper = await mountBar(true);

    expect(wrapper.find('[data-test="search"]').exists()).toBe(false);
    expect(wrapper.find('[data-test="lang"]').exists()).toBe(false);
    expect(wrapper.find('[data-test="help"]').exists()).toBe(false);
    expect(wrapper.find('[data-test="theme"]').exists()).toBe(false);
    expect(wrapper.find('[data-test="overflow"]').exists()).toBe(true);
    expect(
      wrapper.find('.app-topbar__burger').attributes('aria-label'),
    ).toBeTruthy();
    wrapper.unmount();
  });

  it('keeps the full controls and the quick search on wider screens', async () => {
    const wrapper = await mountBar(false);

    expect(wrapper.find('[data-test="search"]').exists()).toBe(true);
    expect(wrapper.find('[data-test="help"]').exists()).toBe(true);
    expect(wrapper.find('[data-test="theme"]').exists()).toBe(true);
    expect(wrapper.find('[data-test="overflow"]').exists()).toBe(false);
    wrapper.unmount();
  });
});

describe('CompactNotice', () => {
  const props = {
    id: 'spec',
    text: 'Heads up',
    linkLabel: 'Details',
    to: '/docs',
  };
  const mountNotice = () =>
    mount(CompactNotice, {
      props,
      global: {
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
        plugins: [
          createI18n({ legacy: false, locale: 'en', messages: { en: {} } }),
        ],
      },
    });

  it('shows one line and stays dismissed for the session', async () => {
    const wrapper = mountNotice();

    expect(wrapper.text()).toContain('Heads up');
    await wrapper.find('button').trigger('click');
    expect(wrapper.find('.compact-notice').exists()).toBe(false);
    expect(sessionStorage.getItem('tutti.notice.dismissed.spec')).toBe('1');
    expect(mountNotice().find('.compact-notice').exists()).toBe(false);
  });
});
