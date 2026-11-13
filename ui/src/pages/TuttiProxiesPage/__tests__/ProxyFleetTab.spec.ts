/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProxyFleetTab from '../ProxyFleetTab.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { TuttiService } from '@/services/TuttiService';

tolerateNaiveStyles();

vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  EMPTY_DATE_TEXT: '–',
  formatDateTime: () => 'now',
  formatRelative: () => 'now',
}));

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getProxies: vi.fn() },
}));

function mountTab() {
  return mount(ProxyFleetTab, {
    global: {
      plugins: [
        createPinia(),
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.mocked(TuttiService.getProxies).mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('ProxyFleetTab', () => {
  it('shows a loading skeleton while the first request is pending', async () => {
    vi.mocked(TuttiService.getProxies).mockReturnValue(new Promise(() => {}));
    const wrapper = mountTab();

    await flushPromises();
    expect(wrapper.find('[data-kit-state="loading"]').exists()).toBe(true);
    wrapper.unmount();
  });

  it('shows an empty state when no proxy reports', async () => {
    vi.mocked(TuttiService.getProxies).mockResolvedValue({ proxies: [] });
    const wrapper = mountTab();

    await flushPromises();
    expect(wrapper.find('[data-kit-state="empty"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('No proxies are reporting');
    wrapper.unmount();
  });

  it('shows an error with a working retry instead of a blank page', async () => {
    vi.mocked(TuttiService.getProxies).mockRejectedValueOnce(new Error('down'));
    const wrapper = mountTab();

    await flushPromises();
    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);

    vi.mocked(TuttiService.getProxies).mockResolvedValueOnce({
      proxies: [{ appId: 'proxy-1', address: '10.0.0.1' }],
    });
    await wrapper.get('[data-kit-state="error"] button').trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('proxy-1');
    wrapper.unmount();
  });

  it('lists the proxies when the request succeeds', async () => {
    vi.mocked(TuttiService.getProxies).mockResolvedValue({
      proxies: [{ appId: 'proxy-1' }, { appId: 'proxy-2' }],
      configVersion: 2,
    });
    const wrapper = mountTab();

    await flushPromises();
    expect(wrapper.text()).toContain('proxy-1');
    expect(wrapper.text()).toContain('proxy-2');
    wrapper.unmount();
  });
});
