<!--
  - Copyright © 2026 Clyso GmbH
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
  import { RouterView } from 'vue-router';
  import {
    CConfigProvider,
    CDialogProvider,
    CNotificationProvider,
  } from '@clyso/clyso-ui-kit';
  import { computed, toRefs } from 'vue';
  import { NConfigProvider, dateViVN, viVN } from 'naive-ui';
  import { toKitLocale } from '@/i18n/locales';
  import { useI18nStore } from '@/stores/i18nStore';
  import { useColorSchemeStore } from '@/stores/colorSchemeStore';
  import { DARK_THEME_OVERRIDES, LIGHT_THEME_OVERRIDES } from '@/styles/theme';

  const { locale } = toRefs(useI18nStore());
  const { colorScheme } = toRefs(useColorSchemeStore());
  // ui-kit only knows en/de; Naive UI gets the vi locale from the inner provider.
  const kitLocale = computed(() => toKitLocale(locale.value));
  const isVi = computed(() => locale.value === 'vi');
</script>

<template>
  <CConfigProvider
    class="app"
    :color-scheme="colorScheme"
    :locale="kitLocale"
    :custom-light-theme="LIGHT_THEME_OVERRIDES"
    :custom-dark-theme="DARK_THEME_OVERRIDES"
  >
    <NConfigProvider
      :locale="isVi ? viVN : undefined"
      :date-locale="isVi ? dateViVN : undefined"
      abstract
    >
      <RouterView />

      <CDialogProvider />
      <CNotificationProvider />
    </NConfigProvider>
  </CConfigProvider>
</template>

<style lang="scss" scoped>
  .app {
    min-height: 100vh;
  }
</style>
