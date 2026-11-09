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
  import { NDropdown, NTag } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import type { DropdownOption } from 'naive-ui';
  import i18nAuth from './i18nAuth';
  import { useAuthStore } from '@/stores/authStore';
  import { AuthRole } from '@/utils/types/auth';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nAuth });
  const router = useRouter();
  const auth = useAuthStore();
  const { username, role } = storeToRefs(auth);
  const isLoggingOut = ref(false);

  const roleLabel = computed(() =>
    role.value === AuthRole.ADMIN ? t('roleAdmin') : t('roleViewer'),
  );

  const options = computed<DropdownOption[]>(() => [
    {
      key: 'info',
      type: 'render',
      render: () =>
        h('div', { class: 'auth-user-menu__info' }, [
          h('div', { class: 'auth-user-menu__info-label' }, t('signedInAs')),
          h(
            'div',
            { class: 'auth-user-menu__info-name' },
            username.value ?? '',
          ),
          h(
            'div',
            { class: 'auth-user-menu__info-role' },
            role.value === AuthRole.ADMIN
              ? t('roleInfoAdmin')
              : t('roleInfoViewer'),
          ),
        ]),
    },
    { key: 'divider', type: 'divider' },
    { key: 'logout', label: t('logout'), disabled: isLoggingOut.value },
  ]);

  async function handleLogout() {
    isLoggingOut.value = true;

    try {
      await auth.logout();
      await router.push({ name: RouteName.LOGIN });
    } finally {
      isLoggingOut.value = false;
    }
  }

  function handleSelect(key: string) {
    if (key === 'logout') {
      handleLogout();
    }
  }
</script>

<template>
  <NDropdown
    trigger="click"
    placement="bottom-end"
    :options="options"
    @select="handleSelect"
  >
    <button
      type="button"
      class="auth-user-menu"
      :aria-label="t('userMenu')"
    >
      <span class="auth-user-menu__name">{{ username }}</span>
      <NTag
        size="small"
        round
        :type="role === AuthRole.ADMIN ? 'success' : 'warning'"
      >
        {{ roleLabel }}
      </NTag>
      <span
        class="auth-user-menu__caret"
        aria-hidden="true"
        >▾</span
      >
    </button>
  </NDropdown>
</template>

<style lang="scss" scoped>
  .auth-user-menu {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 32px;
    padding: 0 8px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    font: inherit;
    color: var(--text-color-1);
    cursor: pointer;

    &:hover {
      background-color: var(--button-color-2-hover);
    }

    &:focus-visible {
      outline: 2px solid var(--primary-color);
    }

    &__name {
      max-width: 140px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 500;

      @media screen and (max-width: 639px) {
        display: none;
      }
    }

    &__caret {
      font-size: 10px;
      color: var(--text-color-3);
    }
  }
</style>

<style lang="scss">
  .auth-user-menu__info {
    padding: 8px 12px;
    min-width: 200px;
    max-width: 280px;
  }

  .auth-user-menu__info-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-color-3);
  }

  .auth-user-menu__info-name {
    font-weight: 600;
    word-break: break-all;
    color: var(--text-color-1);
  }

  .auth-user-menu__info-role {
    margin-top: 4px;
    font-size: 12px;
    color: var(--text-color-3);
  }
</style>
