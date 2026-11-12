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
  import { NButton, NDropdown, NModal } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import type { DropdownOption } from 'naive-ui';
  import i18nAuth from './i18nAuth';
  import ChangePasswordForm from './ChangePasswordForm.vue';
  import { useAuthStore } from '@/stores/authStore';
  import { AuthRole } from '@/utils/types/auth';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nAuth });
  const router = useRouter();
  const auth = useAuthStore();
  const { username, role } = storeToRefs(auth);
  const isLoggingOut = ref(false);
  const showPassword = ref(false);
  const showConfigNotice = ref(false);

  const roleLabel = computed(() =>
    role.value === AuthRole.ADMIN ? t('roleNameAdmin') : t('roleNameViewer'),
  );
  const initial = computed(() =>
    (username.value ?? '?').trim().charAt(0).toUpperCase(),
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
    {
      key: 'password',
      label: t('changePassword'),
    },
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
    if (key === 'password') {
      if (auth.source === 'config') {
        showConfigNotice.value = true;
      } else {
        showPassword.value = true;
      }
    }

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
      <span
        class="auth-user-menu__avatar"
        aria-hidden="true"
        >{{ initial }}</span
      >
      <span class="auth-user-menu__name">{{ username }}</span>
      <span class="auth-user-menu__role">{{ roleLabel }}</span>
    </button>
  </NDropdown>
  <NModal
    v-model:show="showPassword"
    preset="card"
    :title="t('changePassword')"
    style="width: 480px; max-width: 94vw"
  >
    <ChangePasswordForm
      v-if="showPassword"
      @changed="showPassword = false"
    />
  </NModal>
  <NModal
    v-model:show="showConfigNotice"
    preset="card"
    :title="t('changePassword')"
    style="width: 480px; max-width: 94vw"
  >
    <div style="display: flex; flex-direction: column; gap: 14px">
      <p style="margin: 0; line-height: 1.5">
        Account <strong>{{ username }}</strong> is defined in the static server configuration file (<code>worker config.yaml</code>).
      </p>
      <p style="margin: 0; font-size: 13px; color: var(--tt-text-3)">
        Passwords for static configuration accounts cannot be modified via the web UI. To change this password, update the bcrypt password hash directly in the server configuration file and restart the worker.
      </p>
      <div style="display: flex; justify-content: flex-end; margin-top: 6px">
        <NButton
          type="primary"
          @click="showConfigNotice = false"
        >
          {{ t('close') || 'Close' }}
        </NButton>
      </div>
    </div>
  </NModal>
</template>

<style lang="scss" scoped>
  .auth-user-menu {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 36px;
    padding: 0 10px;
    border: 1px solid var(--tt-border-strong);
    border-radius: var(--tt-radius-control);
    background-color: var(--tt-surface);
    font: inherit;
    font-size: 13px;
    color: var(--tt-text);
    cursor: pointer;

    &:hover {
      background-color: var(--tt-subtle);
    }

    &__avatar {
      display: flex;
      align-items: center;
      justify-content: center;
      flex: none;
      width: 24px;
      height: 24px;
      border-radius: 50%;
      background-color: var(--tt-avatar-bg);
      color: var(--tt-accent);
      font-size: 12px;
      font-weight: 700;
    }

    &__name {
      max-width: 140px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 600;

      @media screen and (max-width: 639px) {
        display: none;
      }
    }

    &__role {
      color: var(--tt-text-3);

      @media screen and (max-width: 959px) {
        display: none;
      }
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
    color: var(--tt-text-3);
  }

  .auth-user-menu__info-name {
    font-weight: 600;
    word-break: break-all;
    color: var(--tt-text);
  }

  .auth-user-menu__info-role {
    margin-top: 4px;
    font-size: 12px;
    color: var(--tt-text-3);
  }
</style>
