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

import axios, { type AxiosInstance } from 'axios';
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';

const AUTH_PATH = '/auth/';

interface AuthInterceptorDeps {
  getToken: () => string | null;
  /** Clears the session and navigates to the login page. */
  onUnauthorized: () => void;
  /** HTTP 403 PASSWORD_CHANGE_REQUIRED: keep the session, ask for a new password. */
  onPasswordChangeRequired?: () => void;
}

function isAuthRequest(url: string | undefined): boolean {
  return (url ?? '').includes(AUTH_PATH);
}

/**
 * Adds the Bearer header to every request while a token exists and reacts to 401s.
 * Requests to /auth/* never trigger the 401 redirect (login failure is a normal
 * outcome there, and /auth/me is handled by the auth store), which also prevents
 * redirect loops on the login page.
 */
export function installAuthInterceptor(
  client: AxiosInstance,
  { getToken, onUnauthorized, onPasswordChangeRequired }: AuthInterceptorDeps,
): () => void {
  const requestId = client.interceptors.request.use((config) => {
    const token = getToken();

    if (token && !config.headers.has('Authorization')) {
      config.headers.set('Authorization', `Bearer ${token}`);
    }

    return config;
  });

  const responseId = client.interceptors.response.use(
    (response) => response,
    (error: unknown) => {
      if (
        axios.isAxiosError(error) &&
        error.response?.status === 401 &&
        !isAuthRequest(error.config?.url)
      ) {
        onUnauthorized();
      } else if (ErrorHelper.isPasswordChangeRequired(error)) {
        onPasswordChangeRequired?.();
      }

      return Promise.reject(error);
    },
  );

  return () => {
    client.interceptors.request.eject(requestId);
    client.interceptors.response.eject(responseId);
  };
}
