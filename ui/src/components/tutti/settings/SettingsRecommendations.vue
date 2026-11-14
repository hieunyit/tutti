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
   * Collapsible "Recommended values" helper. Shows the hardware the worker
   * detected (cgroup / Kubernetes limits included) and the values derived
   * from it. Values are only copied into the draft by "Fill in".
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
  // Optional preview for other hardware; empty = use the detected hardware.
  const cores = ref<number | null>(null);
  const ramGb = ref<number | null>(null);

  const hardware = computed(() => store.settings?.hardware);
  const detected = computed(() => {
    const hw = hardware.value;

    if (!hw?.cpu || !hw.memoryBytes) return null;

    const cpu =
      hw.cpuFractional && hw.cpuFractional !== hw.cpu
        ? `${hw.cpu} (${t('recDetectedFractional', { cpu: hw.cpuFractional })})`
        : String(hw.cpu);

    return t('recDetected', {
      cpu,
      mem: TrafficStatsHelper.formatBytes(Number(hw.memoryBytes)),
      source: hw.source ?? 'host',
    });
  });

  const rec = computed<RecommendedSettings | null>(() => {
    if (cores.value || ramGb.value) {
      return recommendSettings(
        cores.value ?? hardware.value?.cpu ?? 4,
        ramGb.value ?? Number(hardware.value?.memoryBytes ?? 0) / 1024 ** 3,
      );
    }

    const r = store.settings?.recommended;

    if (!r) return null;

    return {
      concurrency: r.concurrency ?? 0,
      bufferBudgetBytes: r.bufferBudgetBytes ?? '0',
      multipartThreads: r.multipartThreads ?? 1,
      bulkConcurrencyPercent: r.bulkConcurrencyPercent ?? 0,
    };
  });

  const rows = computed<
    { key: keyof RecommendedSettings; text: string; value: string }[]
  >(() => {
    const r = rec.value;

    if (!r) return [];

    return [
      {
        key: 'concurrency',
        text: t('recConcurrency'),
        value: String(r.concurrency),
      },
      {
        key: 'bufferBudgetBytes',
        text: t('recBuffer'),
        value: `${r.bufferBudgetBytes} (${TrafficStatsHelper.formatBytes(Number(r.bufferBudgetBytes))})`,
      },
      {
        key: 'multipartThreads',
        text: t('recThreads'),
        value: String(r.multipartThreads),
      },
      {
        key: 'bulkConcurrencyPercent',
        text: t('recBulk'),
        value: `${r.bulkConcurrencyPercent}%`,
      },
    ];
  });

  function fill(key: keyof RecommendedSettings) {
    const worker = store.draft.worker!;

    if (!rec.value) return;

    if (key === 'bufferBudgetBytes')
      worker.bufferBudgetBytes = rec.value.bufferBudgetBytes;
    else worker[key] = rec.value[key];
  }

  /** 0 = auto: the worker derives these from its hardware. */
  function useAuto() {
    const worker = store.draft.worker!;

    worker.concurrency = 0;
    worker.bufferBudgetBytes = '0';
    worker.bulkConcurrencyPercent = 0;
    worker.multipartThreads = 0;
    worker.multipartPartSizeMib = 0;
  }
</script>

<template>
  <details
    class="settings-rec"
    data-test="recommendations"
    open
  >
    <summary class="settings-rec__summary">{{ t('recTitle') }}</summary>
    <p class="settings-rec__intro">{{ t('recIntro') }}</p>
    <p
      class="settings-rec__detected"
      data-test="detected-hardware"
    >
      {{ detected ?? t('recUnavailable') }}
    </p>
    <details class="settings-rec__override">
      <summary>{{ t('recOverride') }}</summary>
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
    </details>
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
    <NButton
      size="small"
      :disabled="disabled"
      data-test="use-auto"
      @click="useAuto"
      >{{ t('recUseAuto') }}</NButton
    >
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

    &__detected {
      font-weight: var(--tt-weight-medium);
    }

    &__override {
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
