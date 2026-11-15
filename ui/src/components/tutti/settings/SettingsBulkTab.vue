<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /** Initial copy limits: queue watermarks and copy bandwidth. */
  import { NInput, NInputNumber } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import SettingsSection from './SettingsSection.vue';
  import SettingsTabLayout from './SettingsTabLayout.vue';
  import { WATERMARK_KEYS } from './settingsMeta';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft, err, inputProps, status } = useSettingsForm();
</script>

<template>
  <SettingsTabLayout>
    <SettingsSection
      :title="t('sectionQueue')"
      :description="t('bulkLead')"
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
    </SettingsSection>
    <SettingsSection :title="t('sectionBandwidth')">
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
    </SettingsSection>
  </SettingsTabLayout>
</template>
