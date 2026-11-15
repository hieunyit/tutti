<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Initial copy limits: queue watermarks and copy bandwidth. */
  import { NInput, NInputNumber } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import { WATERMARK_KEYS } from './settingsMeta';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft, err, inputProps, status } = useSettingsForm();
</script>

<template>
  <div class="settings-tab">
    <p class="settings-tab__lead">{{ t('bulkLead') }}</p>
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
      id="bandwidthLimitBytes"
      :label="t('bandwidthLimitBytes')"
      :hint="t('bandwidthLimitHint')"
      :error="err('bandwidthLimitBytes')"
    >
      <NInput
        v-model:value="draft.worker!.bandwidthLimitBytes"
        :disabled="disabled"
        :status="status('bandwidthLimitBytes')"
        :input-props="{
          ...inputProps('bandwidthLimitBytes'),
          inputmode: 'numeric',
        }"
      />
    </SettingsField>
  </div>
</template>

<style lang="scss" scoped>
  .settings-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    max-width: var(--tt-measure);

    &__lead {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }
  }
</style>
