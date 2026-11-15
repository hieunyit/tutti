<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Worker concurrency, intervals, throughput tuning (Auto badges) and recommendations. */
  import { NInput, NInputNumber } from 'naive-ui';
  import { computed } from 'vue';
  import SettingsField from './SettingsField.vue';
  import SettingsSection from './SettingsSection.vue';
  import SettingsTabLayout from './SettingsTabLayout.vue';
  import SettingsRecommendations from './SettingsRecommendations.vue';
  import { useSettingsForm } from './useSettingsForm';
  import { RouteName } from '@/utils/types/router';
  import {
    WORKER_DURATION_FIELDS,
    WORKER_TUNING_FIELDS,
    type WorkerTuningKey,
  } from '@/utils/helpers/SystemSettingsHelper';

  defineProps<{ disabled: boolean }>();

  const { t, store, draft, err, restart, inputProps, status, autoLabel } =
    useSettingsForm();

  const tuningDefaults: Record<WorkerTuningKey, string> = {
    bulkConcurrencyPercent: '20',
    multipartThreads: '1',
    multipartPartSizeMib: '16',
    largeObjectStreams: '4',
  };

  /** Placeholder of an unset tuning field: the value the worker derived for itself. */
  function tuningPlaceholder(key: WorkerTuningKey): string {
    return String(store.settings?.effective?.[key] ?? tuningDefaults[key]);
  }

  /** 0 is stored, but shown as an empty field with the default as placeholder. */
  function tuningValue(key: WorkerTuningKey): number | null {
    return draft.value.worker![key] || null;
  }

  function setTuning(key: WorkerTuningKey, value: number | null) {
    draft.value.worker![key] = value ?? 0;
  }

  /** The buffer budget is stored as 0 when auto: show it empty with the derived value as placeholder. */
  function bufferValue(): string {
    const v = draft.value.worker!.bufferBudgetBytes;

    return v === '0' || v === '' ? '' : (v ?? '');
  }

  const timingHasError = computed(() =>
    WORKER_DURATION_FIELDS.some((key) => err(key)),
  );

  function setBuffer(value: string) {
    draft.value.worker!.bufferBudgetBytes = value.trim() === '' ? '0' : value;
  }
</script>

