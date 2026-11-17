/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TuttiLifecyclePage from '../TuttiLifecyclePage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { LifecycleService } from '@/services/LifecycleService';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { RouteName } from '@/utils/types/router';
import { useAuthStore } from '@/stores/authStore';
import type {
  LifecycleAuditLogEntry,
  LifecycleRule,
} from '@/utils/types/lifecycle';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({
    createNotification: vi.fn(),
    removeNotification: vi.fn(),
  }),
}));

vi.mock('@/services/LifecycleService', () => ({
  LifecycleService: {
    listRules: vi.fn(),
    createRule: vi.fn(),
    updateRule: vi.fn(),
    deleteRule: vi.fn(),
    enableRule: vi.fn(),
    disableRule: vi.fn(),
    dryRun: vi.fn(),
    listAuditLogs: vi.fn(),
  },
}));

vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: {
    listSummary: vi.fn(),
  },
}));

const listRules = vi.mocked(LifecycleService.listRules);
const listSummary = vi.mocked(RuntimeStorageService.listSummary);

function mountPage(canWrite = true) {
  const pinia = createPinia();

  useAuthStore(pinia).authEnabled = canWrite ? false : true;
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
  });

  return mount(TuttiLifecyclePage, {
    global: {
      plugins: [
        pinia,
        router,
        createI18n({
          legacy: false,
          locale: 'en',
          messages: {
            en: { [RouteName.TUTTI_LIFECYCLE]: 'Lifecycle rules' },
          },
        }),
      ],
    },
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  setActivePinia(createPinia());
  listRules.mockReset();
  listSummary.mockReset();
  listSummary.mockResolvedValue({
    storages: [{ name: 's3-main' }],
  } as unknown as import('@/utils/types/runtimeStorage').RuntimeStorageList);
});

type LifecyclePageVM = {
  formModel: {
    id: string;
    name: string;
    storageName: string;
    bucket: string;
    prefix: string;
    expirationDays: number;
    schedule: string;
    enabled: boolean;
  };
  isEditing: boolean;
  showEditModal: boolean;
  showAuditDrawer: boolean;
  auditLoading: boolean;
  auditLogs: LifecycleAuditLogEntry[];
  currentRule: LifecycleRule | null;
  handleSaveRule: () => Promise<void>;
  runDryRun: (
    storage: string,
    bucket: string,
    prefix: string,
    days: number,
  ) => Promise<void>;
  openAuditLogs: (rule: LifecycleRule) => Promise<void>;
};

