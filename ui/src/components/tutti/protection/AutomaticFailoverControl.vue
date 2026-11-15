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
  import { NSwitch } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtection';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { ConfirmDialog, ErrorBlock } from '@/components/kit';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';

  /**
   * The global automatic-failover switch (one setting for all groups), shown
   * as a small labelled control next to the list toolbar, not in the header.
   */
  const { t } = useI18n({ messages: i18nProtection });
  const store = useTuttiProtectionStore();
  const { automaticFrozen } = storeToRefs(store);

  const showConfirm = ref(false);
  const isSaving = ref(false);
  const failed = ref(false);

  async function confirm() {
    isSaving.value = true;
    failed.value = false;

    const err = await store.setAutomaticFreeze(!automaticFrozen.value);

    isSaving.value = false;
    showConfirm.value = false;
    failed.value = err !== null;
  }
</script>

<template>
  <div
    v-if="automaticFrozen !== null"
    class="pg-failover"
  >
    <label class="pg-failover__control">
      <span id="pg-failover-label">{{ t('freezeSwitch') }}</span>
      <WriteGate>
        <NSwitch
          :value="!automaticFrozen"
          :loading="isSaving"
          aria-labelledby="pg-failover-label"
          @update:value="showConfirm = true"
        />
      </WriteGate>
      <strong>{{
        automaticFrozen ? t('freezeSwitchOff') : t('freezeSwitchOn')
      }}</strong>
    </label>
    <p class="pg-failover__help">{{ t('freezeHelp') }}</p>
    <ErrorBlock
      v-if="failed"
      :title="t('freezeErrorTitle')"
      :description="t('freezeFailed')"
      @retry="showConfirm = true"
    />
    <ConfirmDialog
      :open="showConfirm"
      :title="
        automaticFrozen ? t('freezeDisableTitle') : t('freezeEnableTitle')
      "
      :danger="!automaticFrozen"
      :loading="isSaving"
      :confirm-label="
        automaticFrozen ? t('freezeDisableAction') : t('freezeEnableAction')
      "
      @confirm="confirm"
      @cancel="showConfirm = false"
    >
      {{ automaticFrozen ? t('freezeDisableText') : t('freezeEnableText') }}
    </ConfirmDialog>
  </div>
</template>

<style scoped lang="scss">
  .pg-failover {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);

    &__control {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);

      strong {
        color: var(--tt-text);
        font-weight: var(--tt-weight-medium);
      }
    }

    &__help {
      margin: 0;
      max-width: var(--tt-measure);
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }
  }
</style>
