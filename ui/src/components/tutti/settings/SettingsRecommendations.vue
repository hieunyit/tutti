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
   * Collapsible "Recommended values" helper: suggestions for the hardware the
   * operator enters. Values are only copied into the draft by "Fill in".
   */
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInputNumber } from 'naive-ui';
  import messages from './i18nSettings';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import {
    recommendSettings,
    type RecommendedSettings,
  } from '@/utils/helpers/SystemSettingsHelper';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';

  defineProps<{ disabled: boolean }>();

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const cores = ref<number | null>(4);
  const ramGb = ref<number | null>(32);
  const rec = computed(() =>
    recommendSettings(cores.value ?? 4, ramGb.value ?? 32),
  );

  const rows = computed<
    { key: keyof RecommendedSettings; text: string; value: string }[]
  >(() => [
    {
      key: 'concurrency',
      text: t('recConcurrency'),
      value: String(rec.value.concurrency),
    },
    {
      key: 'bufferBudgetBytes',
      text: t('recBuffer'),
      value: `${rec.value.bufferBudgetBytes} (${TrafficStatsHelper.formatBytes(Number(rec.value.bufferBudgetBytes))})`,
    },
    {
      key: 'multipartThreads',
      text: t('recThreads'),
      value: String(rec.value.multipartThreads),
    },
    {
      key: 'bulkConcurrencyPercent',
      text: t('recBulk'),
      value: `${rec.value.bulkConcurrencyPercent}%`,
    },
  ]);

  function fill(key: keyof RecommendedSettings) {
    const worker = store.draft.worker!;

    if (key === 'bufferBudgetBytes') worker.bufferBudgetBytes = rec.value[key];
    else worker[key] = rec.value[key];
  }
</script>

<template>
  <details
    class="settings-rec"
    data-test="recommendations"
  >
    <summary class="settings-rec__summary">{{ t('recTitle') }}</summary>
    <p class="settings-rec__intro">{{ t('recIntro') }}</p>
    <div class="settings-rec__inputs">
      <label class="settings-rec__input">
        <span>{{ t('recCores') }}</span>
        <NInputNumber
          v-model:value="cores"
          :min="1"
          :max="512"
          :precision="0"
          :input-props="{ id: 'settings-rec-cores' }"
        />
      </label>
      <label class="settings-rec__input">
        <span>{{ t('recRam') }}</span>
        <NInputNumber
          v-model:value="ramGb"
          :min="1"
          :max="4096"
          :precision="0"
          :input-props="{ id: 'settings-rec-ram' }"
        />
      </label>
    </div>
    <ul class="settings-rec__list">
      <li
        v-for="row in rows"
        :key="row.key"
        class="settings-rec__row"
      >
        <span
          >{{ row.text }} <strong class="tt-mono">{{ row.value }}</strong></span
        >
        <NButton
          size="small"
          :disabled="disabled"
          :data-test="`fill-${row.key}`"
          @click="fill(row.key)"
          >{{ t('recFill') }}</NButton
        >
      </li>
    </ul>
  </details>
</template>

<style lang="scss" scoped>
  .settings-rec {
    padding-top: var(--tt-space-3);
    font-size: var(--tt-text-sm);

    &__summary {
      cursor: pointer;
      font-weight: var(--tt-weight-medium);
    }

    &__intro {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }

    &__inputs {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-4);
    }

    &__input {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
    }

    &__list {
      margin: var(--tt-space-3) 0 0;
      padding: 0;
      list-style: none;
    }

    &__row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-3);
      padding: var(--tt-space-2) 0;
      border-bottom: 1px solid var(--tt-border-subtle);
    }
  }
</style>
