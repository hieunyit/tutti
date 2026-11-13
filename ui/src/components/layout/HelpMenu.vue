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
  /** Topbar Help menu: guides (/docs) and the API reference (/docs/api). */
  import { computed } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NDropdown, type DropdownOption } from 'naive-ui';
  import i18nLayout from './i18nLayout';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nLayout });
  const router = useRouter();

  const options = computed<DropdownOption[]>(() => [
    { key: RouteName.TUTTI_DOCS, label: t('helpDocs') },
    { key: RouteName.TUTTI_API_REFERENCE, label: t('helpApi') },
  ]);

  function select(key: string | number) {
    router.push({ name: key as RouteName });
  }
</script>

<template>
  <NDropdown
    trigger="click"
    :options="options"
    @select="select"
  >
    <button
      type="button"
      class="help-menu"
      :aria-label="t('help')"
      :title="t('help')"
      aria-haspopup="menu"
    >
      <svg
        aria-hidden="true"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <circle
          cx="12"
          cy="12"
          r="9"
        />
        <path
          d="M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6V14M12 17h.01"
        />
      </svg>
    </button>
  </NDropdown>
</template>

<style lang="scss" scoped>
  .help-menu {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    width: 36px;
    height: 36px;
    border: 1px solid var(--tt-border-strong);
    border-radius: var(--tt-radius-control);
    background-color: var(--tt-surface);
    color: var(--tt-text-2);
    cursor: pointer;

    &:hover {
      background-color: var(--tt-subtle);
      color: var(--tt-text);
    }
  }
</style>
