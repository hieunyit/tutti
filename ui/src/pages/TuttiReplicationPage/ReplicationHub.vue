<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->
<script setup lang="ts">
  /**
   * Replication hub: replications, routing policies and diff reports as three
   * query-synced tabs (?tab=routing | ?tab=diff). Each tab keeps its store and
   * owns its loading, empty and error states.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import DiffTab from './DiffTab.vue';
  import ReplicationsTab from './ReplicationsTab.vue';
  import RoutingTab from './RoutingTab.vue';
  import i18nReplicationHub from './i18nReplicationHub';
  import { RouteName } from '@/utils/types/router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { PageShell, useQueryTab, type PageShellTab } from '@/components/kit';

  const TAB_KEYS = ['replications', 'routing', 'diff'] as const;

  const { t } = useI18n({ messages: i18nReplicationHub });
  const queryTab = useQueryTab('tab', 'replications');

  const activeTab = computed<string>({
    get: () =>
      TAB_KEYS.includes(queryTab.value as never)
        ? queryTab.value
        : 'replications',
    set: (value) => {
      queryTab.value = value;
    },
  });

  const tabs = computed<PageShellTab[]>(() => [
    { key: 'replications', label: t('tabReplications') },
    { key: 'routing', label: t('tabRouting') },
    { key: 'diff', label: t('tabDiff') },
  ]);

  const addAction = computed(() => {
    if (activeTab.value === 'routing')
      return {
        to: RouteName.CHORUS_ADD_ROUTING_POLICY,
        label: t('addRoutingPolicy'),
      };

    if (activeTab.value === 'diff')
      return {
        to: RouteName.CHORUS_ADD_DIFF_REPORT,
        label: t('addDiffReport'),
      };

    return { to: RouteName.CHORUS_ADD_REPLICATION, label: t('addReplication') };
  });
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('description')"
    state="ready"
    :tabs="tabs"
    :active-tab="activeTab"
    @update:active-tab="activeTab = $event"
  >
    <template #actions>
      <WriteGate>
        <RouterLink :to="{ name: addAction.to }">
          <NButton
            type="primary"
            tag="span"
          >
            {{ addAction.label }}
          </NButton>
        </RouterLink>
      </WriteGate>
    </template>

    <RoutingTab v-if="activeTab === 'routing'" />
    <DiffTab v-else-if="activeTab === 'diff'" />
    <ReplicationsTab v-else />
  </PageShell>
</template>