const rule = (id: string): LifecycleRule => ({
  id,
  name: id,
  storageName: 's3-main',
  bucket: 'bucket',
  prefix: '',
  expirationDays: 7,
  schedule: '0 2 * * *',
  enabled: true,
  createdAt: '',
  updatedAt: '',
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

describe('TuttiLifecyclePage', () => {
  it('lets an administrator open the create form on an empty page', async () => {
    listRules.mockResolvedValue([]);
    const wrapper = mountPage();

    await flushPromises();
    const action = wrapper
      .findAll('button')
      .find((button) => button.text() === 'Add Lifecycle Rule');

    expect(action).toBeDefined();
    await action!.trigger('click');
    expect((wrapper.vm as unknown as LifecyclePageVM).showEditModal).toBe(true);
    wrapper.unmount();
  });

  it('keeps the create action inert for viewers', async () => {
    listRules.mockResolvedValue([]);
    const wrapper = mountPage(false);

    await flushPromises();
    expect(wrapper.get('.write-gate__inner[inert]').text()).toContain('Add Lifecycle Rule');
    wrapper.unmount();
  });

  it.each([' ', 'logs/ ', ' logs/'])(
    'preserves prefix %j in preview, create and update',
    async (prefix) => {
      listRules.mockResolvedValue([]);
      vi.mocked(LifecycleService.dryRun).mockResolvedValue({
        estimatedDeletedCount: '0',
        estimatedBytesFreed: '0',
      });
      const wrapper = mountPage();

      await flushPromises();
      const vm = wrapper.vm as unknown as LifecyclePageVM;

      vm.formModel = {
        id: '1',
        name: 'cleanup',
        storageName: 's3-main',
        bucket: 'bucket',
        prefix,
        expirationDays: 7,
        schedule: '0 2 * * *',
        enabled: true,
      };
      await vm.runDryRun('s3-main', 'bucket', prefix, 7);
      await vm.handleSaveRule();
      vm.isEditing = true;
      await vm.handleSaveRule();
      expect(LifecycleService.dryRun).toHaveBeenLastCalledWith(
        expect.objectContaining({ prefix }),
      );
      expect(LifecycleService.createRule).toHaveBeenLastCalledWith(
        expect.objectContaining({ prefix }),
      );
      expect(LifecycleService.updateRule).toHaveBeenLastCalledWith(
        expect.objectContaining({ prefix }),
      );
      wrapper.unmount();
    },
  );

  it('keeps the current audit response when an older request finishes last', async () => {
    listRules.mockResolvedValue([]);
    type AuditResponse = Awaited<
      ReturnType<typeof LifecycleService.listAuditLogs>
    >;
    const first = deferred<AuditResponse>();
    const second = deferred<AuditResponse>();

    vi.mocked(LifecycleService.listAuditLogs)
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const wrapper = mountPage();

    await flushPromises();
    const vm = wrapper.vm as unknown as LifecyclePageVM;
    const openingFirst = vm.openAuditLogs(rule('a'));
    const openingSecond = vm.openAuditLogs(rule('b'));

    first.resolve({
      entries: [{ id: 'a-log' } as LifecycleAuditLogEntry],
      total: '1',
    });
    await openingFirst;
    expect(vm.auditLogs).toEqual([]);
    expect(vm.auditLoading).toBe(true);
    second.resolve({
      entries: [{ id: 'b-log' } as LifecycleAuditLogEntry],
      total: '1',
    });
    await openingSecond;
    expect(vm.currentRule?.id).toBe('b');
    expect(vm.auditLogs[0]?.id).toBe('b-log');
    expect(vm.auditLoading).toBe(false);
    wrapper.unmount();
  });

  it('clears previous audit entries while the next rule loads', async () => {
    listRules.mockResolvedValue([]);
    vi.mocked(LifecycleService.listAuditLogs).mockResolvedValueOnce({
      entries: [{ id: 'old-log' } as LifecycleAuditLogEntry],
      total: '1',
    });
    const next =
      deferred<Awaited<ReturnType<typeof LifecycleService.listAuditLogs>>>();

    vi.mocked(LifecycleService.listAuditLogs).mockImplementationOnce(
      () => next.promise,
    );
    const wrapper = mountPage();

    await flushPromises();
    const vm = wrapper.vm as unknown as LifecyclePageVM;

    await vm.openAuditLogs(rule('a'));
    const opening = vm.openAuditLogs(rule('b'));

    expect(vm.auditLogs).toEqual([]);
    next.resolve({ entries: [], total: '0' });
    await opening;
    wrapper.unmount();
  });

  it('renders rule list when data is returned', async () => {
    listRules.mockResolvedValue([
      {
        id: '1',
        name: 'test-cleanup',
        storageName: 's3-main',
        bucket: 'my-bucket',
        prefix: 'logs/',
        expirationDays: 7,
        schedule: '0 2 * * *',
        enabled: true,
        createdAt: '2026-11-17T00:00:00Z',
        updatedAt: '2026-11-17T00:00:00Z',
      },
    ]);

    const wrapper = mountPage();

    await flushPromises();

    expect(wrapper.text()).toContain('Lifecycle Rules');
    expect(wrapper.text()).toContain('test-cleanup');
    expect(wrapper.text()).toContain('my-bucket');
    expect(wrapper.text()).toContain('7 days');
  });

  it('renders empty table when no rules exist', async () => {
    listRules.mockResolvedValue([]);

    const wrapper = mountPage();

    await flushPromises();

    expect(wrapper.text()).toContain('Lifecycle Rules');
  });
});
