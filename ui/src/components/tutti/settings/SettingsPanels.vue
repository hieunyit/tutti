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
  /** Content of one settings tab, bound to the system settings draft. */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NInput, NInputNumber, NSwitch } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import SettingsRecommendations from './SettingsRecommendations.vue';
  import messages from './i18nSettings';
  import {
    BYTE_KEYS,
    FEATURE_KEYS,
    RESTART_FIELDS,
    WATERMARK_KEYS,
    type SettingsTab,
  } from './settingsMeta';
  import { FormSection } from '@/components/kit';
  import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';
  import {
    WORKER_DURATION_FIELDS,
    WORKER_TUNING_FIELDS,
    WORKER_TUNING_KEYS,
    type WorkerTuningKey,
  } from '@/utils/helpers/SystemSettingsHelper';

  const props = defineProps<{ tab: SettingsTab; disabled: boolean }>();

  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const errors = computed(() => store.validationErrors);
  const draft = computed(() => store.draft);

  function err(key: string): string | undefined {
    return errors.value[key] ? t(errors.value[key]!) : undefined;
  }

  function restart(key: string): string | undefined {
    return RESTART_FIELDS.has(key) ? t('restartRequired') : undefined;
  }

  function inputProps(key: string, hintId = `settings-${key}-hint`) {
    return {
      id: `settings-${key}`,
      'aria-describedby': hintId,
      'aria-invalid': Boolean(errors.value[key]),
    };
  }

  const tuningDefaults: Record<WorkerTuningKey, string> = {
    bulkConcurrencyPercent: '20',
    multipartThreads: '1',
    multipartPartSizeMib: '16',
  };

  /** 0 is stored, but shown as an empty field with the default as placeholder. */
  function tuningValue(key: WorkerTuningKey): number | null {
    return draft.value.worker![key] || null;
  }

  function setTuning(key: WorkerTuningKey, value: number | null) {
    draft.value.worker![key] = value ?? 0;
  }

  const status = (key: string) => (errors.value[key] ? 'error' : undefined);
</script>

