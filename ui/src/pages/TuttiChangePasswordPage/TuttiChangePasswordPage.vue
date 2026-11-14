<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { useI18n } from 'vue-i18n';
  import { useRoute, useRouter } from 'vue-router';
  import { NButton } from 'naive-ui';
  import ChangePasswordForm from '@/components/auth/ChangePasswordForm.vue';
  import messages from '@/components/auth/i18nAuth';
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
        : { name: RouteName.TUTTI_HOME },
    );
  }

  async function logout() {
    await auth.logout();
    await router.replace({ name: RouteName.LOGIN });
  }
</script>

<template>
  <main class="password-page">
    <section class="password-page__card">
      <h1>{{ t('changePassword') }}</h1>
      <ChangePasswordForm
        forced
        @changed="changed"
      />
      <NButton
        text
        @click="logout"
        >{{ t('logout') }}</NButton
      >
    </section>
  </main>
</template>

<style scoped>
  .password-page {
    min-height: 100dvh;
    display: grid;
    place-items: center;
    padding: var(--tt-space-6);
    background: var(--tt-bg);
  }

  .password-page__card {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    box-sizing: border-box;
    width: 100%;
    max-width: 480px;
    padding: var(--tt-space-6);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-modal);
    background: var(--tt-surface);
  }

  h1 {
    font-size: var(--tt-text-xl);
    font-weight: var(--tt-weight-strong);
    margin: 0;
  }
</style>
