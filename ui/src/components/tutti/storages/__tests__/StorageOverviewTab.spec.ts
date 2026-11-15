import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { describe, expect, it, vi } from 'vitest';
import StorageOverviewTab from '../StorageOverviewTab.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

tolerateNaiveStyles();

vi.mock('@/components/tutti/monitoring/useNumberFormat', () => ({
  useNumberFormat: () => (value: number) => String(value),
}));
vi.mock('@/components/auth/WriteGate.vue', () => ({
  default: { template: '<div><slot /></div>' },
}));
vi.mock('../StorageHealthTab.vue', () => ({
  default: { template: '<i data-test="health" />' },
}));

const row = {
  health: { status: 'available', reporting: 1 },
  breaker: { state: 'closed', failures: 0 },
  requestsPerSecond: 3,
  users: { count: 2, missing: [] },
  replication: { sourceOf: 1, targetOf: 0, lag: 0, paused: 0 },
  failover: null,
  openFailoverEvents: 0,
};
const storage: RuntimeStorage = {
  name: 'a',
  address: 'https://a',
  provider: 'Minio',
  defaultRegion: '',
  isSecure: true,
  httpTimeout: '30s',
  healthCheckInterval: '10s',
  source: 'managed',
};

function mountTab(value: RuntimeStorage) {
  return mount(StorageOverviewTab, {
    props: { row: row as never, storage: value, proxies: [], now: 0 },
    global: {
      plugins: [createI18n({ legacy: false, locale: 'en', messages: {} })],
    },
  });
}

describe('StorageOverviewTab', () => {
  it('warns that replication needs a connection account and links to settings', async () => {
    const wrapper = mountTab(storage);

    expect(wrapper.text()).toContain('Replication and health checks use');
    await wrapper.get('button').trigger('click');
    expect(wrapper.emitted('open-settings')).toHaveLength(1);
    expect(wrapper.find('[data-test="health"]').exists()).toBe(true);
  });

  it('shows the account key and no warning when configured', () => {
    const wrapper = mountTab({
      ...storage,
      connectionAccount: { configured: true, accessKeyId: 'AKIA1' },
    });

    expect(wrapper.text()).toContain('AKIA1');
    expect(wrapper.text()).not.toContain('Set connection account');
  });
});
