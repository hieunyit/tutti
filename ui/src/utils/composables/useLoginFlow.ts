/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { storeToRefs } from 'pinia';
import { computed, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import i18nAuth from '@/components/auth/i18nAuth';
import { useAuthStore } from '@/stores/authStore';
import { AuthHelper } from '@/utils/helpers/AuthHelper';
import { RouteName } from '@/utils/types/router';

/**
 * Logic of the login page: validation, sign-in, error text and the
 * unreachable-server state with retry. Never leaves the page blank: a failed
 * /auth/me is surfaced through `initFailed`.
 */
export function useLoginFlow() {
  const { t } = useI18n({ messages: i18nAuth });
  const route = useRoute();
  const router = useRouter();
  const auth = useAuthStore();
  const { isLoggingIn, loginError, retryAfterSeconds, remainingAttempts } =
    storeToRefs(auth);

  const form = reactive({ username: '', password: '', keepSignedIn: false });
  const showErrors = ref(false);
  const isRetrying = ref(false);

  const initFailed = computed(() => auth.initError);

  const usernameError = computed(() =>
    showErrors.value && form.username.trim() === ''
      ? t('usernameRequired')
      : '',
  );
  const passwordError = computed(() =>
    showErrors.value && form.password === '' ? t('passwordRequired') : '',
  );

  const errorMessage = computed(() => {
    if (loginError.value === 'invalidCredentials') {
      return remainingAttempts.value === null
        ? t('errorInvalidCredentials')
        : t('errorInvalidCredentialsAttempts', { n: remainingAttempts.value });
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

    return loginError.value ? t('errorUnknown') : '';
  });

  function goAfterLogin() {
    return router.replace(
      AuthHelper.getSafeRedirect(route.query.redirect) ?? {
        name: RouteName.CHORUS_HOME,
      },
    );
  }

  async function submit(): Promise<void> {
    if (isLoggingIn.value) return;

    showErrors.value = true;

    if (form.username.trim() === '' || form.password === '') return;

    if (
      await auth.login(form.username.trim(), form.password, form.keepSignedIn)
    ) {
      await goAfterLogin();
    }
  }

  async function retryInit(): Promise<void> {
    if (isRetrying.value) return;

    isRetrying.value = true;

    try {
      await auth.init(true);

      if (auth.isAuthenticated) await goAfterLogin();
    } finally {
      isRetrying.value = false;
    }
  }

  return {
    t,
    form,
    submit,
    errorMessage,
    remainingAttempts,
    initFailed,
    retryInit,
    isRetrying,
    isLoggingIn,
    usernameError,
    passwordError,
  };
}
