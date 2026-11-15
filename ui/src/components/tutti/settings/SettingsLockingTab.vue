<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NInput } from 'naive-ui';
  import SettingsField from './SettingsField.vue';
  import SettingsSection from './SettingsSection.vue';
  import SettingsTabLayout from './SettingsTabLayout.vue';
  import { useSettingsForm } from './useSettingsForm';

  defineProps<{ disabled: boolean }>();

  const { t, draft, err, restart, inputProps, status } = useSettingsForm();
</script>

<template>
  <SettingsTabLayout>
    <SettingsSection :title="t('sectionLocks')">
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
    </SettingsSection>
  </SettingsTabLayout>
</template>
