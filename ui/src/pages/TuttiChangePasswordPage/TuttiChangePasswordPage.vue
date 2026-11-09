<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import { NButton } from 'naive-ui';
  import ChangePasswordForm from '@/components/auth/ChangePasswordForm.vue';
  import messages from '@/components/tutti/users/i18nManagement';
  import { useAuthStore } from '@/stores/authStore';
  import { RouteName } from '@/utils/types/router';
  import { AuthHelper } from '@/utils/helpers/AuthHelper';
  const { t } = useI18n({ messages });
  const router = useRouter();
  const route = useRoute();
  const auth = useAuthStore();

  async function changed() {
    const redirect = AuthHelper.getSafeRedirect(route.query.redirect);

    await router.replace(
      typeof redirect === 'string' && !redirect.startsWith('/change-password')
        ? redirect
        : { name: RouteName.CHORUS_HOME },
    );
  }

  async function logout() {
    await auth.logout();
    await router.replace({ name: RouteName.LOGIN });
  }
</script>

<template>
  <main class="password-page">
    <section class="phase-d password-page__card">
      <h1>{{ t('changePassword') }}</h1>
      <ChangePasswordForm
        forced
        @changed="changed"
      /><NButton
        text
        @click="logout"
        >{{ t('logout') }}</NButton
      >
    </section>
  </main>
</template>

<style lang="scss" src="@/components/tutti/users/phaseD.scss"></style>

<style scoped>
  .password-page {
    min-height: 100dvh;
    display: grid;
    place-items: center;
    padding: 24px;
    background: var(--tt-bg-app);
  }

  .password-page__card {
    width: 100%;
    max-width: 480px;
    padding: 24px;
    border: 1px solid var(--tt-border);
    border-radius: 12px;
    background: var(--tt-surface);
  }

  h1 {
    font-size: 24px;
    margin: 0;
  }
</style>
