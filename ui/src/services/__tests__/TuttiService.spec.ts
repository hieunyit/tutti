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
import { TuttiService } from '../TuttiService';
import { apiClient } from '@/http';

vi.mock('@/http', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}));

const mocked = apiClient as unknown as Record<
  'get' | 'post' | 'put',
  ReturnType<typeof vi.fn>
>;

describe('TuttiService', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('gets proxies', async () => {
    mocked.get.mockResolvedValue({ data: { proxies: [{ appId: 'a' }] } });
    expect(await TuttiService.getProxies()).toEqual({
      proxies: [{ appId: 'a' }],
    });
    expect(mocked.get).toHaveBeenCalledWith(
      expect.stringContaining('/tutti/proxies'),
    );
  });

  it('lists policies and events', async () => {
    mocked.get.mockResolvedValue({ data: {} });
    await TuttiService.getFailoverPolicies();
    await TuttiService.getFailoverEvents();
    expect(mocked.get.mock.calls[0]?.[0]).toContain('/tutti/failover/policies');
    expect(mocked.get.mock.calls[1]?.[0]).toContain('/tutti/failover/events');
  });

  it('posts a policy', async () => {
    mocked.post.mockResolvedValue({ data: {} });
    await TuttiService.setFailoverPolicy({ source: 's', targets: ['t'] });
    expect(mocked.post).toHaveBeenCalledWith(
      expect.stringContaining('/tutti/failover/policies'),
      { source: 's', targets: ['t'] },
    );
  });

  it('deletes a policy and acknowledges an event via PUT', async () => {
    mocked.put.mockResolvedValue({ data: {} });
    await TuttiService.deleteFailoverPolicy('s');
    await TuttiService.deleteFailoverEvent({ user: 'u', bucket: '' });
    expect(mocked.put.mock.calls[0]).toEqual([
      expect.stringContaining('/tutti/failover/policies/delete'),
      { source: 's' },
    ]);
    expect(mocked.put.mock.calls[1]).toEqual([
      expect.stringContaining('/tutti/failover/events/delete'),
      { user: 'u', bucket: '' },
    ]);
  });

  it('lists proxy accounts and manages access policies', async () => {
    mocked.get.mockResolvedValue({ data: { accounts: [] } });
    await TuttiService.getProxyAccounts();
    expect(mocked.get.mock.calls[0]?.[0]).toContain('/tutti/proxy-accounts');

    mocked.post.mockResolvedValue({ data: {} });
    await TuttiService.setAccessPolicy({ user: 'u', permission: 'read-only' });
    expect(mocked.post).toHaveBeenCalledWith(
      expect.stringContaining('/tutti/access-policies'),
      { user: 'u', permission: 'read-only' },
    );

    mocked.put.mockResolvedValue({ data: {} });
    await TuttiService.deleteAccessPolicy('u');
    expect(mocked.put).toHaveBeenCalledWith(
      expect.stringContaining('/tutti/access-policies/delete'),
      { user: 'u' },
    );
  });

  it('manages dynamic proxy accounts', async () => {
    mocked.post.mockResolvedValue({
      data: { accessKeyId: 'A', secretAccessKey: 'S' },
    });
    expect(await TuttiService.createProxyAccount('u', 'd')).toEqual({
      accessKeyId: 'A',
      secretAccessKey: 'S',
    });
    expect(mocked.post).toHaveBeenCalledWith(
      expect.stringContaining('/tutti/proxy-accounts'),
      { user: 'u', description: 'd' },
    );

    mocked.put.mockResolvedValue({ data: {} });
    await TuttiService.rotateProxyAccountSecret('A', '0s');
    await TuttiService.setProxyAccountStatus('A', 'disabled');
    await TuttiService.deleteProxyAccount('A');
    expect(mocked.put.mock.calls).toEqual([
      [
        expect.stringContaining('/tutti/proxy-accounts/rotate'),
        { accessKeyId: 'A', gracePeriod: '0s' },
      ],
      [
        expect.stringContaining('/tutti/proxy-accounts/status'),
        { accessKeyId: 'A', status: 'disabled' },
      ],
      [
        expect.stringContaining('/tutti/proxy-accounts/delete'),
        { accessKeyId: 'A' },
      ],
    ]);
  });
});
