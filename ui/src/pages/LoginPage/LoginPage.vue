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
  import {
    NAlert,
    NButton,
    NCheckbox,
    NForm,
    NFormItem,
    NInput,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import i18nAuth from '@/components/auth/i18nAuth';
  import { useAuthStore } from '@/stores/authStore';
  import { AuthHelper } from '@/utils/helpers/AuthHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nAuth });
  const route = useRoute();
  const router = useRouter();
  const auth = useAuthStore();
  const { isLoggingIn, loginError, retryAfterSeconds } = storeToRefs(auth);

  const username = ref('');
  const password = ref('');
  const keepSignedIn = ref(false);
  const showErrors = ref(false);

  const usernameError = computed(() =>
    showErrors.value && username.value.trim() === ''
      ? t('usernameRequired')
      : undefined,
  );
  const passwordError = computed(() =>
    showErrors.value && password.value === ''
      ? t('passwordRequired')
      : undefined,
  );

  const errorMessage = computed(() => {
    if (loginError.value === 'invalidCredentials') {
      return t('errorInvalidCredentials');
    }

    if (loginError.value === 'rateLimited') {
      return retryAfterSeconds.value === null
        ? t('errorRateLimitedUnknown')
        : t('errorRateLimited', {
            time: AuthHelper.formatRetryAfter(retryAfterSeconds.value, {
              seconds: (n) => t('secondsShort', { n }),
              minutes: (n) => t('minutesShort', { n }),
            }),
          });
    }

    return loginError.value ? t('errorUnknown') : null;
  });

  async function submit() {
    showErrors.value = true;

    if (username.value.trim() === '' || password.value === '') return;

    if (
      await auth.login(
        username.value.trim(),
        password.value,
        keepSignedIn.value,
      )
    ) {
      await router.replace(
        AuthHelper.getSafeRedirect(route.query.redirect) ?? {
          name: RouteName.CHORUS_HOME,
        },
      );
    }
  }
</script>

<template>
  <main class="login-page">
    <form
      class="login-page__card"
      @submit.prevent="submit"
    >
      <h1 class="login-page__brand">Tutti</h1>
      <h2 class="login-page__title">{{ t('loginTitle') }}</h2>
      <p class="login-page__subtitle">{{ t('loginSubtitle') }}</p>

      <NAlert
        v-if="errorMessage"
        type="error"
        class="login-page__error"
      >
        {{ errorMessage }}
      </NAlert>

      <NForm
        label-placement="top"
        @submit.prevent="submit"
      >
        <NFormItem
          :label="t('username')"
          :validation-status="usernameError ? 'error' : undefined"
          :feedback="usernameError"
        >
          <NInput
            v-model:value="username"
            autofocus
            :input-props="{ autocomplete: 'username' }"
            :disabled="isLoggingIn"
          />
        </NFormItem>
        <NFormItem
          :label="t('password')"
          :validation-status="passwordError ? 'error' : undefined"
          :feedback="passwordError"
        >
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            :input-props="{ autocomplete: 'current-password' }"
            :disabled="isLoggingIn"
          />
        </NFormItem>
        <NCheckbox
          v-model:checked="keepSignedIn"
          :disabled="isLoggingIn"
        >
          {{ t('keepSignedIn') }}
        </NCheckbox>
        <p class="login-page__hint">{{ t('keepSignedInHint') }}</p>

        <NButton
          type="primary"
          attr-type="submit"
          block
          :loading="isLoggingIn"
        >
          {{ t('signIn') }}
        </NButton>
      </NForm>
    </form>
  </main>
</template>

<style lang="scss" scoped>
  .login-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;

    &__card {
      width: 100%;
      max-width: 400px;
    }

    &__brand {
      margin: 0 0 24px;
      font-size: 40px;
      line-height: 1;
    }

    &__title {
      margin: 0 0 4px;
    }

    &__subtitle {
      margin: 0 0 16px;
      opacity: 0.7;
    }

    &__error {
      margin-bottom: 16px;
    }

    &__hint {
      margin: 4px 0 16px 24px;
      font-size: 12px;
      opacity: 0.7;
    }
  }
</style>
