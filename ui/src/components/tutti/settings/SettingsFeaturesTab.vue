<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import SettingsSection from './SettingsSection.vue';
  import SettingsSwitchRow from './SettingsSwitchRow.vue';
  import SettingsTabLayout from './SettingsTabLayout.vue';
  import { FEATURE_GROUPS } from './settingsMeta';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft } = useSettingsForm();
</script>

<template>
  <SettingsTabLayout>
    <SettingsSection
      v-for="group in FEATURE_GROUPS"
      :key="group.title"
      :title="t(group.title)"
      :description="t(group.description)"
      list
    >
      <SettingsSwitchRow
        v-for="key in group.keys"
        :id="key"
        :key="key"
        v-model:value="draft.features![key]"
        :label="t(key)"
        :hint="t(`${key}Desc`)"
        :disabled="disabled"
      />
    </SettingsSection>
  </SettingsTabLayout>
</template>
