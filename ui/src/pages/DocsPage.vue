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
  import { useRoute } from 'vue-router';
  import DocsGuides from '@/components/tutti/docs/DocsGuides.vue';
  import i18nDocs from '@/components/tutti/docs/i18nDocs';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import { RouteName } from '@/utils/types/router';

  // Swagger UI and the spec are only fetched on the API reference route
  const DocsApiReference = defineAsyncComponent(
    () => import('@/components/tutti/docs/DocsApiReference.vue'),
  );

  const { t } = useI18n({ messages: i18nDocs });
  const route = useRoute();
  const isApi = computed(() => route.name === RouteName.TUTTI_API_REFERENCE);
  const slug = computed(() =>
    typeof route.params.slug === 'string' ? route.params.slug : undefined,
  );
</script>

<template>
  <div class="docs-page">
    <PageHeader
      :title="t('title')"
      :description="isApi ? t('apiDescription') : t('description')"
    >
      <template #actions>
        <nav
          class="docs-page__tabs"
          :aria-label="t('sections')"
        >
          <RouterLink
            class="docs-page__tab"
            :to="{ name: RouteName.TUTTI_DOCS }"
            :aria-current="isApi ? undefined : 'page'"
            :class="{ 'docs-page__tab--active': !isApi }"
          >
            {{ t('tabGuides') }}
          </RouterLink>
          <RouterLink
            class="docs-page__tab"
            :to="{ name: RouteName.TUTTI_API_REFERENCE }"
            :aria-current="isApi ? 'page' : undefined"
            :class="{ 'docs-page__tab--active': isApi }"
          >
            {{ t('tabApi') }}
          </RouterLink>
        </nav>
      </template>
    </PageHeader>

    <DocsApiReference v-if="isApi" />
    <DocsGuides
      v-else
      :slug="slug"
    />
  </div>
</template>

<style lang="scss" scoped>
  .docs-page {
    max-width: 1600px;

    &__tabs {
      display: flex;
      gap: 4px;
      padding: 3px;
      background-color: var(--tt-disabled-bg);
      border-radius: 10px;
    }

    &__tab {
      display: inline-flex;
      align-items: center;
      height: 32px;
      padding: 0 14px;
      font-size: 13px;
      color: var(--tt-text-2);
      text-decoration: none;
      border-radius: var(--tt-radius-control);

      &--active {
        font-weight: 600;
        color: var(--tt-text);
        background-color: var(--tt-surface);
        box-shadow: var(--tt-shadow);
      }
    }
  }
</style>
