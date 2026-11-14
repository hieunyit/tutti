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
  import { NButton, NCheckbox, NInput } from 'naive-ui';
  import { onMounted, ref } from 'vue';
  import { Callout, ErrorBlock } from '@/components/kit';
  import SessionEndedNotice from '@/components/auth/SessionEndedNotice.vue';
  import { useLoginFlow } from '@/utils/composables/useLoginFlow';
  import { APP_VERSION } from '@/utils/constants/env';
  import { GITHUB_BLOB_URL } from '@/utils/helpers/DocsHelper';

  // The in-app docs sit behind sign-in, so the hint points at the repository copy of the guide.
  const firstRunDocsUrl = `${GITHUB_BLOB_URL}docs/tutti/operations.md#first-login-and-password-recovery`;

  const {
    t,
    form,
    submit,
    errorMessage,
    initFailed,
    retryInit,
    isLoggingIn,
    usernameError,
    passwordError,
  } = useLoginFlow();

  const usernameInput = ref<InstanceType<typeof NInput> | null>(null);

  onMounted(() => usernameInput.value?.focus());
</script>

<template>
  <main
    class="login-page"
    :aria-label="t('loginTitle')"
  >
    <div class="login-page__card">
      <div class="login-page__logo-row">
        <span
          class="login-page__logo"
          aria-hidden="true"
          >T</span
        >
        <span class="login-page__product">Tutti</span>
      </div>

      <ErrorBlock
        v-if="initFailed"
        :title="t('connectionUnavailable')"
        description=""
        :retry-label="t('retryConnection')"
        @retry="retryInit"
      />
      <form
        v-else
        class="login-page__form"
        novalidate
        @submit.prevent="submit"
      >
        <div class="login-page__heading">
          <h1 class="login-page__title">{{ t('loginTitle') }}</h1>
          <p class="login-page__subtitle">{{ t('loginDescription') }}</p>
        </div>

        <SessionEndedNotice />

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
          <NInput
            ref="usernameInput"
            v-model:value="form.username"
            :input-props="{
              id: 'login-username',
              name: 'username',
              autocomplete: 'username',
              autocapitalize: 'none',
              spellcheck: false,
            }"
            :placeholder="t('usernamePlaceholder')"
            :disabled="isLoggingIn"
            :status="usernameError ? 'error' : undefined"
          />
          <span
            v-if="usernameError"
            class="login-page__error"
            role="alert"
            >{{ usernameError }}</span
          >
        </div>

        <div class="login-page__field">
          <label
            class="login-page__label"
            for="login-password"
            >{{ t('password') }}</label
          >
          <NInput
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            :input-props="{
              id: 'login-password',
              name: 'password',
              autocomplete: 'current-password',
            }"
            :placeholder="t('passwordPlaceholder')"
            :disabled="isLoggingIn"
            :status="passwordError ? 'error' : undefined"
          />
          <span
            v-if="passwordError"
            class="login-page__error"
            role="alert"
            >{{ passwordError }}</span
          >
        </div>

        <NCheckbox
          v-model:checked="form.keepSignedIn"
          :disabled="isLoggingIn"
        >
          {{ t('keepSignedIn') }}
        </NCheckbox>
        <span class="login-page__hint">{{ t('keepSignedInHint') }}</span>

        <NButton
          type="primary"
          attr-type="submit"
          size="large"
          block
          :loading="isLoggingIn"
        >
          {{ isLoggingIn ? t('signingIn') : t('signIn') }}
        </NButton>

        <p class="login-page__help">{{ t('loginHelp') }}</p>
        <p class="login-page__help">
          <a
            class="login-page__link"
            data-test="first-run-hint"
            :href="firstRunDocsUrl"
            target="_blank"
            rel="noopener noreferrer"
            >{{ t('firstRunHint') }}</a
          >
        </p>
      </form>

      <span class="login-page__version">v{{ APP_VERSION }}</span>
    </div>
  </main>
</template>

<style lang="scss" scoped>
  .login-page {
    display: flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    min-height: 100vh;
    padding: var(--tt-space-6) var(--tt-space-4);
    background-color: var(--tt-bg);
    color: var(--tt-text);

    &__card {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-5);
      box-sizing: border-box;
      width: 400px;
      max-width: 100%;
      padding: var(--tt-space-7);
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-modal);

      @media screen and (max-width: 639px) {
        padding: var(--tt-space-5) var(--tt-space-4);
      }
    }

    &__logo-row {
      display: flex;
      align-items: center;
      gap: var(--tt-space-2);
    }

    &__logo {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      border-radius: var(--tt-radius-control);
      font-weight: var(--tt-weight-strong);
      color: var(--tt-on-accent);
      background-color: var(--tt-accent-solid);
    }

    &__product {
      font-size: var(--tt-text-lg);
      font-weight: var(--tt-weight-strong);
    }

    &__form {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-4);
    }

    &__heading {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
    }

    &__title {
      margin: 0;
      font-size: var(--tt-text-xl);
      font-weight: var(--tt-weight-strong);
    }

    &__subtitle {
      margin: 0;
      font-size: var(--tt-text-md);
      color: var(--tt-text-2);
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
    }

    &__label {
      font-size: var(--tt-text-md);
      font-weight: var(--tt-weight-medium);
    }

    &__error {
      font-size: var(--tt-text-xs);
      color: var(--tt-danger);
    }

    &__hint {
      margin-top: calc(var(--tt-space-2) * -1);
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }

    &__help {
      margin: 0;
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
      text-align: center;
    }

    &__link {
      color: inherit;
      text-decoration: underline;
    }

    &__version {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
      text-align: center;
    }
  }
</style>
