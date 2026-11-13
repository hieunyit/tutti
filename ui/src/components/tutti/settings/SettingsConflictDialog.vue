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
   * Version conflict (someone saved first): "Reload" drops the draft and loads
   * the newer version, "Overwrite" saves the draft on top of it. Closing the
   * dialog keeps the draft; the banner below the header keeps both actions.
   */
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NModal } from 'naive-ui';
  import messages from './i18nSettings';
  import { Callout } from '@/components/kit';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const dismissed = ref(false);

  watch(
    () => store.conflictError,
    (conflict) => {
      if (conflict) dismissed.value = false;
    },
  );
</script>

<template>
  <template v-if="store.conflictError">
    <Callout
      tone="warning"
      :title="t('conflictTitle')"
      data-test="conflict-banner"
    >
      {{ t('draftRetained') }}
      <template #actions>
        <NButton
          :disabled="store.isLoading || store.isSaving"
          @click="store.load()"
          >{{ t('reload') }}</NButton
        >
        <NButton
          type="warning"
          :disabled="store.isLoading || store.isSaving"
          @click="store.overwrite()"
          >{{ t('overwrite') }}</NButton
        >
      </template>
    </Callout>
    <NModal
      :show="!dismissed"
      preset="card"
      role="alertdialog"
      :title="t('conflictTitle')"
      :style="{ width: '460px', maxWidth: 'calc(100vw - 32px)' }"
      @update:show="dismissed = true"
    >
      <div data-test="conflict">
        <p>{{ t('conflictText') }}</p>
        <div class="settings-conflict__actions">
          <NButton
            data-test="conflict-reload"
            :loading="store.isLoading"
            @click="store.load()"
            >{{ t('reload') }}</NButton
          >
          <NButton
            type="warning"
            data-test="conflict-overwrite"
            :loading="store.isSaving"
            @click="store.overwrite()"
            >{{ t('overwrite') }}</NButton
          >
        </div>
      </div>
    </NModal>
  </template>
</template>

<style lang="scss" scoped>
  .settings-conflict__actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--tt-space-2);
  }
</style>
