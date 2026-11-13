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
  import { NButton } from 'naive-ui';
  import AuditEntryDrawer from './AuditEntryDrawer.vue';
  import AuditFilters from './AuditFilters.vue';
  import AuditTable from './AuditTable.vue';
  import { useAuditI18n } from './useAuditI18n';
  import { Callout, PageShell, StatusBadge } from '@/components/kit';
  import { useAuditLogStore } from '@/stores/tuttiAuditLogStore';
  import type { AuditLogEntry } from '@/utils/types/audit';

  const store = useAuditLogStore();
  const { t, locale } = useAuditI18n();
  const selected = ref<AuditLogEntry | null>(null);
  const state = computed(() => {
    if (store.loadFailed) return 'error';

    return store.loaded ? 'ready' : 'loading';
  });

  onBeforeMount(() => {
    void store.refresh();
  });
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('pageDescription')"
    :state="state"
    :error-text="t('errorText')"
    @retry="store.refresh()"
  >
    <template #actions>
      <NButton
        size="small"
        @click="store.toggleLocale(locale)"
      >
        {{ locale === 'vi' ? t('langSwitchEn') : t('langSwitchVi') }}
      </NButton>
      <NButton
        :loading="store.loading"
        @click="store.refresh()"
        >{{ t('refresh') }}</NButton
      >
    </template>
    <template #meta>
      <StatusBadge
        :tone="store.backend === 'postgres' ? 'success' : 'info'"
        :label="
          store.backend === 'postgres'
            ? t('backendPostgres')
            : t('backendRedis')
        "
      />
      <StatusBadge
        :tone="store.readOnly ? 'warning' : 'success'"
        :label="store.readOnly ? t('modeReadOnly') : t('modeReadWrite')"
      />
    </template>

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
      <AuditFilters />
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
