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
  AuditLogEntry,
  AuditLogList,
  ListAuditLogParams,
} from '@/utils/types/audit';

const BASE = '/tutti/audit-log';

interface RawAuditLogEntry {
  id?: string | number;
  at?: string;
  occurredAt?: string;
  occurred_at?: string;
  actor?: string;
  requestId?: string;
  request_id?: string;
  action?: string;
  family?: string;
  entityKey?: string;
  entity_key?: string;
  beforeJson?: string;
  before_json?: string;
  afterJson?: string;
  after_json?: string;
}

interface RawAuditLogList {
  backend?: string;
  readOnly?: boolean;
  read_only?: boolean;
  entries?: RawAuditLogEntry[];
}

export abstract class AuditLogService {
  static async list(params: ListAuditLogParams = {}): Promise<AuditLogList> {
    const query: Record<string, string | number> = {};

    if (params.family?.trim()) {
      query.family = params.family.trim();
    }

    if (params.entityKey?.trim()) {
      query.entityKey = params.entityKey.trim();
    }

    if (params.actor?.trim()) {
      query.actor = params.actor.trim();
    }

    if (params.action?.trim()) {
      query.action = params.action.trim();
    }

    if (
      params.beforeId !== undefined &&
      params.beforeId !== null &&
      String(params.beforeId).trim() !== '' &&
      String(params.beforeId).trim() !== '0'
    ) {
      query.beforeId = params.beforeId;
    }

    if (params.limit !== undefined && params.limit > 0) {
      query.limit = Math.min(params.limit, 500);
    }

    const { data } = await apiClient.get<RawAuditLogList>(
      ApiHelper.getTuttiAPIUrl(BASE),
      { params: query },
    );

    const entries: AuditLogEntry[] = (data?.entries ?? []).map((entry) => ({
      id: entry.id ?? '',
      at: entry.at ?? entry.occurredAt ?? entry.occurred_at ?? '',
      actor: entry.actor ?? '',
      requestId: entry.requestId ?? entry.request_id ?? '',
      action: entry.action ?? '',
      family: entry.family ?? '',
      entityKey: entry.entityKey ?? entry.entity_key ?? '',
      beforeJson: entry.beforeJson ?? entry.before_json,
      afterJson: entry.afterJson ?? entry.after_json,
    }));

    return {
      backend: data?.backend || 'redis',
      readOnly: Boolean(data?.readOnly ?? data?.read_only),
      entries,
    };
  }
}
