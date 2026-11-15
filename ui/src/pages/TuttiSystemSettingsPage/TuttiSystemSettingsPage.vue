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
  import {
    Callout,
    ConfirmDialog,
    PageShell,
    useQueryTab,
    type PageShellTab,
  } from '@/components/kit';
  import ProxyConfigTab from '@/pages/TuttiProxiesPage/ProxyConfigTab.vue';
  import SettingsConflictDialog from '@/components/tutti/settings/SettingsConflictDialog.vue';
  import SettingsBulkTab from '@/components/tutti/settings/SettingsBulkTab.vue';
  import SettingsFeaturesTab from '@/components/tutti/settings/SettingsFeaturesTab.vue';
  import SettingsLockingTab from '@/components/tutti/settings/SettingsLockingTab.vue';
  import SettingsWebhookTab from '@/components/tutti/settings/SettingsWebhookTab.vue';
  import SettingsWorkerTab from '@/components/tutti/settings/SettingsWorkerTab.vue';
  import SettingsSaveBar from '@/components/tutti/settings/SettingsSaveBar.vue';
  import messages from '@/components/tutti/settings/i18nSettings';
  import {
    SETTINGS_TABS,
    tabForField,
    type SettingsTab,
  } from '@/components/tutti/settings/settingsMeta';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const tab = useQueryTab('tab', 'features');
  const activeTab = computed<SettingsTab>(() =>
    (SETTINGS_TABS as readonly string[]).includes(tab.value)
      ? (tab.value as SettingsTab)
      : 'features',
  );
  const busy = computed(() => store.isLoading || store.isSaving);
  // the Proxies tab has its own store: it must not wait for the settings load
  const isProxies = computed(() => activeTab.value === 'proxies');
  const state = computed(() => {
    if (store.settings || isProxies.value) return 'ready';

    if (store.hasError) return 'error';

    return 'loading';
  });
  const tabLabels: Record<SettingsTab, string> = {
    features: 'tabFeatures',
    worker: 'tabWorker',
    bulk: 'tabBulk',
    locking: 'tabLocking',
    webhook: 'tabWebhook',
    proxies: 'tabProxies',
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
    :state="state"
    :tabs="tabs"
    :active-tab="activeTab"
    :error-text="t('loadFailed')"
    @update:active-tab="tab = $event"
    @retry="store.load()"
  >
    <ProxyConfigTab v-if="isProxies" />
    <template v-else>
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
        <SettingsFeaturesTab
          v-if="activeTab === 'features'"
          :disabled="busy"
        />
        <SettingsWorkerTab
          v-else-if="activeTab === 'worker'"
          :disabled="busy"
        />
        <SettingsBulkTab
          v-else-if="activeTab === 'bulk'"
          :disabled="busy"
        />
        <SettingsLockingTab
          v-else-if="activeTab === 'locking'"
          :disabled="busy"
        />
        <SettingsWebhookTab
          v-else
          :disabled="busy"
        />
        <SettingsSaveBar
          @discard="discard"
          @invalid="reviewInvalidFields"
        />
      </form>
    </template>

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
    &__form {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
      min-width: 0;
    }
  }
</style>
