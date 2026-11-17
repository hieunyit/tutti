/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ReplicationPoliciesCard from '../ReplicationPoliciesCard.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { useAuthStore } from '@/stores/authStore';
import {
  DeletePropagationMode,
  ReplicationType,
  type TuttiReplication,
  type TuttiReplicationOpts,
  type TuttiReplicationSwitchInfo,
} from '@/utils/types/tuttiCore';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({ createNotification: vi.fn(), removeNotification: vi.fn() }),
}));

vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: {
    updateDeletePropagation: vi.fn().mockResolvedValue(undefined),
    updateReplicationFilters: vi.fn().mockResolvedValue(undefined),
    listDeleteEvents: vi.fn().mockResolvedValue({ events: [], total: '0' }),
    retryFailedDeletes: vi.fn().mockResolvedValue(undefined),
  },
}));

const mockReplication: TuttiReplication = {
  id: {
    user: 'u1',
    fromStorage: 's1',
    toStorage: 's2',
    fromBucket: 'b1',
    toBucket: 'b1',
  },
  idStr: '{"user":"u1","fromStorage":"s1","toStorage":"s2","fromBucket":"b1","toBucket":"b1"}',
  opts: {} as unknown as TuttiReplicationOpts,
  createdAt: '2026-11-17T00:00:00Z',
  isPaused: false,
  isInitDone: true,
  initObjListed: '10',
  initObjDone: '10',
  events: '0',
  eventsDone: '0',
  eventLag: '0s',
  hasSwitch: false,
  isArchived: false,
  archivedAt: '',
  switchInfo: {} as unknown as TuttiReplicationSwitchInfo,
  replicationType: ReplicationType.BUCKET,
  deletePropagation: {
    mode: 'DELETE_PROPAGATION_SYNC',
    includePrefixes: ['data/'],
    excludePrefixes: ['logs/'],
    delayBeforeDelete: '120s',
  },
  includePatterns: ['*.jpg'],
  excludePatterns: ['*.tmp'],
};

let pinia: ReturnType<typeof createPinia>;

function mountComponent(props: { replication: TuttiReplication; reload: () => Promise<void> }) {
  return mount(ReplicationPoliciesCard, {
    props,
    global: {
      plugins: [
        pinia,
        createI18n({
          legacy: false,
          locale: 'en',
          messages: { en: {} },
        }),
      ],
    },
  });
}

describe('ReplicationPoliciesCard', () => {
  beforeEach(() => {
    pinia = createPinia();
    setActivePinia(pinia);
    const authStore = useAuthStore(pinia);

    authStore.authEnabled = false;
    vi.clearAllMocks();
  });

  it('renders correctly with nested deletePropagation policy', async () => {
    const reload = vi.fn().mockResolvedValue(undefined);
    const wrapper = mountComponent({
      replication: mockReplication,
      reload,
    });

    await flushPromises();

    expect(wrapper.text()).toContain('Delete Propagation Policy');
    expect(wrapper.text()).toContain('Advanced Pattern Filtering');
    expect(wrapper.text()).toContain('data/');
    expect(wrapper.text()).toContain('logs/');
    expect(wrapper.text()).toContain('*.jpg');
    expect(wrapper.text()).toContain('*.tmp');
  });

  it('handles legacy flat string deletePropagation gracefully', async () => {
    const legacyRep: TuttiReplication = {
      ...mockReplication,
      deletePropagation: 'marker-only',
      deleteIncludePrefixes: ['docs/'],
      deleteExcludePrefixes: ['cache/'],
      deleteDelaySeconds: 60,
    };

    const reload = vi.fn().mockResolvedValue(undefined);
    const wrapper = mountComponent({
      replication: legacyRep,
      reload,
    });

    await flushPromises();

    expect(wrapper.text()).toContain('docs/');
    expect(wrapper.text()).toContain('cache/');
  });

  it('formats delay duration with "s" suffix when saving', async () => {
    const reload = vi.fn().mockResolvedValue(undefined);
    const wrapper = mountComponent({
      replication: mockReplication,
      reload,
    });

    await flushPromises();

    // Trigger save on delete policy
    const buttons = wrapper.findAll('button');
    const deleteSaveBtn = buttons.find((b) => b.text().includes('Save Delete Policy'));

    expect(deleteSaveBtn).toBeDefined();

    // Call handleSaveDeletePropagation directly via component instance
    await (wrapper.vm as unknown as { handleSaveDeletePropagation: () => Promise<void> }).handleSaveDeletePropagation();

    expect(TuttiCoreService.updateDeletePropagation).toHaveBeenCalledWith({
      id: mockReplication.id,
      deletePropagation: {
        mode: DeletePropagationMode.SYNC,
        includePrefixes: ['data/'],
        excludePrefixes: ['logs/'],
        delayBeforeDelete: '120s',
      },
    });
    expect(reload).toHaveBeenCalled();
  });
});
