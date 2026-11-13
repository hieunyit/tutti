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
  import {
    NCheckbox,
    NFormItem,
    NInputNumber,
    NRadio,
    NSelect,
    NSpace,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtectionUi';
  import { FormSection } from '@/components/kit';
  import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
  import type {
    ProtectionFormError,
    ProtectionGroupForm,
  } from '@/utils/types/protection';

  defineProps<{
    errorText: (...codes: ProtectionFormError[]) => string | undefined;
  }>();

  const form = defineModel<ProtectionGroupForm>({ required: true });
  const { t } = useI18n({ messages: i18nProtection });
  const { proxyIds } = storeToRefs(useTuttiProtectionStore());

  const proxyOptions = computed(() =>
    proxyIds.value.map((id) => ({ label: id, value: id })),
  );
</script>

<template>
  <div>
    <FormSection :title="t('wizFailoverTitle')">
      <NFormItem :label="t('fieldMode')">
        <NSpace
          vertical
          size="small"
        >
          <NRadio
            :checked="form.mode === 'manual'"
            value="manual"
            @update:checked="form.mode = 'manual'"
          >
            {{ t('fieldModeManual') }}
          </NRadio>
          <NRadio
            :checked="form.mode === 'auto'"
            value="auto"
            @update:checked="form.mode = 'auto'"
          >
            {{ t('fieldModeAuto') }}
          </NRadio>
        </NSpace>
      </NFormItem>
      <NFormItem
        v-if="form.mode === 'auto'"
        :label="t('fieldElectorate')"
        :validation-status="errorText('ELECTORATE_MIN') ? 'error' : undefined"
        :feedback="errorText('ELECTORATE_MIN') ?? t('fieldElectorateHelp')"
      >
        <NSelect
          v-model:value="form.electorate"
          :options="proxyOptions"
          multiple
          filterable
          tag
        />
      </NFormItem>
    </FormSection>

    <FormSection :title="t('wizOptionsTitle')">
      <NFormItem
        :label="t('fieldAfter')"
        :feedback="errorText('AFTER_NEGATIVE') ?? t('fieldAfterHelp')"
        :validation-status="errorText('AFTER_NEGATIVE') ? 'error' : undefined"
      >
        <NInputNumber
          v-model:value="form.afterSeconds"
          :min="0"
          :precision="0"
          placeholder="30"
          clearable
        />
      </NFormItem>
      <NFormItem
        :label="t('fieldMaxLag')"
        :feedback="errorText('LAG_NEGATIVE') ?? t('fieldMaxLagHelp')"
        :validation-status="errorText('LAG_NEGATIVE') ? 'error' : undefined"
      >
        <NInputNumber
          v-model:value="form.maxEventLag"
          :min="0"
          :precision="0"
          placeholder="0"
          clearable
        />
      </NFormItem>
      <NFormItem
        :label="t('fieldSwitches')"
        :feedback="errorText('SWITCHES_POSITIVE') ?? t('fieldSwitchesHelp')"
        :validation-status="
          errorText('SWITCHES_POSITIVE') ? 'error' : undefined
        "
      >
        <NInputNumber
          v-model:value="form.maxSwitchesPerHour"
          :min="1"
          :precision="0"
          placeholder="3"
          clearable
        />
      </NFormItem>
      <NCheckbox v-model:checked="form.adoptExisting">
        {{ t('fieldAdopt') }}
      </NCheckbox>
    </FormSection>
  </div>
</template>