<template>
  <div
    v-if="props.tab === 'features'"
    class="settings-panel"
  >
    <FormSection
      :title="t('featuresTitle')"
      :description="t('featuresDesc')"
    >
      <div
        v-for="key in FEATURE_KEYS"
        :key="key"
        class="settings-panel__switch"
      >
        <div class="settings-panel__copy">
          <span
            :id="`settings-${key}-label`"
            class="settings-panel__title"
            >{{ t(key) }}</span
          >
          <span
            :id="`settings-${key}-hint`"
            class="settings-panel__muted"
            >{{ t(`${key}Desc`) }}</span
          >
        </div>
        <NSwitch
          v-model:value="draft.features![key]"
          :disabled="disabled"
          :aria-labelledby="`settings-${key}-label`"
          :aria-describedby="`settings-${key}-hint`"
        />
      </div>
    </FormSection>
  </div>

  <div
    v-else-if="props.tab === 'bulk'"
    class="settings-panel"
  >
    <FormSection
      :title="t('bulkTitle')"
      :description="t('bulkDesc')"
    >
      <SettingsField
        v-for="key in WATERMARK_KEYS"
        :id="key"
        :key="key"
        :label="t(key)"
        :hint="t(`${key}Hint`)"
        :error="err(key)"
      >
        <NInputNumber
          v-model:value="draft.worker![key]"
          :disabled="disabled"
          :min="1"
          :max="2147483647"
          :precision="0"
          :status="status(key)"
          :input-props="inputProps(key)"
        />
      </SettingsField>
      <SettingsField
        v-for="key in BYTE_KEYS"
        :id="key"
        :key="key"
        :label="t(key)"
        :hint="
          t(
            key === 'bandwidthLimitBytes'
              ? 'bandwidthLimitHint'
              : 'bufferBudgetHint',
          )
        "
        :error="err(key)"
      >
        <NInput
          v-model:value="draft.worker![key]"
          :disabled="disabled"
          :status="status(key)"
          :input-props="{ ...inputProps(key), inputmode: 'numeric' }"
        />
      </SettingsField>
    </FormSection>
  </div>

  <div
    v-else-if="props.tab === 'worker'"
    class="settings-panel"
  >
    <FormSection
      :title="t('workerTitle')"
      :description="t('workerDesc')"
      :tag="t('restartRequired')"
    >
      <SettingsField
        id="concurrency"
        :label="t('concurrency')"
        :hint="t('concurrencyHint')"
        :error="err('concurrency')"
        :restart="restart('concurrency')"
      >
        <NInputNumber
          v-model:value="draft.worker!.concurrency"
          :disabled="disabled"
          :min="1"
          :max="1000"
          :precision="0"
          :status="status('concurrency')"
          :input-props="inputProps('concurrency')"
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
    </FormSection>
    <FormSection
      :title="t('tuneTitle')"
      :description="t('tuneDesc')"
    >
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
        :live="key === 'bulkConcurrencyPercent' ? undefined : t('appliesLive')"
      >
        <NInputNumber
          :value="tuningValue(key)"
          :disabled="disabled"
          :min="0"
          :max="WORKER_TUNING_FIELDS[key].max"
          :precision="0"
          :placeholder="tuningDefaults[key]"
          :status="status(key)"
          :input-props="inputProps(key)"
          @update:value="(v) => setTuning(key, v)"
        />
      </SettingsField>
      <RouterLink
        class="settings-panel__link"
        to="/dashboard?tab=metrics"
        data-test="throughput-link"
        >{{ t('throughputLink') }}</RouterLink
      >
      <SettingsRecommendations :disabled="disabled" />
    </FormSection>
  </div>

  <div
    v-else-if="props.tab === 'locking'"
    class="settings-panel"
  >
    <FormSection
      :title="t('lockTitle')"
      :description="t('lockDesc')"
      :tag="t('restartRequired')"
    >
      <SettingsField
        id="lockOverlap"
        :label="t('lockOverlap')"
        :hint="t('lockOverlapHint')"
        :error="err('lockOverlap')"
        :restart="restart('lockOverlap')"
      >
        <NInput
          v-model:value="draft.lock!.overlap"
          :disabled="disabled"
          placeholder="2s"
          :status="status('lockOverlap')"
          :input-props="inputProps('lockOverlap')"
        />
      </SettingsField>
    </FormSection>
  </div>

  <div
    v-else
    class="settings-panel"
  >
    <FormSection
      :title="t('webhookTitle')"
      :description="t('webhookDesc')"
    >
      <div class="settings-panel__switch">
        <span
          id="settings-webhook-label"
          class="settings-panel__title"
          >{{ t('webhookEnabled') }}</span
        >
        <NSwitch
          v-model:value="draft.webhook!.enabled"
          :disabled="disabled"
          aria-labelledby="settings-webhook-label"
        />
      </div>
      <SettingsField
        id="webhookBaseUrl"
        :label="t('webhookBaseUrl')"
        :hint="t('webhookBaseUrlHint')"
        :error="err('webhookBaseUrl')"
      >
        <NInput
          v-model:value="draft.webhook!.baseUrl"
          :disabled="disabled"
          placeholder="https://tutti.example.com"
          :status="status('webhookBaseUrl')"
          :input-props="{ ...inputProps('webhookBaseUrl'), type: 'url' }"
        />
      </SettingsField>
    </FormSection>
  </div>
</template>

<style lang="scss" scoped>
  .settings-panel {
    max-width: var(--tt-measure);

    &__switch {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-6);
      padding: var(--tt-space-3) 0;
      border-bottom: 1px solid var(--tt-border-subtle);

      &:last-child {
        border-bottom: 0;
      }
    }

    &__copy {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      min-width: 0;
    }

    &__title {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text);
    }

    &__muted {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }
  }
</style>
