/*
 * Copyright © 2026 Clyso GmbH
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
  TuttiAddReplicationsRequest,
  TuttiBucketListRequest,
  TuttiBucketListResponse,
  TuttiCompareBucketRequest,
  TuttiCompareBucketResponse,
  TuttiProxyCredentials,
  TuttiReplicationId,
  TuttiReplicationListResponse,
  DiffReportDeleteRequest,
  DiffReportRestartRequest,
  DiffReportListResponse,
  RoutingPolicyListRequest,
  RoutingPolicyListResponse,
  TuttiStorageListResponse,
  RoutingPolicyEditRequest,
  RoutingPolicyAddRequest,
  UserCredentialSetRequest,
  AddDiffReportRequest,
  DiffReportDetailRequest,
  DiffReportDetailResponse,
  DiffReportEntriesRequest,
  DiffReportEntriesResponse,
} from '@/utils/types/tuttiCore';

export abstract class TuttiCoreService {
  static async getProxyCredentials(): Promise<TuttiProxyCredentials> {
    const { data } = await apiClient.get<TuttiProxyCredentials>(
      ApiHelper.getTuttiAPIUrl('/proxy'),
    );

    return data;
  }

  static async getStorages(): Promise<TuttiStorageListResponse> {
    return dedupe(DedupeKey.STORAGES, async () => {
      const { data } = await apiClient.get<TuttiStorageListResponse>(
        ApiHelper.getTuttiAPIUrl('/storage'),
      );

      return data;
    });
  }

  static async compareBucket(
    payload: TuttiCompareBucketRequest,
  ): Promise<TuttiCompareBucketResponse> {
    const { data } = await apiClient.post<TuttiCompareBucketResponse>(
      ApiHelper.getTuttiAPIUrl('/replication/compare-bucket'),
      payload,
    );

    return data;
  }

  static async getReplications(
    signal?: AbortSignal,
  ): Promise<TuttiReplicationListResponse> {
    const { data } = await apiClient.post<TuttiReplicationListResponse>(
      ApiHelper.getTuttiAPIUrl('/replication'),
      undefined,
      { signal },
    );

    return data;
  }

  static async addReplication(
    payload: TuttiAddReplicationsRequest,
  ): Promise<void> {
    await apiClient.post(ApiHelper.getTuttiAPIUrl('/replication/add'), payload);
  }

  static async deleteReplication(payload: TuttiReplicationId): Promise<void> {
    await apiClient.put(
      ApiHelper.getTuttiAPIUrl('/replication/delete'),
      payload,
    );
  }

  static async pauseBucketReplication(
    payload: TuttiReplicationId,
  ): Promise<void> {
    await apiClient.put(
      ApiHelper.getTuttiAPIUrl('/replication/pause'),
      payload,
    );
  }

  static async resumeBucketReplication(
    payload: TuttiReplicationId,
  ): Promise<void> {
    await apiClient.put(
      ApiHelper.getTuttiAPIUrl('/replication/resume'),
      payload,
    );
  }

  static async getBucketsForReplication(
    payload: TuttiBucketListRequest,
  ): Promise<TuttiBucketListResponse> {
    const { data } = await apiClient.post<TuttiBucketListResponse>(
      ApiHelper.getTuttiAPIUrl('/replication/list-buckets'),
      payload,
    );

    return data;
  }

  static async getRoutingPolicies(
    payload: RoutingPolicyListRequest,
  ): Promise<RoutingPolicyListResponse> {
    const { data } = await apiClient.post<RoutingPolicyListResponse>(
      ApiHelper.getTuttiAPIUrl('/routing'),
      payload,
    );

    return {
      ...data,
      userRoutings: data.userRoutings ?? [],
      bucketRoutings: data.bucketRoutings ?? [],
    };
  }

  static async deleteRoutingPolicy(
    payload: RoutingPolicyEditRequest,
  ): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl('/routing/delete'), payload);
  }

  static async blockRoutingPolicy(
    payload: RoutingPolicyEditRequest,
  ): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl('/routing/block'), payload);
  }

  static async unblockRoutingPolicy(
    payload: RoutingPolicyEditRequest,
  ): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl('/routing/unblock'), payload);
  }

  static async updateRoutingPolicy(
    payload: RoutingPolicyAddRequest,
  ): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl('/routing/update'), payload);
  }

  static async addRoutingPolicy(
    payload: RoutingPolicyAddRequest,
  ): Promise<void> {
    await apiClient.post(ApiHelper.getTuttiAPIUrl('/routing/add'), payload);
  }

  static async setUserCredentials(
    payload: UserCredentialSetRequest,
  ): Promise<void> {
    try {
      await apiClient.post(ApiHelper.getTuttiAPIUrl('/credentials'), payload);
    } finally {
      invalidate(DedupeKey.STORAGES, DedupeKey.USERS);
    }
  }

  static async deleteDiffReport(
    payload: DiffReportDeleteRequest,
  ): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl('/diff/delete'), payload);
  }

  static async restartDiffReport(
    payload: DiffReportRestartRequest,
  ): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl('/diff/restart'), payload);
  }

  static async getDiffReports(): Promise<DiffReportListResponse> {
    const { data } = await apiClient.get<DiffReportListResponse>(
      ApiHelper.getTuttiAPIUrl('/diff/list'),
    );

    return data;
  }

  static async addDiffReport(payload: AddDiffReportRequest): Promise<void> {
    await apiClient.post(ApiHelper.getTuttiAPIUrl('/diff/start'), payload);
  }

  static async getDiffReport(
    payload: DiffReportDetailRequest,
  ): Promise<DiffReportDetailResponse> {
    const { data } = await apiClient.post<DiffReportDetailResponse>(
      ApiHelper.getTuttiAPIUrl('/diff/report'),
      payload,
    );

    return data;
  }

  static async getDiffReportEntries(
    payload: DiffReportEntriesRequest,
  ): Promise<DiffReportEntriesResponse> {
    const { data } = await apiClient.post<DiffReportEntriesResponse>(
      ApiHelper.getTuttiAPIUrl('/diff/report-entries'),
      payload,
    );

    return data;
  }
}