<template>
  <SettingsTabLayout>
    <SettingsSection
      :title="t('sectionConcurrency')"
      :description="t('tuneDesc')"
    >
      <SettingsField
        id="concurrency"
        :label="t('concurrency')"
        :hint="t('concurrencyHint')"
        :error="err('concurrency')"
        :restart="restart('concurrency')"
        :auto="autoLabel('concurrency')"
        :auto-title="t('autoBadgeTitle')"
      >
        <NInputNumber
          :value="draft.worker!.concurrency || null"
          :disabled="disabled"
          :min="0"
          :max="1000"
          :precision="0"
          :placeholder="String(store.settings?.effective?.concurrency ?? 10)"
          :status="status('concurrency')"
          :input-props="inputProps('concurrency')"
          @update:value="(v) => (draft.worker!.concurrency = v ?? 0)"
        />
      </SettingsField>
      <SettingsField
        id="bulkConcurrencyPercent"
        :label="t('bulkConcurrencyPercent')"
        :hint="t('bulkConcurrencyPercentHint')"
        :error="err('bulkConcurrencyPercent')"
        :restart="t('restartRequired')"
        :auto="autoLabel('bulkConcurrencyPercent')"
        :auto-title="t('autoBadgeTitle')"
      >
        <NInputNumber
          :value="tuningValue('bulkConcurrencyPercent')"
          :disabled="disabled"
          :min="0"
          :max="WORKER_TUNING_FIELDS.bulkConcurrencyPercent.max"
          :precision="0"
          :placeholder="tuningPlaceholder('bulkConcurrencyPercent')"
          :status="status('bulkConcurrencyPercent')"
          :input-props="inputProps('bulkConcurrencyPercent')"
          @update:value="(v) => setTuning('bulkConcurrencyPercent', v)"
        />
      </SettingsField>
      <SettingsField
        id="largeObjectStreams"
        :label="t('largeObjectStreams')"
        :hint="t('largeObjectStreamsHint')"
        :error="err('largeObjectStreams')"
        :live="t('appliesLive')"
        :auto="autoLabel('largeObjectStreams')"
        :auto-title="t('autoBadgeTitle')"
      >
        <NInputNumber
          :value="tuningValue('largeObjectStreams')"
          :disabled="disabled"
          :min="0"
          :max="WORKER_TUNING_FIELDS.largeObjectStreams.max"
          :precision="0"
          :placeholder="tuningPlaceholder('largeObjectStreams')"
          :status="status('largeObjectStreams')"
          :input-props="inputProps('largeObjectStreams')"
          @update:value="(v) => setTuning('largeObjectStreams', v)"
        />
      </SettingsField>
    </SettingsSection>
    <SettingsSection :title="t('sectionMultipart')">
      <SettingsField
        id="multipartThreads"
        :label="t('multipartThreads')"
        :hint="t('multipartThreadsHint')"
        :error="err('multipartThreads')"
        :live="t('appliesLive')"
        :auto="autoLabel('multipartThreads')"
        :auto-title="t('autoBadgeTitle')"
      >
        <NInputNumber
          :value="tuningValue('multipartThreads')"
          :disabled="disabled"
          :min="0"
          :max="WORKER_TUNING_FIELDS.multipartThreads.max"
          :precision="0"
          :placeholder="tuningPlaceholder('multipartThreads')"
          :status="status('multipartThreads')"
          :input-props="inputProps('multipartThreads')"
          @update:value="(v) => setTuning('multipartThreads', v)"
        />
      </SettingsField>
      <SettingsField
        id="multipartPartSizeMib"
        :label="t('multipartPartSizeMib')"
        :hint="t('multipartPartSizeMibHint')"
        :error="err('multipartPartSizeMib')"
        :live="t('appliesLive')"
        :auto="autoLabel('multipartPartSizeMib')"
        :auto-title="t('autoBadgeTitle')"
      >
        <NInputNumber
          :value="tuningValue('multipartPartSizeMib')"
          :disabled="disabled"
          :min="0"
          :max="WORKER_TUNING_FIELDS.multipartPartSizeMib.max"
          :precision="0"
          :placeholder="tuningPlaceholder('multipartPartSizeMib')"
          :status="status('multipartPartSizeMib')"
          :input-props="inputProps('multipartPartSizeMib')"
          @update:value="(v) => setTuning('multipartPartSizeMib', v)"
        />
      </SettingsField>
      <SettingsField
        id="bufferBudgetBytes"
        :label="t('bufferBudgetBytes')"
        :hint="t('bufferBudgetHint')"
        :error="err('bufferBudgetBytes')"
        :auto="autoLabel('bufferBudgetBytes')"
        :auto-title="t('autoBadgeTitle')"
      >
        <NInput
          :value="bufferValue()"
          :disabled="disabled"
          :placeholder="
            String(store.settings?.effective?.bufferBudgetBytes ?? '')
          "
          :status="status('bufferBudgetBytes')"
          :input-props="{
            ...inputProps('bufferBudgetBytes'),
            inputmode: 'numeric',
          }"
          @update:value="setBuffer"
        />
      </SettingsField>
    </SettingsSection>
    <RouterLink
      class="settings-worker__link"
      :to="{ name: RouteName.TUTTI_DASHBOARD, query: { tab: 'traffic' } }"
      data-test="throughput-link"
      >{{ t('throughputLink') }}</RouterLink
    >
    <SettingsSection
      collapsible
      :force-open="timingHasError"
      :title="t('sectionTiming')"
      :description="t('sectionTimingDesc')"
    >
      <SettingsField
        v-for="key in WORKER_DURATION_FIELDS"
        :id="key"
        :key="key"
        :label="t(key)"
        :hint="t('durationHint')"
        :error="err(key)"
        :restart="restart(key)"
      >
        <NInput
          v-model:value="draft.worker![key]"
          :disabled="disabled"
          placeholder="30s"
          :status="status(key)"
          :input-props="inputProps(key)"
        />
      </SettingsField>
    </SettingsSection>
    <template #aside>
      <SettingsRecommendations :disabled="disabled" />
    </template>
  </SettingsTabLayout>
</template>

<style lang="scss" scoped>
  .settings-worker__link {
    align-self: flex-start;
    margin-top: calc(-1 * var(--tt-space-4));
    font-size: var(--tt-text-sm);
    color: var(--tt-accent);
  }
</style>
