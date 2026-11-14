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
import { DedupeKey, dedupe, invalidate } from '@/utils/helpers/dedupe';
import type {
  TuttiAccessPolicy,
  TuttiAccessPolicyListResponse,
  TuttiDynamicKeyStatus,
  TuttiProxyAccountListResponse,
  TuttiProxyAccountSecret,
  TuttiFailoverEventId,
  TuttiFailoverEventListResponse,
  TuttiFailoverPolicy,
  TuttiFailoverPolicyListResponse,
  TuttiInt64,
  TuttiProxyFleetConfig,
  TuttiProxySettingsOverride,
  TuttiProxyStatusListResponse,
  TuttiStatsHistory,
  TuttiStatsRange,
  TuttiSystemSettings,
  TuttiUpdateSystemSettingsRequest,
  TuttiWorkerStatsResponse,
} from '@/utils/types/tutti';
import type {
  ConsoleUserCreateRequest,
  ConsoleUserCreateResponse,
  ConsoleUserListResponse,
  ConsoleUserPasswordRequest,
  ConsoleUserPasswordResponse,
  S3UserCreateRequest,
  S3UserCreateResponse,
  S3UserListResponse,
  S3UserTestRequest,
  S3UserTestResponse,
} from '@/utils/types/users';

export abstract class TuttiService {
  static async getProxies(
    signal?: AbortSignal,
  ): Promise<TuttiProxyStatusListResponse> {
    const fetchProxies = async () => {
      const { data } = await apiClient.get<TuttiProxyStatusListResponse>(
        ApiHelper.getChorusAPIUrl('/tutti/proxies'),
        { signal },
      );

      return data;
    };

    // an abortable request must not be shared: aborting it would fail the others
    return signal ? fetchProxies() : dedupe(DedupeKey.PROXIES, fetchProxies);
  }

  static async getProxyFleetConfig(
    signal?: AbortSignal,
  ): Promise<TuttiProxyFleetConfig> {
    const { data } = await apiClient.get<TuttiProxyFleetConfig>(
      ApiHelper.getChorusAPIUrl('/tutti/proxy-config'),
      { signal },
    );

    return data;
  }

