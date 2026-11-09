<!--
  - Copyright © 2026 hieunyit
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
  import { computed } from 'vue';
  import { useRoute } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import { RouteName } from '@/utils/types/router';

  // Placeholder: the documentation and API reference content is added later.
  const { t } = useI18n({
    messages: {
      en: {
        docsTitle: 'Documentation',
        docsDescription: 'Guides for configuring and operating Tutti.',
        apiTitle: 'API reference',
        apiDescription: 'Endpoints of the Tutti management API.',
        soon: 'This page is being prepared.',
      },
      de: {
        docsTitle: 'Dokumentation',
        docsDescription:
          'Anleitungen zur Konfiguration und zum Betrieb von Tutti.',
        apiTitle: 'API-Referenz',
        apiDescription: 'Endpunkte der Tutti-Management-API.',
        soon: 'Diese Seite wird gerade vorbereitet.',
      },
    },
  });
  const route = useRoute();
  const isApi = computed(() => route.name === RouteName.TUTTI_API_REFERENCE);
</script>

<template>
  <div class="docs-page">
    <PageHeader
      :title="isApi ? t('apiTitle') : t('docsTitle')"
      :description="isApi ? t('apiDescription') : t('docsDescription')"
    />
    <SectionCard :title="isApi ? t('apiTitle') : t('docsTitle')">
      <p class="docs-page__soon">{{ t('soon') }}</p>
    </SectionCard>
  </div>
</template>

<style lang="scss" scoped>
  .docs-page__soon {
    margin: 0;
    font-size: 14px;
    color: var(--tt-text-2);
  }
</style>
