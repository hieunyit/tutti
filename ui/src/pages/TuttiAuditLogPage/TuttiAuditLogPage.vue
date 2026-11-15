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
  import { computed, onBeforeMount, ref } from 'vue';
  import AuditEntryDrawer from './AuditEntryDrawer.vue';
  import AuditFilters from './AuditFilters.vue';
  import AuditTable from './AuditTable.vue';
  import { useAuditI18n } from './useAuditI18n';
  import { Callout, PageShell } from '@/components/kit';
  import { useAuditLogStore } from '@/stores/tuttiAuditLogStore';
  import type { AuditLogEntry } from '@/utils/types/audit';

  const store = useAuditLogStore();
  const { t } = useAuditI18n();
  const selected = ref<AuditLogEntry | null>(null);
  const state = computed(() => {
    if (store.loadFailed) return 'error';

    return store.loaded ? 'ready' : 'loading';
  });
  // nothing recorded and nothing filtered: the toolbar would only be noise
  const bare = computed(
    () => store.entries.length === 0 && !store.hasActiveFilters,
  );

  onBeforeMount(() => {
    void store.refresh();
  });
</script>

<template>
  <PageShell
    :title="t('title')"
    :state="state"
    :error-text="t('errorText')"
    @retry="store.refresh()"
  >
    <div class="audit-page">
      <Callout
        v-if="store.backend === 'redis'"
        tone="info"
        :title="t('redisBannerTitle')"
      >
        {{ t('redisBannerText') }}
      </Callout>
      <Callout
        v-if="store.readOnly"
        tone="warning"
        :title="t('readOnlyBannerTitle')"
      >
        {{ t('readOnlyBannerText') }}
      </Callout>
      <AuditFilters :hidden="bare" />
      <AuditTable @open="selected = $event" />
    </div>
    <AuditEntryDrawer
      :entry="selected"
      @close="selected = null"
    />
  </PageShell>
</template>

<style lang="scss" scoped>
  .audit-page {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
  }
</style>