  static async setProxyFleetConfig(payload: {
    defaults?: TuttiProxySettingsOverride;
    groups?: Record<string, TuttiProxySettingsOverride>;
    /** the version the caller loaded: a different stored version is rejected (FailedPrecondition) */
    expectedVersion?: TuttiInt64;
  }): Promise<TuttiProxyFleetConfig> {
    try {
      const { data } = await apiClient.put<TuttiProxyFleetConfig>(
        ApiHelper.getChorusAPIUrl('/tutti/proxy-config'),
        payload,
      );

      return data;
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async getStatsHistory(
    range: TuttiStatsRange,
    signal?: AbortSignal,
  ): Promise<TuttiStatsHistory> {
    const { data } = await apiClient.get<TuttiStatsHistory>(
      ApiHelper.getChorusAPIUrl('/tutti/stats/history'),
      { params: { range }, signal },
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
    try {
      await apiClient.post(
        ApiHelper.getChorusAPIUrl('/tutti/failover/policies'),
        policy,
      );
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async deleteFailoverPolicy(source: string): Promise<void> {
    try {
      await apiClient.put(
        ApiHelper.getChorusAPIUrl('/tutti/failover/policies/delete'),
        { source },
      );
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
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
    try {
      await apiClient.post(
        ApiHelper.getChorusAPIUrl('/tutti/access-policies'),
        policy,
      );
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async deleteAccessPolicy(user: string): Promise<void> {
    try {
      await apiClient.put(
        ApiHelper.getChorusAPIUrl('/tutti/access-policies/delete'),
        { user },
      );
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async createProxyAccount(
    user: string,
    description: string,
  ): Promise<TuttiProxyAccountSecret> {
    try {
      const { data } = await apiClient.post<TuttiProxyAccountSecret>(
        ApiHelper.getChorusAPIUrl('/tutti/proxy-accounts'),
        { user, description },
      );

      return data;
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async rotateProxyAccountSecret(
    accessKeyId: string,
    gracePeriod: string,
  ): Promise<TuttiProxyAccountSecret> {
    try {
      const { data } = await apiClient.put<TuttiProxyAccountSecret>(
        ApiHelper.getChorusAPIUrl('/tutti/proxy-accounts/rotate'),
        { accessKeyId, gracePeriod },
      );

      return data;
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async setProxyAccountStatus(
    accessKeyId: string,
    status: TuttiDynamicKeyStatus,
  ): Promise<void> {
    try {
      await apiClient.put(
        ApiHelper.getChorusAPIUrl('/tutti/proxy-accounts/status'),
        { accessKeyId, status },
      );
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async deleteProxyAccount(accessKeyId: string): Promise<void> {
    try {
      await apiClient.put(
        ApiHelper.getChorusAPIUrl('/tutti/proxy-accounts/delete'),
        { accessKeyId },
      );
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async getUsers(signal?: AbortSignal): Promise<S3UserListResponse> {
    const fetchUsers = async () => {
      const { data } = await apiClient.get<S3UserListResponse>(
        ApiHelper.getChorusAPIUrl('/tutti/users'),
        { signal },
      );

      return data;
    };

    return signal ? fetchUsers() : dedupe(DedupeKey.USERS, fetchUsers);
  }

  static async createUser(
    payload: S3UserCreateRequest,
  ): Promise<S3UserCreateResponse> {
    try {
      const { data } = await apiClient.post<S3UserCreateResponse>(
        ApiHelper.getChorusAPIUrl('/tutti/users'),
        payload,
      );

      return data;
    } finally {
      invalidate(DedupeKey.USERS);
    }
  }

  static async updateUserCredentials(payload: {
    name: string;
    storage: string;
    accessKeyId: string;
    secretAccessKey: string;
    verify?: boolean;
  }): Promise<void> {
    try {
      await apiClient.put(
        ApiHelper.getChorusAPIUrl('/tutti/users/credentials'),
        {
          user: payload.name,
          credential: {
            storage: payload.storage,
            accessKeyId: payload.accessKeyId,
            secretAccessKey: payload.secretAccessKey,
          },
          verify: payload.verify,
        },
      );
    } finally {
      invalidate(DedupeKey.USERS);
    }
  }

  /** Removes the per-user override on one storage (falls back to the connection account). */
  static async removeUserCredential(
    name: string,
    storage: string,
  ): Promise<void> {
    try {
      await apiClient.put(
        ApiHelper.getChorusAPIUrl('/tutti/users/credentials/delete'),
        { name, storage },
      );
    } finally {
      invalidate(DedupeKey.USERS);
    }
  }

  static async deleteUser(name: string, force: boolean): Promise<void> {
    try {
      await apiClient.put(ApiHelper.getChorusAPIUrl('/tutti/users/delete'), {
        name,
        force,
      });
    } finally {
      invalidate(DedupeKey.USERS);
    }
  }

  static async testUser(
    payload: S3UserTestRequest,
  ): Promise<S3UserTestResponse> {
    const { data } = await apiClient.post<S3UserTestResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/users/test'),
      {
        user: payload.name,
        storage: payload.storage,
        ...(payload.accessKeyId !== undefined ||
        payload.secretAccessKey !== undefined
          ? {
              credential: {
                storage: payload.storage,
                accessKeyId: payload.accessKeyId,
                secretAccessKey: payload.secretAccessKey,
              },
            }
          : {}),
      },
    );

    return data;
  }

  static async getConsoleUsers(): Promise<ConsoleUserListResponse> {
    const { data } = await apiClient.get<ConsoleUserListResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/console-users'),
    );

    return data;
  }

  static async createConsoleUser(
    payload: ConsoleUserCreateRequest,
  ): Promise<ConsoleUserCreateResponse> {
    const { data } = await apiClient.post<ConsoleUserCreateResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/console-users'),
      payload,
    );

    return data;
  }

  static async updateConsoleUser(payload: {
    username: string;
    role?: 'admin' | 'viewer';
    disabled?: boolean;
  }): Promise<void> {
    await apiClient.put(
      ApiHelper.getChorusAPIUrl('/tutti/console-users'),
      payload,
    );
  }

  static async resetConsoleUserPassword(
    payload: ConsoleUserPasswordRequest,
  ): Promise<ConsoleUserPasswordResponse> {
    const { data } = await apiClient.put<ConsoleUserPasswordResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/console-users/password'),
      payload,
    );

    return data;
  }

  static async deleteConsoleUser(username: string): Promise<void> {
    await apiClient.put(
      ApiHelper.getChorusAPIUrl('/tutti/console-users/delete'),
      { username },
    );
  }

  static async getSystemSettings(
    signal?: AbortSignal,
  ): Promise<TuttiSystemSettings> {
    const { data } = await apiClient.get<TuttiSystemSettings>(
      ApiHelper.getChorusAPIUrl('/tutti/system-settings'),
      { signal },
    );

    return data;
  }

  static async updateSystemSettings(
    payload: TuttiUpdateSystemSettingsRequest,
  ): Promise<TuttiSystemSettings> {
    try {
      const { data } = await apiClient.put<TuttiSystemSettings>(
        ApiHelper.getChorusAPIUrl('/tutti/system-settings'),
        payload,
      );

      return data;
    } finally {
      invalidate(DedupeKey.PROXIES);
    }
  }

  static async getWorkerStats(
    signal?: AbortSignal,
  ): Promise<TuttiWorkerStatsResponse> {
    const { data } = await apiClient.get<TuttiWorkerStatsResponse>(
      ApiHelper.getChorusAPIUrl('/tutti/worker-stats'),
      { signal },
    );

    return data;
  }
}
