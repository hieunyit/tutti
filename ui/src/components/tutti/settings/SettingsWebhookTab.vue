<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NInput } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import SettingsSection from './SettingsSection.vue';
  import SettingsTabLayout from './SettingsTabLayout.vue';
  import SettingsSwitchRow from './SettingsSwitchRow.vue';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft, err, inputProps, status } = useSettingsForm();
</script>

<template>
  <SettingsTabLayout>
    <SettingsSection
      :title="t('sectionWebhook')"
      :description="t('webhookLead')"
      list
    >
      <SettingsSwitchRow
        id="webhook"
        v-model:value="draft.webhook!.enabled"
        :label="t('webhookEnabled')"
        :disabled="disabled"
      />
    </SettingsSection>
    <SettingsSection :title="t('sectionAddress')">
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
    </SettingsSection>
  </SettingsTabLayout>
</template>
