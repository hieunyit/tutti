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

import { apiClient } from '@/http';
import { ApiHelper } from '@/utils/helpers/ApiHelper';
import type {
  TuttiAccessPolicy,
  TuttiAccessPolicyListResponse,
  TuttiProxyAccountListResponse,
  TuttiFailoverEventId,
  TuttiFailoverEventListResponse,
  TuttiFailoverPolicy,
  TuttiFailoverPolicyListResponse,
  TuttiProxyStatusListResponse,
} from '@/utils/types/tutti';

export abstract class TuttiService {
  static async getProxies(): Promise<TuttiProxyStatusListResponse> {
    const { data } = await apiClient.get<TuttiProxyStatusListResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/proxies'),
    );

    return data;
  }

  static async getFailoverPolicies(): Promise<TuttiFailoverPolicyListResponse> {
    const { data } = await apiClient.get<TuttiFailoverPolicyListResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/failover/policies'),
    );

    return data;
  }

  static async setFailoverPolicy(policy: TuttiFailoverPolicy): Promise<void> {
    await apiClient.post(
      ApiHelper.getChorusAPIUrl('/tutti/failover/policies'),
      policy,
    );
  }

  static async deleteFailoverPolicy(source: string): Promise<void> {
    await apiClient.put(
      ApiHelper.getChorusAPIUrl('/tutti/failover/policies/delete'),
      { source },
    );
  }

  static async getFailoverEvents(): Promise<TuttiFailoverEventListResponse> {
    const { data } = await apiClient.get<TuttiFailoverEventListResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/failover/events'),
    );

    return data;
  }

  static async deleteFailoverEvent(id: TuttiFailoverEventId): Promise<void> {
    await apiClient.put(
      ApiHelper.getChorusAPIUrl('/tutti/failover/events/delete'),
      id,
    );
  }

  static async getProxyAccounts(): Promise<TuttiProxyAccountListResponse> {
    const { data } = await apiClient.get<TuttiProxyAccountListResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/proxy-accounts'),
    );

    return data;
  }

  static async getAccessPolicies(): Promise<TuttiAccessPolicyListResponse> {
    const { data } = await apiClient.get<TuttiAccessPolicyListResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/access-policies'),
    );

    return data;
  }

  static async setAccessPolicy(policy: TuttiAccessPolicy): Promise<void> {
    await apiClient.post(
      ApiHelper.getChorusAPIUrl('/tutti/access-policies'),
      policy,
    );
  }

  static async deleteAccessPolicy(user: string): Promise<void> {
    await apiClient.put(
      ApiHelper.getChorusAPIUrl('/tutti/access-policies/delete'),
      { user },
    );
  }
}
