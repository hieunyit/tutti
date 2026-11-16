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
   * Optional "which objects to copy" choice: all (default), the last N days or
   * hours, or since a date. Errors show after blur and clear as soon as the
   * value is fixed. The absolute timestamp is only computed on Create.
   */
  import { computed, ref, useId } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    NDatePicker,
    NInputNumber,
    NRadio,
    NRadioGroup,
    NSelect,
  } from 'naive-ui';
  import i18nReplicationSince from '@/components/tutti/replications/i18nReplicationSince';
  import {
    ReplicationSinceHelper as H,
    type SinceChoice,
    type SinceMode,
    type SinceUnit,
  } from '@/utils/helpers/ReplicationSinceHelper';

  const { t, locale } = useI18n({ messages: i18nReplicationSince });
  const model = defineModel<SinceChoice>({ required: true });
  const uid = useId();
  const touched = ref(false);

  const unitOptions = computed(() => [
    { label: t('unitDays'), value: 'days' },
    { label: t('unitHours'), value: 'hours' },
  ]);

  const problem = computed(() => H.getProblem(model.value));
  const error = computed(() => {
    if (!touched.value || !problem.value) return undefined;

    const keys = {
      amount: 'sinceAmountError',
      future: 'sinceFutureError',
      tooOld: 'sinceTooOldError',
      dateMissing: 'sinceDateMissing',
    } as const;

    return t(keys[problem.value]);
  });
  const computedLine = computed(() => {
    if (model.value.mode !== 'recent' || problem.value) return '';

    const moment = H.momentOf(model.value, Date.now());

    return moment === null
      ? ''
      : t('sinceComputed', {
          date: H.formatMoment(moment, String(locale.value)),
        });
  });

  function patch(change: Partial<SinceChoice>) {
    model.value = { ...model.value, ...change };
  }

  function setMode(mode: SinceMode) {
    touched.value = false;
    patch({ mode });
  }

  function isFuture(ts: number) {
    return ts > Date.now();
  }
</script>

<template>
  <fieldset class="since">
    <legend class="since__legend">{{ t('sinceLabel') }}</legend>
    <NRadioGroup
      :value="model.mode"
      name="since-mode"
      class="since__modes"
      @update:value="setMode"
    >
      <NRadio value="all">{{ t('sinceAll') }}</NRadio>
      <NRadio value="recent">{{ t('sinceRecent') }}</NRadio>
      <NRadio value="date">{{ t('sinceDate') }}</NRadio>
    </NRadioGroup>

    <div
      v-if="model.mode === 'recent'"
      class="since__row"
    >
      <div class="since__field">
        <label :for="`since-amount-${uid}`">{{ t('sinceAmount') }}</label>
        <NInputNumber
          :value="model.amount"
          :precision="0"
          :show-button="false"
          :status="error ? 'error' : undefined"
          :input-props="{
            id: `since-amount-${uid}`,
            'aria-describedby': `since-help-${uid}`,
            'aria-invalid': error ? 'true' : undefined,
          }"
          @update:value="(value) => patch({ amount: value })"
          @blur="touched = true"
        />
      </div>
      <div class="since__field">
        <label :for="`since-unit-${uid}`">{{ t('sinceUnit') }}</label>
        <NSelect
          :value="model.unit"
          :options="unitOptions"
          :input-props="{ id: `since-unit-${uid}` }"
          @update:value="(value: SinceUnit) => patch({ unit: value })"
        />
      </div>
    </div>

    <div
      v-else-if="model.mode === 'date'"
      class="since__field"
    >
      <label :for="`since-date-${uid}`">{{ t('sinceDateTime') }}</label>
      <NDatePicker
        :value="model.date"
        type="datetime"
        clearable
        :is-date-disabled="isFuture"
        :status="error ? 'error' : undefined"
        :input-props="{
          id: `since-date-${uid}`,
          'aria-describedby': `since-help-${uid}`,
          'aria-invalid': error ? 'true' : undefined,
        }"
        @update:value="
          (value: number | null) => {
            patch({ date: value });
            touched = true;
          }
        "
        @blur="touched = true"
      />
    </div>

    <p
      v-if="model.mode !== 'all'"
      :id="`since-help-${uid}`"
      class="since__help"
      :class="{ 'since__help--error': error }"
      :role="error ? 'alert' : undefined"
    >
      {{ error ?? (computedLine || t('sinceHint')) }}
    </p>
  </fieldset>
</template>

<style lang="scss" scoped>
  .since {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    min-width: 0;
    max-width: 420px;
    margin: 0;
    padding: 0;
    border: 0;

    &__legend {
      padding: 0;
      margin-bottom: var(--tt-space-2);
      font-weight: 600;
    }

    &__modes {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2) var(--tt-space-4);
    }

    &__row {
      display: grid;
      grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
      gap: var(--tt-space-3);
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      min-width: 0;
    }

    &__help {
      margin: 0;
      font-size: 0.875rem;
      opacity: 0.8;

      &--error {
        opacity: 1;
        color: var(--tt-danger, #d03050);
      }
    }
  }
</style>
