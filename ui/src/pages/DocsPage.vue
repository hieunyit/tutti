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
  import { computed, defineAsyncComponent } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import { PageShell } from '@/components/kit';
  import DocsGuides from '@/components/tutti/docs/DocsGuides.vue';
  import i18nDocs from '@/components/tutti/docs/i18nDocs';
  import { RouteName } from '@/utils/types/router';

  // Swagger UI and the spec are only fetched on the API reference route
  const DocsApiReference = defineAsyncComponent(
    () => import('@/components/tutti/docs/DocsApiReference.vue'),
  );

  const { t } = useI18n({ messages: i18nDocs });
  const route = useRoute();
  const router = useRouter();
  const isApi = computed(() => route.name === RouteName.TUTTI_API_REFERENCE);
  const activeTab = computed(() => (isApi.value ? 'api' : 'guides'));
  const tabs = computed(() => [
    { key: 'guides', label: t('tabGuides') },
    { key: 'api', label: t('tabApi') },
  ]);
  const slug = computed(() =>
    typeof route.params.slug === 'string' && route.params.slug.trim() !== ''
      ? route.params.slug
      : undefined,
  );

  function selectTab(key: string) {
    router.push({
      name:
        key === 'api' ? RouteName.TUTTI_API_REFERENCE : RouteName.TUTTI_DOCS,
    });
  }
</script>

<template>
  <PageShell
    class="docs-page"
    :title="t('title')"
    :description="isApi ? t('apiDescription') : t('description')"
    state="ready"
    :tabs="tabs"
    :active-tab="activeTab"
    @update:active-tab="selectTab"
  >
    <DocsApiReference v-if="isApi" />
    <DocsGuides
      v-else
      :slug="slug"
    />
  </PageShell>
</template>
