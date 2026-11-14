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
  ProtectionGroup,
  ProtectionGroupCreateRequest,
  ProtectionGroupListResponse,
  ProtectionGroupStatus,
  ProtectionWriterClearRequest,
  ProtectionWriters,
} from '@/utils/types/protection';

const BASE = '/tutti/protection-groups';

export abstract class ProtectionGroupService {
  static async list(): Promise<ProtectionGroupListResponse> {
    const { data } = await apiClient.get<ProtectionGroupListResponse>(
      ApiHelper.getTuttiAPIUrl(BASE),
    );

    return data;
  }

  static async get(id: string): Promise<ProtectionGroupStatus> {
    const { data } = await apiClient.get<ProtectionGroupStatus>(
      ApiHelper.getTuttiAPIUrl(`${BASE}/${encodeURIComponent(id)}`),
    );

    return data;
  }

  static async create(
    request: ProtectionGroupCreateRequest,
  ): Promise<ProtectionGroup> {
    const { data } = await apiClient.post<ProtectionGroup>(
      ApiHelper.getTuttiAPIUrl(BASE),
      request,
    );

    return data;
  }

  static async update(group: ProtectionGroup): Promise<ProtectionGroup> {
    const { data } = await apiClient.put<ProtectionGroup>(
      ApiHelper.getTuttiAPIUrl(BASE),
      group,
    );

    return data;
  }

  /** Detaches management only; routing, replication and objects stay. */
  static async remove(id: string): Promise<void> {
    await apiClient.post(ApiHelper.getTuttiAPIUrl(`${BASE}/delete`), { id });
  }

  static async switchover(id: string, target: string): Promise<void> {
    await apiClient.post(ApiHelper.getTuttiAPIUrl(`${BASE}/switchover`), {
      id,
      target,
    });
  }

  static async abortSwitch(id: string): Promise<void> {
    await apiClient.post(ApiHelper.getTuttiAPIUrl(`${BASE}/abort-switch`), {
      id,
    });
  }

  static async getAutomaticFreeze(): Promise<boolean> {
    const { data } = await apiClient.get<{ frozen?: boolean }>(
      ApiHelper.getTuttiAPIUrl(`${BASE}/automatic-freeze/status`),
    );

    return data.frozen === true;
  }

  static async setAutomaticFreeze(frozen: boolean): Promise<void> {
    await apiClient.put(ApiHelper.getTuttiAPIUrl(`${BASE}/automatic-freeze`), {
      frozen,
    });
  }

  static async listWriters(id: string): Promise<ProtectionWriters> {
    const { data } = await apiClient.get<ProtectionWriters>(
      ApiHelper.getTuttiAPIUrl(`${BASE}/${encodeURIComponent(id)}/writers`),
    );

    return data;
  }

  static async clearWriter(
    request: ProtectionWriterClearRequest,
  ): Promise<void> {
    await apiClient.post(
      ApiHelper.getTuttiAPIUrl(`${BASE}/clear-writer`),
      request,
    );
  }
}
