<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Worker concurrency, intervals, throughput tuning (Auto badges) and recommendations. */
  import { NInput, NInputNumber } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import SettingsRecommendations from './SettingsRecommendations.vue';
  import { useSettingsForm } from './useSettingsForm';
  import { SectionCard } from '@/components/kit';
  import {
    WORKER_DURATION_FIELDS,
    WORKER_TUNING_FIELDS,
    WORKER_TUNING_KEYS,
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

  function setBuffer(value: string) {
    draft.value.worker!.bufferBudgetBytes = value.trim() === '' ? '0' : value;
  }
</script>

<template>
  <div class="settings-tab">
    <div class="settings-tab__group">
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
    </div>
    <SectionCard
      :title="t('tuneTitle')"
      variant="section"
    >
      <div class="settings-tab__group">
        <p class="settings-tab__lead">{{ t('tuneDesc') }}</p>
        <SettingsField
          v-for="key in WORKER_TUNING_KEYS"
          :id="key"
          :key="key"
          :label="t(key)"
          :hint="t(`${key}Hint`)"
          :error="err(key)"
          :restart="
            key === 'bulkConcurrencyPercent' ? t('needsRestart') : undefined
          "
          :live="
            key === 'bulkConcurrencyPercent' ? undefined : t('appliesLive')
          "
          :auto="autoLabel(key)"
          :auto-title="t('autoBadgeTitle')"
        >
          <NInputNumber
            :value="tuningValue(key)"
            :disabled="disabled"
            :min="0"
            :max="WORKER_TUNING_FIELDS[key].max"
            :precision="0"
            :placeholder="tuningPlaceholder(key)"
            :status="status(key)"
            :input-props="inputProps(key)"
            @update:value="(v) => setTuning(key, v)"
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
        <RouterLink
          class="settings-tab__link"
          to="/dashboard?tab=traffic"
          data-test="throughput-link"
          >{{ t('throughputLink') }}</RouterLink
        >
        <SettingsRecommendations :disabled="disabled" />
      </div>
    </SectionCard>
  </div>
</template>

<style lang="scss" scoped>
  .settings-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-6);
    max-width: var(--tt-measure);

    &__group {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
    }

    &__lead {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__link {
      align-self: flex-start;
      font-size: var(--tt-text-sm);
      color: var(--tt-accent);
    }
  }
</style>
