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

import axios, { type AxiosAdapter, type AxiosInstance } from 'axios';
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import { installAuthInterceptor } from '../authInterceptor';

function makeClient(status: number): {
  client: AxiosInstance;
  seen: { url?: string; auth?: string }[];
} {
  const seen: { url?: string; auth?: string }[] = [];
  const adapter: AxiosAdapter = async (config) => {
    seen.push({
      url: config.url,
      auth: config.headers.get('Authorization') as string | undefined,
    });

    const response = {
      data: {},
      status,
      statusText: '',
      headers: {},
      config,
    };

    if (status >= 400) {
      throw new axios.AxiosError(
        'fail',
        String(status),
        config,
        null,
        response,
      );
    }

    return response;
  };

  return { client: axios.create({ adapter }), seen };
}

describe('auth interceptor', () => {
  let onUnauthorized: Mock<() => void>;

  beforeEach(() => {
    onUnauthorized = vi.fn<() => void>();
  });

  it('adds the Bearer header when a token exists', async () => {
    const { client, seen } = makeClient(200);

    installAuthInterceptor(client, {
      getToken: () => 'abc',
      onUnauthorized,
    });
    await client.get('/api/tutti/proxies');
    expect(seen[0]?.auth).toBe('Bearer abc');
  });

  it('sends no header without a token', async () => {
    const { client, seen } = makeClient(200);

    installAuthInterceptor(client, { getToken: () => null, onUnauthorized });
    await client.get('/api/tutti/proxies');
    expect(seen[0]?.auth).toBeUndefined();
  });

  it('reports 401 from API calls and still rejects', async () => {
    const { client } = makeClient(401);

    installAuthInterceptor(client, { getToken: () => 't', onUnauthorized });
    await expect(client.get('/api/replication')).rejects.toBeDefined();
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it('does not treat 401 on /auth/* as session expiry (no redirect loop)', async () => {
    const { client } = makeClient(401);

    installAuthInterceptor(client, { getToken: () => null, onUnauthorized });
    await expect(client.post('/api/auth/login')).rejects.toBeDefined();
    await expect(client.get('/api/auth/me')).rejects.toBeDefined();
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('ignores other error statuses such as 403', async () => {
    const { client } = makeClient(403);

    installAuthInterceptor(client, { getToken: () => 't', onUnauthorized });
    await expect(client.put('/api/replication/pause')).rejects.toBeDefined();
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('can be uninstalled', async () => {
    const { client, seen } = makeClient(200);
    const eject = installAuthInterceptor(client, {
      getToken: () => 'abc',
      onUnauthorized,
    });

    eject();
    await client.get('/x');
    expect(seen[0]?.auth).toBeUndefined();
  });
});
