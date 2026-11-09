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
  import { NButton, NTag } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref } from 'vue';
  import { useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
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

  async function handleLogout() {
    isLoggingOut.value = true;

    try {
      await auth.logout();
      await router.push({ name: RouteName.LOGIN });
    } finally {
      isLoggingOut.value = false;
    }
  }
</script>

<template>
  <div class="auth-user-menu">
    <span class="auth-user-menu__name">{{ username }}</span>
    <NTag
      size="small"
      round
      :type="role === AuthRole.ADMIN ? 'success' : 'warning'"
    >
      {{ roleLabel }}
    </NTag>
    <NButton
      size="small"
      secondary
      :loading="isLoggingOut"
      @click="handleLogout"
    >
      {{ t('logout') }}
    </NButton>
  </div>
</template>

<style lang="scss" scoped>
  .auth-user-menu {
    display: flex;
    align-items: center;
    gap: 8px;

    &__name {
      font-weight: 500;
    }
  }
</style>
