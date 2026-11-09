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
  import { storeToRefs } from 'pinia';
  import { computed, onMounted, ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import i18nAuth from '@/components/auth/i18nAuth';
  import Callout from '@/components/ui/Callout.vue';
  import { useAuthStore } from '@/stores/authStore';
  import { APP_VERSION } from '@/utils/constants/env';
  import { AuthHelper } from '@/utils/helpers/AuthHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nAuth });
  const route = useRoute();
  const router = useRouter();
  const auth = useAuthStore();
  const { isLoggingIn, loginError, retryAfterSeconds, remainingAttempts } =
    storeToRefs(auth);

  const username = ref('');
  const password = ref('');
  const keepSignedIn = ref(false);
  const showPassword = ref(false);
  const showErrors = ref(false);
  const usernameInput = ref<HTMLInputElement | null>(null);

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
      return remainingAttempts.value === null
        ? t('errorInvalidCredentials')
        : t('errorInvalidCredentialsAttempts', {
            n: remainingAttempts.value,
          });
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

  onMounted(() => usernameInput.value?.focus());
</script>

<template>
  <div class="login-page">
    <section
      class="login-page__brand"
      :aria-label="t('aboutTutti')"
    >
      <div class="login-page__logo-row">
        <span
          class="login-page__logo"
          aria-hidden="true"
          >T</span
        >
        <span class="login-page__product">Tutti</span>
      </div>

      <div class="login-page__pitch">
        <h2 class="login-page__headline">{{ t('brandHeadline') }}</h2>
        <p class="login-page__lead">{{ t('brandLead') }}</p>
        <ul class="login-page__points">
          <li
            v-for="key in ['brandPoint1', 'brandPoint2', 'brandPoint3']"
            :key="key"
          >
            <span
              class="login-page__check"
              aria-hidden="true"
              >✓</span
            >
            {{ t(key) }}
          </li>
        </ul>
      </div>

      <span class="login-page__version"
        >v{{ APP_VERSION }} · {{ t('brandBasedOn') }}</span
      >
    </section>

    <main
      class="login-page__side"
      :aria-label="t('loginTitle')"
    >
      <form
        class="login-page__form"
        novalidate
        @submit.prevent="submit"
      >
        <div class="login-page__heading">
          <h1 class="login-page__title">{{ t('loginTitle') }}</h1>
          <p class="login-page__subtitle">{{ t('loginDescription') }}</p>
        </div>

        <Callout
          v-if="errorMessage"
          tone="danger"
        >
          {{ errorMessage }}
        </Callout>

        <div class="login-page__field">
          <label
            class="login-page__label"
            for="login-username"
            >{{ t('username') }}</label
          >
          <input
            id="login-username"
            ref="usernameInput"
            v-model="username"
            class="login-page__input"
            :class="{ 'login-page__input--error': usernameError }"
            type="text"
            name="username"
            autocomplete="username"
            autocapitalize="none"
            spellcheck="false"
            :placeholder="t('usernamePlaceholder')"
            :disabled="isLoggingIn"
            :aria-invalid="usernameError ? 'true' : undefined"
            :aria-describedby="
              usernameError ? 'login-username-error' : undefined
            "
          />
          <span
            v-if="usernameError"
            id="login-username-error"
            class="login-page__error"
            >{{ usernameError }}</span
          >
        </div>

        <div class="login-page__field">
          <label
            class="login-page__label"
            for="login-password"
            >{{ t('password') }}</label
          >
          <span
            class="login-page__password"
            :class="{ 'login-page__password--error': passwordError }"
          >
            <input
              id="login-password"
              v-model="password"
              class="login-page__password-input"
              :type="showPassword ? 'text' : 'password'"
              name="password"
              autocomplete="current-password"
              :placeholder="t('passwordPlaceholder')"
              :disabled="isLoggingIn"
              :aria-invalid="passwordError ? 'true' : undefined"
              :aria-describedby="
                passwordError ? 'login-password-error' : undefined
              "
            />
            <button
              type="button"
              class="login-page__reveal"
              :aria-label="showPassword ? t('hidePassword') : t('showPassword')"
              :aria-pressed="showPassword"
              @click="showPassword = !showPassword"
            >
              {{ showPassword ? t('hide') : t('show') }}
            </button>
          </span>
          <span
            v-if="passwordError"
            id="login-password-error"
            class="login-page__error"
            >{{ passwordError }}</span
          >
        </div>

        <label class="login-page__keep">
          <input
            v-model="keepSignedIn"
            type="checkbox"
            :disabled="isLoggingIn"
          />
          <span class="login-page__keep-text">
            {{ t('keepSignedIn') }}
            <span class="login-page__hint">{{ t('keepSignedInHint') }}</span>
          </span>
        </label>

        <button
          type="submit"
          class="login-page__submit"
          :disabled="isLoggingIn"
          :aria-busy="isLoggingIn"
        >
          <span
            v-if="isLoggingIn"
            class="login-page__spinner"
            aria-hidden="true"
          />
          {{ isLoggingIn ? t('signingIn') : t('signIn') }}
        </button>

        <p class="login-page__help">{{ t('loginHelp') }}</p>
      </form>
    </main>
  </div>
</template>

<style lang="scss" scoped>
  .login-page {
    display: flex;
    flex-wrap: wrap;
    min-height: 100vh;
    background-color: var(--tt-surface);
    color: var(--tt-text);

    &__brand {
      display: flex;
      flex: 1 1 520px;
      flex-direction: column;
      gap: 32px;
      box-sizing: border-box;
      padding: 48px 56px;
      background-color: var(--tt-nav-bg);
      color: #cbd5e1;

      @media screen and (max-width: 639px) {
        padding: 24px 20px;
        gap: 20px;
      }
    }

    &__logo-row {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    &__logo {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 36px;
      height: 36px;
      border-radius: 9px;
      font-weight: 700;
      color: #ffffff;
      background-color: #3b6cf6;
    }

    &__product {
      font-size: 18px;
      font-weight: 600;
      color: #f8fafc;
    }

    &__pitch {
      display: flex;
      flex-direction: column;
      gap: 16px;
      max-width: 460px;
      margin-top: auto;

      @media screen and (max-width: 639px) {
        margin-top: 0;
      }
    }

    &__headline {
      margin: 0;
      font-size: 32px;
      line-height: 40px;
      font-weight: 700;
      color: #f8fafc;

      @media screen and (max-width: 639px) {
        font-size: 24px;
        line-height: 32px;
      }
    }

    &__lead {
      margin: 0;
      font-size: 15px;
      line-height: 1.6;

      @media screen and (max-width: 639px) {
        display: none;
      }
    }

    &__points {
      display: flex;
      flex-direction: column;
      gap: 10px;
      margin: 0;
      padding: 0;
      list-style: none;
      font-size: 14px;

      li {
        display: flex;
        gap: 10px;
      }

      @media screen and (max-width: 639px) {
        display: none;
      }
    }

    &__check {
      color: #6e95ff;
    }

    &__version {
      font-size: 12px;
      color: #94a3b8;

      @media screen and (max-width: 639px) {
        display: none;
      }
    }

    &__side {
      display: flex;
      flex: 1 1 480px;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      padding: 48px 24px;
      background-color: var(--tt-bg-app);

      @media screen and (max-width: 639px) {
        padding: 24px 16px;
        align-items: flex-start;
      }
    }

    &__form {
      display: flex;
      flex-direction: column;
      gap: 18px;
      box-sizing: border-box;
      width: 400px;
      max-width: 100%;
      padding: 32px;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: 16px;
      box-shadow: var(--tt-shadow);

      @media screen and (max-width: 639px) {
        padding: 24px 20px;
      }
    }

    &__heading {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    &__title {
      margin: 0;
      font-size: 22px;
      line-height: 30px;
      font-weight: 700;
    }

    &__subtitle {
      margin: 0;
      font-size: 14px;
      color: var(--tt-text-2);
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    &__label {
      font-size: 14px;
      font-weight: 500;
    }

    &__input,
    &__password {
      box-sizing: border-box;
      height: 40px;
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-surface);
      font: inherit;
      font-size: 14px;
      color: var(--tt-text);
    }

    &__input {
      width: 100%;
      padding: 0 12px;
    }

    &__input:focus-visible {
      outline: 2px solid var(--tt-accent);
      outline-offset: 0;
    }

    &__password {
      display: flex;
      align-items: center;
      padding-right: 6px;

      &:focus-within {
        outline: 2px solid var(--tt-accent);
      }
    }

    &__password-input {
      flex: 1;
      min-width: 0;
      height: 38px;
      padding: 0 12px;
      border: 0;
      border-radius: var(--tt-radius-control);
      background: transparent;
      font: inherit;
      font-size: 14px;
      color: var(--tt-text);

      &:focus-visible {
        outline: 0;
      }
    }

    &__input--error,
    &__password--error {
      border-color: var(--tt-danger-border);
      box-shadow: 0 0 0 3px var(--tt-danger-bg);
    }

    &__input::placeholder,
    &__password-input::placeholder {
      color: var(--tt-text-3);
    }

    &__reveal {
      height: 32px;
      padding: 0 8px;
      border: 0;
      border-radius: 6px;
      background: transparent;
      font: inherit;
      font-size: 13px;
      color: var(--tt-text-3);
      cursor: pointer;

      &:hover {
        color: var(--tt-text);
      }
    }

    &__error {
      font-size: 12px;
      color: var(--tt-danger);
    }

    &__keep {
      display: flex;
      align-items: flex-start;
      gap: 10px;
      font-size: 14px;

      input {
        margin: 3px 0 0;
        accent-color: var(--tt-accent-solid);
      }
    }

    &__keep-text {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    &__hint {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__submit {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      height: 44px;
      border: 0;
      border-radius: var(--tt-radius-control);
      background-color: var(--tt-accent-solid);
      font: inherit;
      font-size: 15px;
      font-weight: 600;
      color: var(--tt-on-accent);
      cursor: pointer;

      &:hover:not(:disabled) {
        background-color: var(--tt-accent-solid-hover);
      }

      &:disabled {
        cursor: progress;
        opacity: 0.85;
      }
    }

    &__spinner {
      width: 16px;
      height: 16px;
      border: 2px solid currentcolor;
      border-right-color: transparent;
      border-radius: 50%;
      animation: login-spin 0.7s linear infinite;
    }

    &__help {
      margin: 0;
      font-size: 12px;
      color: var(--tt-text-3);
      text-align: center;
    }
  }

  @keyframes login-spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .login-page__spinner {
      animation-duration: 2s;
    }
  }
</style>
