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
  import { onBeforeUnmount, onMounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { loadSpec } from './docsSources';
  import i18nDocs from './i18nDocs';
  import { EmptyBlock, ErrorBlock, LoadingBlock } from '@/components/kit';
  import { useAuthStore } from '@/stores/authStore';
  import { API_BASE_URL, API_PREFIX } from '@/utils/constants/env';
  import { prefixApiUrl, roleForPath } from '@/utils/helpers/ApiDocsHelper';
  import '@/styles/swagger.scss';

  const { t } = useI18n({ messages: i18nDocs });
  const auth = useAuthStore();

  const root = ref<HTMLElement | null>(null);
  const status = ref<'loading' | 'ready' | 'missing' | 'failed'>('loading');
  let spec: Record<string, unknown> | null = null;

  interface RequestLike {
    url: string;
    headers: Record<string, string>;
  }

  async function init() {
    status.value = 'loading';

    const pending = loadSpec();

    if (!pending) {
      status.value = 'missing';

      return;
    }

    try {
      // Swagger UI is large: it is only loaded on this page
      const [{ default: SwaggerUI }] = await Promise.all([
        import('swagger-ui-dist/swagger-ui-es-bundle.js'),
        import('swagger-ui-dist/swagger-ui.css'),
      ]);

      spec = await pending;
      status.value = 'ready';

      const el = root.value;

      if (!el) return;

      const roleLabel = {
        admin: t('roleAdmin'),
        viewer: t('roleViewer'),
        public: t('rolePublic'),
      };

      SwaggerUI({
        spec,
        domNode: el,
        deepLinking: false,
        docExpansion: 'none',
        defaultModelsExpandDepth: -1,
        displayRequestDuration: true,
        tryItOutEnabled: false,
        presets: [SwaggerUI.presets.apis],
        // requests go to the management API with the current session
        requestInterceptor: (req: RequestLike) => {
          req.url = prefixApiUrl(
            req.url,
            API_BASE_URL,
            API_PREFIX,
            window.location.origin,
          );

          if (auth.token) req.headers.Authorization = `Bearer ${auth.token}`;

          return req;
        },
        plugins: [
          () => ({
            wrapComponents: {
              // marks who may call an operation next to its path
              OperationSummaryPath:
                (
                  Original: unknown,
                  system: {
                    React: {
                      createElement: (...args: unknown[]) => unknown;
                      Fragment: unknown;
                    };
                  },
                ) =>
                (props: { specPath: { toArray: () => string[] } }) => {
                  const [, path = '', method = ''] = props.specPath.toArray();
                  const role = roleForPath(spec ?? {}, method, path);
                  const { React } = system;

                  return React.createElement(
                    React.Fragment,
                    null,
                    React.createElement(Original as never, props as never),
                    React.createElement(
                      'span',
                      { className: `tt-role tt-role--${role}` },
                      roleLabel[role],
                    ),
                  );
                },
            },
          }),
        ],
      });
    } catch {
      status.value = 'failed';
    }
  }

  function retry() {
    void init();
  }

  function downloadSpec() {
    if (!spec) return;

    const blob = new Blob([JSON.stringify(spec, null, 2)], {
      type: 'application/json',
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');

    link.href = url;
    link.download = 'tutti-openapi.json';
    link.click();
    URL.revokeObjectURL(url);
  }

  onMounted(init);
  onBeforeUnmount(() => {
    if (root.value) root.value.innerHTML = '';
  });
</script>

<template>
  <section
    class="docs-api"
    :aria-label="t('apiTitle')"
  >
    <header class="docs-api__header">
      <h2 class="docs-api__title">{{ t('apiTitle') }}</h2>
      <span class="docs-api__meta">
        {{ t('apiSpecMeta', { prefix: API_PREFIX || '/' }) }}
      </span>
      <button
        type="button"
        class="docs-api__download"
        :disabled="status !== 'ready'"
        @click="downloadSpec"
      >
        {{ t('downloadSpec') }}
      </button>
    </header>
    <p class="docs-api__hint">{{ t('apiHint') }}</p>

    <LoadingBlock
      v-if="status === 'loading'"
      class="docs-api__loading"
      :title="t('apiLoading')"
      :rows="8"
    />
    <EmptyBlock
      v-else-if="status === 'missing'"
      :title="t('apiNotBundledTitle')"
      :description="t('apiNotBundledText')"
    />
    <ErrorBlock
      v-else-if="status === 'failed'"
      :title="t('apiFailed')"
      :retry-label="t('retry')"
      @retry="retry"
    />

    <div
      v-show="status === 'ready'"
      ref="root"
      class="tt-swagger"
    />
  </section>
</template>

<style lang="scss" scoped>
  .docs-api {
    min-width: 0;
    overflow: hidden;
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);

    &__header {
      display: flex;
      flex-wrap: wrap;
      gap: 8px 12px;
      align-items: center;
      padding: 14px var(--tt-card-padding);
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    &__title {
      margin: 0;
      font-size: 15px;
      font-weight: 600;
      color: var(--tt-text);
    }

    &__meta {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__download {
      padding: 0;
      margin-left: auto;
      font: inherit;
      font-size: 13px;
      font-weight: 600;
      color: var(--tt-accent);
      cursor: pointer;
      background: none;
      border: 0;

      &:hover:not(:disabled) {
        color: var(--tt-accent-hover);
        text-decoration: underline;
      }

      &:disabled {
        color: var(--tt-disabled-text);
        cursor: default;
      }
    }

    &__hint {
      margin: 0;
      padding: 10px var(--tt-card-padding);
      font-size: 13px;
      color: var(--tt-text-2);
      border-bottom: 1px solid var(--tt-border-subtle);
    }

    &__loading {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: var(--tt-card-padding);
    }
  }
</style>
