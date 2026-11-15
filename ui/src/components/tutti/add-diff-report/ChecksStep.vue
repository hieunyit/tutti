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
   * Step 2: the user that reads both buckets (chosen for you when only one
   * fits) and what to compare. The defaults suit most checks.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NFormItem, NSelect, NSwitch } from 'naive-ui';
  import i18nAddDiffReport from './i18nAddDiffReport';
  import { Callout } from '@/components/kit';
  import { useTuttiAddDiffReportStore } from '@/stores/tuttiAddDiffReportStore';

  const { t } = useI18n({ messages: i18nAddDiffReport });
  const {
    users,
    selectedUser,
    checkOnlyLastVersions,
    ignoreEtags,
    ignoreSizes,
    validator,
  } = storeToRefs(useTuttiAddDiffReportStore());

  const userOptions = computed(() =>
    users.value.map((user) => ({ label: user, value: user })),
  );
  const userError = computed(() =>
    validator.value.selectedUser.$error
      ? String(validator.value.selectedUser.$errors[0]?.$message ?? '')
      : undefined,
  );
  const checks = computed(() => [
    { id: 'last', model: checkOnlyLastVersions, label: 'lastVersions' },
    { id: 'etags', model: ignoreEtags, label: 'ignoreEtags' },
    { id: 'sizes', model: ignoreSizes, label: 'ignoreSizes' },
  ]);
</script>

<template>
  <div class="checks-step">
    <Callout
      v-if="users.length === 0"
      tone="warning"
    >
      {{ t('noUsers') }}
    </Callout>
    <NFormItem
      v-else
      :label="t('userLabel')"
      :feedback="userError ?? t('userHint')"
      :validation-status="userError ? 'error' : undefined"
    >
      <NSelect
        v-model:value="selectedUser"
        :options="userOptions"
        filterable
        @blur="validator.selectedUser.$touch()"
      />
    </NFormItem>

    <fieldset class="checks-step__group">
      <legend>{{ t('checksLegend') }}</legend>
      <div
        v-for="check in checks"
        :key="check.id"
        class="checks-step__item"
      >
        <label class="checks-step__label">
          <NSwitch
            :value="check.model.value"
            size="small"
            @update:value="check.model.value = $event"
          />
          <span>{{ t(check.label) }}</span>
        </label>
        <p class="checks-step__hint">{{ t(`${check.label}Hint`) }}</p>
      </div>
    </fieldset>
  </div>
</template>

<style scoped lang="scss">
  .checks-step {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    max-width: 520px;

    &__group {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-3);
      margin: 0;
      padding: 0;
      border: 0;

      legend {
        margin-bottom: var(--tt-space-2);
        padding: 0;
        font-weight: var(--tt-weight-strong);
      }
    }

    &__label {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      cursor: pointer;
    }

    &__hint {
      margin: var(--tt-space-1) 0 0 calc(var(--tt-space-2) + 32px);
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }
  }
</style>
