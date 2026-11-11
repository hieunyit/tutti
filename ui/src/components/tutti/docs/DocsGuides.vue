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
  import { CSkeleton } from '@clyso/clyso-ui-kit';
  import { computed, nextTick, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import { loadGuide } from './docsSources';
  import i18nDocs from './i18nDocs';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import {
    DOCS,
    GITHUB_BLOB_URL,
    findDoc,
    renderDoc,
    searchDocs,
    sectionHeadings,
    type DocHeading,
    type RenderedDoc,
  } from '@/utils/helpers/DocsHelper';
  import { RouteName } from '@/utils/types/router';

  const props = defineProps<{ slug?: string }>();

  const { t } = useI18n({ messages: i18nDocs });
  const route = useRoute();
  const router = useRouter();

  const current = computed(() => findDoc(props.slug || DOCS[0]?.slug));
  const rendered = ref<RenderedDoc | null>(null);
  const status = ref<'loading' | 'ready' | 'missing' | 'failed'>('loading');
  let loadToken = 0;

  const query = ref('');
  const searching = ref(false);
  const index = ref<Record<string, DocHeading[]>>({});

  const titles = computed(() =>
    Object.fromEntries(DOCS.map((d) => [d.slug, t(`guide_${d.slug}`)])),
  );
  const results = computed(() =>
    searchDocs(query.value, titles.value, index.value),
  );

  async function load() {
    const doc = current.value;
    const token = (loadToken += 1);

    rendered.value = null;

    if (!doc) {
      status.value = 'missing';

      return;
    }

    const source = loadGuide(doc.path);

    if (!source) {
      status.value = 'missing';

      return;
    }

    status.value = 'loading';

    try {
      const text = await source;

      if (token !== loadToken) return;

      rendered.value = renderDoc(text, doc.path);
      index.value = {
        ...index.value,
        [doc.slug]: sectionHeadings(rendered.value),
      };
      status.value = 'ready';
      await nextTick();
      scrollToHash();
    } catch {
      if (token === loadToken) status.value = 'failed';
    }
  }

  function scrollToHash() {
    const id = decodeURIComponent(route.hash.replace(/^#/, ''));
    const target = id ? document.getElementById(id) : null;

    if (target) target.scrollIntoView({ block: 'start' });
    else window.scrollTo({ top: 0 });
  }

  watch(() => props.slug, load, { immediate: true });

  // headings of every guide are only needed for searching: load on demand
  watch(query, async (value) => {
    if (!value.trim() || searching.value) return;

    searching.value = true;

    await Promise.all(
      DOCS.map(async (doc) => {
        if (index.value[doc.slug]) return;

        const source = loadGuide(doc.path);

        if (!source) return;

        try {
          const text = await source;

          index.value = {
            ...index.value,
            [doc.slug]: sectionHeadings(renderDoc(text, doc.path)),
          };
        } catch {
          // a guide that cannot load is simply not searchable
        }
      }),
    );
    searching.value = false;
  });

  const githubUrl = computed(() =>
    current.value ? `${GITHUB_BLOB_URL}${current.value.path}` : '',
  );

  function goTo(slug: string, hash = '') {
    router.push({
      name: RouteName.TUTTI_DOCS,
      params: { slug },
      hash: hash ? `#${hash}` : '',
    });
  }

  function onArticleClick(event: MouseEvent) {
    const link = (event.target as HTMLElement).closest('a');

    if (
      !link ||
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }

    const { docSlug, docHash } = link.dataset;

    if (docSlug !== undefined) {
      event.preventDefault();
      goTo(docSlug, docHash);
    } else if (docHash !== undefined && current.value) {
      event.preventDefault();
      goTo(current.value.slug, docHash);
    }
  }
</script>

<template>
  <div class="docs-guides">
    <nav
      class="docs-guides__nav"
      :aria-label="t('guidesNav')"
    >
      <label class="docs-guides__search">
        <svg
          aria-hidden="true"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle
            cx="11"
            cy="11"
            r="7"
          />
          <path d="M20 20l-3.5-3.5" />
        </svg>
        <input
          v-model="query"
          type="search"
          :placeholder="t('searchPlaceholder')"
          :aria-label="t('search')"
        />
      </label>

      <ul class="docs-guides__list">
        <li
          v-for="result in results"
          :key="result.slug"
        >
          <RouterLink
            class="docs-guides__link"
            :class="{
              'docs-guides__link--active': result.slug === current?.slug,
            }"
            :aria-current="result.slug === current?.slug ? 'page' : undefined"
            :to="{ name: RouteName.TUTTI_DOCS, params: { slug: result.slug } }"
          >
            {{ titles[result.slug] }}
          </RouterLink>
          <ul
            v-if="result.headings.length > 0"
            class="docs-guides__sub"
          >
            <li
              v-for="heading in result.headings"
              :key="heading.id"
            >
              <RouterLink
                class="docs-guides__sublink"
                :to="{
                  name: RouteName.TUTTI_DOCS,
                  params: { slug: result.slug },
                  hash: `#${heading.id}`,
                }"
              >
                {{ heading.text }}
              </RouterLink>
            </li>
          </ul>
        </li>
      </ul>
      <p
        v-if="query.trim() && results.length === 0"
        class="docs-guides__empty"
        role="status"
      >
        {{ searching ? t('searchLoading') : t('searchEmpty', { query }) }}
      </p>
    </nav>

    <article class="docs-guides__article">
      <template v-if="status === 'ready' && rendered && current">
        <span class="docs-guides__crumb">
          {{ t('breadcrumb') }} / {{ titles[current.slug] }} ·
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            >{{ current.path }}</a
          >
        </span>
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div
          class="docs-prose"
          @click="onArticleClick"
          v-html="rendered.html"
        />
      </template>

      <div
        v-else-if="status === 'loading'"
        class="docs-guides__loading"
        role="status"
        :aria-label="t('loading')"
      >
        <CSkeleton
          :height="28"
          :width="260"
        />
        <CSkeleton :height="14" />
        <CSkeleton :height="14" />
        <CSkeleton
          :height="120"
          :border-radius="10"
        />
      </div>

      <EmptyState
        v-else-if="status === 'failed'"
        :title="t('loadFailed')"
        :text="current ? current.path : ''"
      />

      <EmptyState
        v-else-if="!current"
        :title="t('notFoundTitle')"
        :text="t('notFoundText', { slug: slug ?? '' })"
      />

      <EmptyState
        v-else
        :title="t('notBundledTitle')"
        :text="t('notBundledText')"
      >
        <a
          :href="githubUrl"
          target="_blank"
          rel="noopener noreferrer"
          >{{ t('openOnGithub') }}</a
        >
      </EmptyState>
    </article>
  </div>
</template>

<style lang="scss" scoped>
  .docs-guides {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 20px;

    &__nav {
      position: sticky;
      top: 72px;
      display: flex;
      flex: 1 1 220px;
      flex-direction: column;
      gap: 2px;
      max-width: 280px;
      padding: 12px;
      font-size: 14px;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);

      @media (width <= 900px) {
        position: static;
        flex-basis: 100%;
        max-width: none;
      }
    }

    &__search {
      display: flex;
      align-items: center;
      gap: 8px;
      height: 34px;
      padding: 0 10px;
      margin-bottom: 8px;
      font-size: 13px;
      color: var(--tt-text-3);
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);

      &:focus-within {
        outline: 2px solid var(--tt-accent);
        outline-offset: 1px;
      }

      input {
        width: 100%;
        font: inherit;
        color: var(--tt-text);
        background: transparent;
        border: 0;
        outline: 0;
      }
    }

    &__list,
    &__sub {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 0;
      margin: 0;
      list-style: none;
    }

    &__sub {
      padding: 2px 0 6px 12px;
    }

    &__link {
      display: block;
      padding: 7px 10px;
      color: var(--tt-text-2);
      text-decoration: none;
      border-radius: var(--tt-radius-control);

      &:hover {
        background-color: var(--tt-hover);
      }

      &--active {
        font-weight: 600;
        color: var(--tt-accent);
        background-color: var(--tt-accent-subtle);
      }
    }

    &__sublink {
      display: block;
      padding: 4px 10px;
      font-size: 13px;
      color: var(--tt-text-2);
      text-decoration: none;
      border-radius: var(--tt-radius-control);

      &:hover {
        color: var(--tt-accent);
        background-color: var(--tt-hover);
      }
    }

    &__empty {
      margin: 4px 10px;
      font-size: 13px;
      color: var(--tt-text-3);
    }

    &__article {
      flex: 999 1 520px;
      min-width: 0;
      padding: 28px 32px;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);

      @media (width <= 600px) {
        padding: 20px 16px;
      }
    }

    &__crumb {
      display: block;
      margin-bottom: 4px;
      font-size: 12px;
      color: var(--tt-text-3);
      overflow-wrap: anywhere;
    }

    &__loading {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
  }

  // the markdown is injected with v-html: style it with :deep
  .docs-prose {
    font-size: 15px;
    line-height: 1.65;
    color: var(--tt-text-2);
    overflow-wrap: break-word;

    :deep(h1),
    :deep(h2),
    :deep(h3),
    :deep(h4) {
      position: relative;
      color: var(--tt-text);
      scroll-margin-top: 80px;
    }

    :deep(h1) {
      margin: 6px 0 12px;
      font-size: 24px;
      line-height: 32px;
    }

    :deep(h2) {
      margin: 32px 0 10px;
      font-size: 20px;
      line-height: 28px;
    }

    :deep(h3) {
      margin: 22px 0 8px;
      font-size: 16px;
    }

    :deep(h4) {
      margin: 18px 0 6px;
      font-size: 15px;
    }

    :deep(p),
    :deep(ul),
    :deep(ol),
    :deep(pre),
    :deep(blockquote),
    :deep(.docs-table-wrap) {
      margin: 0 0 12px;
    }

    :deep(ul),
    :deep(ol) {
      padding-left: 22px;
    }

    :deep(ul) {
      list-style: disc;
    }

    :deep(ol) {
      list-style: decimal;
    }

    :deep(li + li) {
      margin-top: 4px;
    }

    :deep(a) {
      color: var(--tt-accent);

      &:hover {
        color: var(--tt-accent-hover);
      }
    }

    :deep(.docs-anchor) {
      margin-left: 8px;
      font-weight: 400;
      color: var(--tt-text-3);
      text-decoration: none;
      opacity: 0;
    }

    :deep(h1:hover .docs-anchor),
    :deep(h2:hover .docs-anchor),
    :deep(h3:hover .docs-anchor),
    :deep(h4:hover .docs-anchor),
    :deep(.docs-anchor:focus-visible) {
      opacity: 1;
    }

    :deep(code) {
      padding: 1px 5px;
      font-family: var(--tt-mono);
      font-size: 0.86em;
      color: var(--tt-text);
      background-color: var(--tt-code-bg);
      border-radius: 4px;
    }

    :deep(pre) {
      padding: 14px 16px;
      overflow-x: auto;
      color: #e2e8f0;
      background-color: var(--tt-nav-bg);
      border-radius: 10px;
    }

    :deep(pre code) {
      padding: 0;
      font-size: 13px;
      line-height: 1.55;
      color: inherit;
      background: none;
    }

    :deep(blockquote) {
      padding: 2px 16px;
      color: var(--tt-text-2);
      border-left: 3px solid var(--tt-border-strong);
    }

    :deep(hr) {
      margin: 24px 0;
      border: 0;
      border-top: 1px solid var(--tt-border);
    }

    :deep(.docs-table-wrap) {
      overflow-x: auto;
      border: 1px solid var(--tt-border);
      border-radius: 10px;
    }

    :deep(table) {
      width: 100%;
      font-size: 13px;
      border-collapse: collapse;
    }

    :deep(th) {
      padding: 10px 14px;
      font-size: 12px;
      font-weight: 600;
      color: var(--tt-text-2);
      text-align: left;
      white-space: nowrap;
      background-color: var(--tt-subtle);
      border-bottom: 1px solid var(--tt-border);
    }

    :deep(td) {
      padding: 10px 14px;
      color: var(--tt-text);
      vertical-align: top;
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    :deep(tr:last-child td) {
      border-bottom: 0;
    }
  }
</style>
