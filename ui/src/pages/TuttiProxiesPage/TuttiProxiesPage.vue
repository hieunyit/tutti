<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /**
   * Proxies: fleet status only. The configuration (the old Config tab) lives in
   * System settings > Proxies; the old /proxies?tab=config deep link is
   * redirected there by the router.
   */
  import { useI18n } from 'vue-i18n';
  import i18nProxies from './i18nProxies';
  import ProxyFleetTab from './ProxyFleetTab.vue';
  import { PageShell } from '@/components/kit';

  const { t } = useI18n({ messages: i18nProxies });
</script>

<template>
  <PageShell
    :title="t('title')"
    state="ready"
  >
    <ProxyFleetTab />
  </PageShell>
</template>
