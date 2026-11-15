<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import SettingsSwitchRow from './SettingsSwitchRow.vue';
  import { FEATURE_KEYS } from './settingsMeta';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft } = useSettingsForm();
</script>

<template>
  <div class="settings-tab">
    <p class="settings-tab__lead">{{ t('featuresLead') }}</p>
    <SettingsSwitchRow
      v-for="key in FEATURE_KEYS"
      :id="key"
      :key="key"
      v-model:value="draft.features![key]"
      :label="t(key)"
      :hint="t(`${key}Desc`)"
      :disabled="disabled"
    />
  </div>
</template>

<style lang="scss" scoped>
  .settings-tab {
    max-width: var(--tt-measure);

    &__lead {
      margin: 0;
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }
  }
</style>
