<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NInput } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import SettingsSwitchRow from './SettingsSwitchRow.vue';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft, err, inputProps, status } = useSettingsForm();
</script>

<template>
  <div class="settings-tab">
    <p class="settings-tab__lead">{{ t('webhookLead') }}</p>
    <SettingsSwitchRow
      id="webhook"
      v-model:value="draft.webhook!.enabled"
      :label="t('webhookEnabled')"
      :disabled="disabled"
    />
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
