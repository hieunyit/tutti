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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TuttiCoreService } from '../TuttiCoreService';
import { apiClient } from '@/http';

vi.mock('@/http', () => ({ apiClient: { get: vi.fn() } }));

const get = vi.mocked(apiClient.get);

describe('getProxyCredentials', () => {
  beforeEach(() => {
    sessionStorage.clear();
    get.mockReset();
  });

  it('asks once, remembers a 501 for the session and then skips the call', async () => {
    get.mockRejectedValue({ response: { status: 501 } });

    await expect(TuttiCoreService.getProxyCredentials()).rejects.toBeTruthy();
    expect(get).toHaveBeenCalledTimes(1);

    expect(await TuttiCoreService.getProxyCredentials()).toEqual({
      address: '',
      credentials: [],
    });
    expect(get).toHaveBeenCalledTimes(1);
  });

  it('does not remember other failures', async () => {
    get.mockRejectedValue({ response: { status: 500 } });

    await expect(TuttiCoreService.getProxyCredentials()).rejects.toBeTruthy();
    await expect(TuttiCoreService.getProxyCredentials()).rejects.toBeTruthy();
    expect(get).toHaveBeenCalledTimes(2);
  });
});
