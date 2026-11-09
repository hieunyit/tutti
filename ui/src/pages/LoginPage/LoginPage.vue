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
  import { APP_VERSION } from '@/utils/constants/env';
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
    if (isLoggingIn.value) return;

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
    <div class="login-page__card">
      <div class="login-page__brand">
        <span
          class="login-page__logo"
          aria-hidden="true"
          >T</span
        >
        <span class="login-page__product">Tutti</span>
      </div>
      <h1 class="login-page__title">{{ t('signInTo') }}</h1>
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
            :placeholder="t('usernamePlaceholder')"
            :input-props="{ autocomplete: 'username', name: 'username' }"
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
            :placeholder="t('passwordPlaceholder')"
            :input-props="{
              autocomplete: 'current-password',
              name: 'password',
            }"
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
          size="large"
          block
          :loading="isLoggingIn"
          :disabled="isLoggingIn"
        >
          {{ t('signIn') }}
        </NButton>
      </NForm>
    </div>

    <footer class="login-page__footer">Tutti · v{{ APP_VERSION }}</footer>
  </main>
</template>

<style lang="scss" scoped>
  .login-page {
    box-sizing: border-box;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    padding: 24px 16px;
    background:
      radial-gradient(
        ellipse at 50% 0%,
        color-mix(in srgb, var(--primary-color) 12%, transparent),
        transparent 65%
      ),
      var(--body-color);

    &__card {
      box-sizing: border-box;
      width: 100%;
      max-width: 400px;
      padding: 32px;
      border: 1px solid var(--border-color);
      border-radius: 12px;
      background-color: var(--card-color);
      box-shadow: 0 8px 32px
        color-mix(in srgb, var(--text-color-1) 10%, transparent);

      @media screen and (max-width: 480px) {
        padding: 24px 20px;
      }
    }

    &__brand {
      display: flex;
      align-items: center;
      gap: 10px;
      margin-bottom: 24px;
    }

    &__logo {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 36px;
      height: 36px;
      border-radius: 10px;
      font-size: 20px;
      font-weight: 700;
      color: var(--card-color);
      background-color: var(--primary-color);
    }

    &__product {
      font-size: 22px;
      font-weight: 600;
      color: var(--text-color-1);
    }

    &__title {
      margin: 0 0 4px;
      font-size: 22px;
      line-height: 30px;
      font-weight: 600;
      color: var(--text-color-1);
    }

    &__subtitle {
      margin: 0 0 20px;
      font-size: 14px;
      color: var(--text-color-3);
    }

    &__error {
      margin-bottom: 16px;
    }

    &__hint {
      margin: 4px 0 20px 24px;
      font-size: 12px;
      line-height: 16px;
      color: var(--text-color-3);
    }

    &__footer {
      font-size: 12px;
      color: var(--text-color-3);
    }
  }
</style>
