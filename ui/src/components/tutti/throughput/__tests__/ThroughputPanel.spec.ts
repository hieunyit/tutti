import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import ThroughputPanel from '../ThroughputPanel.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { TuttiService } from '@/services/TuttiService';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getWorkerStats: vi.fn() },
}));

const stats = {
  workers: [
    {
      workerId: 'worker-a',
      concurrency: 10,
      inFlight: '10',
      objectsPerSec1m: 12.5,
      bytesPerSec1m: '1572864',
      bufferTotalBytes: '1000',
      bufferUsedBytes: '950',
      bufferWaiters: '3',
      failedTotal: '2',
    },
  ],
  totals: {
    inFlight: '10',
    concurrency: '10',
    objectsPerSec1m: 12.5,
    bytesPerSec1m: '1572864',
    objectsPerSec5m: 8,
    bytesPerSec5m: '1048576',
    bufferTotalBytes: '1000',
    bufferUsedBytes: '950',
    bufferWaiters: '3',
    failedTotal: '2',
  },
};

async function mountPanel() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div/>' } }],
  });

  await router.push('/');

  const wrapper = mount(ThroughputPanel, {
    global: {
      plugins: [
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });

  await flushPromises();

  return wrapper;
}

describe('ThroughputPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.mocked(TuttiService.getWorkerStats).mockReset();
  });

  it('renders tiles, hints and the worker table from the response', async () => {
    vi.mocked(TuttiService.getWorkerStats).mockResolvedValue(stats);

    const wrapper = await mountPanel();
    const tiles = wrapper.get('[data-test="throughput-tiles"]').text();

    expect(tiles).toContain('12.5');
    expect(tiles).toContain('1.5 MB/s');
    expect(tiles).toContain('10 / 10');
    expect(tiles).toContain('95%');
    expect(wrapper.get('[data-test="throughput-workers"]').text()).toContain(
      'worker-a',
    );
    expect(wrapper.find('[data-hint="bufferBottleneck"]').exists()).toBe(true);
    expect(wrapper.find('[data-hint="workersBusy"]').exists()).toBe(true);
    wrapper.unmount();
  });

  it('is hidden when the backend has no stats route', async () => {
    vi.mocked(TuttiService.getWorkerStats).mockRejectedValue(
      Object.assign(new Error('nf'), {
        isAxiosError: true,
        response: { status: 404 },
      }),
    );

    const wrapper = await mountPanel();

    expect(wrapper.find('[data-test="throughput-panel"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it('shows an error block with retry on other failures', async () => {
    vi.mocked(TuttiService.getWorkerStats).mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        isAxiosError: true,
        response: { status: 500 },
      }),
    );

    const wrapper = await mountPanel();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);

    vi.mocked(TuttiService.getWorkerStats).mockResolvedValue(stats);
    await wrapper.get('[data-kit-state="error"] button').trigger('click');
    await flushPromises();

    expect(wrapper.find('[data-test="throughput-tiles"]').exists()).toBe(true);
    wrapper.unmount();
  });
});
