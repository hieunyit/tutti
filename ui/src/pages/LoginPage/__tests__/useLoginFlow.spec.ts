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

import { AxiosError, AxiosHeaders } from 'axios';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h } from 'vue';
import { createI18n } from 'vue-i18n';
import { useLoginFlow } from '@/utils/composables/useLoginFlow';
import { AuthService } from '@/services/AuthService';

const replace = vi.fn();

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace }),
}));
vi.mock('@/services/AuthService', () => ({
  AuthService: { login: vi.fn(), logout: vi.fn(), me: vi.fn() },
}));

type MockFn = ReturnType<typeof vi.fn>;

const svc = AuthService as unknown as Record<'login' | 'logout' | 'me', MockFn>;

function httpError(status: number, data: unknown = {}) {
  return new AxiosError('x', String(status), undefined, undefined, {
    status,
    statusText: '',
    data,
    headers: {},
    config: { headers: new AxiosHeaders() },
  });
}

function setup() {
  const pinia = createPinia();

  setActivePinia(pinia);

  let flow!: ReturnType<typeof useLoginFlow>;
  const app = createApp(
    defineComponent({
      setup() {
        flow = useLoginFlow();

        return () => h('div');
      },
    }),
  );

  app.use(pinia);
  app.use(createI18n({ legacy: false, locale: 'en', messages: {} }));
  app.mount(document.createElement('div'));

  return flow;
}

beforeEach(() => {
  window.sessionStorage.clear();
  window.localStorage.clear();
  vi.resetAllMocks();
});

describe('useLoginFlow', () => {
  it('exposes attempts left and a message after a 401', async () => {
    svc.login.mockRejectedValue(httpError(401, { remainingAttempts: 8 }));
    const flow = setup();

    flow.form.username = 'admin';
    flow.form.password = 'wrong';
    await flow.submit();

    expect(flow.remainingAttempts.value).toBe(8);
    expect(flow.errorMessage.value).not.toBe('');
    expect(replace).not.toHaveBeenCalled();
  });

  it('does not call the API when a field is empty', async () => {
    const flow = setup();

    await flow.submit();

    expect(svc.login).not.toHaveBeenCalled();
    expect(flow.usernameError.value).not.toBe('');
  });

  it('flags an unreachable server and retries /auth/me', async () => {
    svc.me.mockRejectedValue(new Error('network'));
    const flow = setup();

    await flow.retryInit();
    expect(flow.initFailed.value).toBe(true);
    expect(svc.me).toHaveBeenCalledTimes(1);

    svc.me.mockResolvedValue({ authEnabled: true });
    await flow.retryInit();
    expect(svc.me).toHaveBeenCalledTimes(2);
    expect(flow.initFailed.value).toBe(false);
  });
});
