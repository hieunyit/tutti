<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProxies from './i18nProxies';
  import ProxyConfigTab from './ProxyConfigTab.vue';
  import ProxyFleetTab from './ProxyFleetTab.vue';
  import { PageShell, useQueryTab } from '@/components/kit';

  const { t } = useI18n({ messages: i18nProxies });
  const tab = useQueryTab('tab', 'fleet');
  const tabs = computed(() => [
    { key: 'fleet', label: t('tabFleet') },
    { key: 'config', label: t('tabConfig') },
  ]);
  // tab content handles its own loading, empty and error states
  const activeTab = computed(() =>
    tab.value === 'config' ? 'config' : 'fleet',
  );
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('description')"
    state="ready"
    :tabs="tabs"
    :active-tab="activeTab"
    @update:active-tab="tab = $event"
  >
    <ProxyConfigTab v-if="activeTab === 'config'" />
    <ProxyFleetTab v-else />
  </PageShell>
</template>
