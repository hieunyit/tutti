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
  import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
  import { onBeforeRouteLeave } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton } from 'naive-ui';
  import {
    Callout,
    ConfirmDialog,
    PageShell,
    StatusBadge,
    useQueryTab,
    type PageShellTab,
  } from '@/components/kit';
  import SettingsConflictDialog from '@/components/tutti/settings/SettingsConflictDialog.vue';
  import SettingsPanels from '@/components/tutti/settings/SettingsPanels.vue';
  import SettingsSaveBar from '@/components/tutti/settings/SettingsSaveBar.vue';
  import messages from '@/components/tutti/settings/i18nSettings';
  import {
    SETTINGS_TABS,
    tabForField,
    type SettingsTab,
  } from '@/components/tutti/settings/settingsMeta';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const tab = useQueryTab('tab', 'features');
  const activeTab = computed<SettingsTab>(() =>
    (SETTINGS_TABS as readonly string[]).includes(tab.value)
      ? (tab.value as SettingsTab)
      : 'features',
  );
  const busy = computed(() => store.isLoading || store.isSaving);
  const state = computed(() => {
    if (store.settings) return 'ready';

    if (store.hasError) return 'error';

    return 'loading';
  });
  const tabLabels: Record<SettingsTab, string> = {
    features: 'tabFeatures',
    bulk: 'tabBulk',
    worker: 'tabWorker',
    locking: 'tabLocking',
    webhook: 'tabWebhook',
  };
  const tabs = computed<PageShellTab[]>(() =>
    SETTINGS_TABS.map((key) => ({
      key,
      label: t(tabLabels[key]),
      badge:
        Object.keys(store.validationErrors).filter(
          (field) => tabForField(field) === key,
        ).length || undefined,
    })),
  );

  async function reviewInvalidFields() {
    const field = Object.keys(store.validationErrors)[0];

    if (!field) return;

    tab.value = tabForField(field);
    await nextTick();
    document.getElementById(`settings-${field}`)?.focus();
  }

  const showDiscard = ref(false);
  let resolveDiscard: ((confirmed: boolean) => void) | undefined;

  function finishDiscard(confirmed: boolean) {
    showDiscard.value = false;
    resolveDiscard?.(confirmed);
    resolveDiscard = undefined;
  }

  function confirmDiscard(): Promise<boolean> {
    resolveDiscard?.(false);
    showDiscard.value = true;

    return new Promise((resolve) => {
      resolveDiscard = resolve;
    });
  }

  async function refresh() {
    if (busy.value) return;

    if (store.hasChanges && !(await confirmDiscard())) return;

    await store.load();
  }

  async function discard() {
    if (await confirmDiscard()) store.resetDraft();
  }

  function beforeUnload(event: BeforeUnloadEvent) {
    if (!store.hasChanges) return;

    event.preventDefault();
    event.returnValue = '';
  }

  onBeforeRouteLeave(async () => {
    if (store.isSaving) return false;

    if (!store.hasChanges) return true;

    if (!(await confirmDiscard())) return false;

    store.resetDraft();

    return true;
  });

  onMounted(() => {
    if (!store.settings) void store.load();

    window.addEventListener('beforeunload', beforeUnload);
  });
  onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', beforeUnload);
    finishDiscard(false);
  });
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('subtitle')"
    :state="state"
    :tabs="tabs"
    :active-tab="activeTab"
    :error-text="t('loadFailed')"
    @update:active-tab="tab = $event"
    @retry="store.load()"
  >
    <template #actions>
      <NButton
        :disabled="busy"
        :loading="store.isLoading"
        @click="refresh"
        >{{ t('refresh') }}</NButton
      >
    </template>
    <template
      v-if="store.settings"
      #meta
    >
      <StatusBadge
        tone="neutral"
        :label="`${t('savedVersion')} ${store.settings.version ?? 0}`"
      />
      <StatusBadge
        v-if="store.hasChanges"
        tone="warning"
        :label="t('unsavedChanges')"
      />
    </template>

    <p
      v-if="store.settings"
      class="settings-page__updated"
    >
      {{ t('lastUpdated') }}:
      {{
        store.settings.updatedAt
          ? formatDateTime(store.settings.updatedAt)
          : t('never')
      }}
    </p>
    <Callout
      v-if="store.hasError && !store.conflictError"
      tone="danger"
      :title="t('bannerFailed')"
    >
      {{ store.errorMessage }}
    </Callout>
    <SettingsConflictDialog />
    <form
      class="settings-page__form"
      :aria-busy="busy"
      @submit.prevent
    >
      <SettingsPanels
        :tab="activeTab"
        :disabled="busy"
      />
      <p class="settings-page__note">{{ t('effectiveNotice') }}</p>
      <SettingsSaveBar
        @discard="discard"
        @invalid="reviewInvalidFields"
      />
    </form>

    <ConfirmDialog
      :open="showDiscard"
      :title="t('discardTitle')"
      :confirm-label="t('discardConfirm')"
      danger
      @confirm="finishDiscard(true)"
      @cancel="finishDiscard(false)"
    >
      {{ t('discardText') }}
    </ConfirmDialog>
  </PageShell>
</template>

<style lang="scss" scoped>
  .settings-page {
    &__updated {
      margin: 0 0 var(--tt-space-3);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__form {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
      min-width: 0;
    }

    &__note {
      margin: 0;
      max-width: var(--tt-measure);
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }
  }
</style>
