// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

import { apiClient } from '@/http';
import { ApiHelper } from '@/utils/helpers/ApiHelper';
import type {
  LifecycleRule,
  CreateLifecycleRuleRequest,
  ListLifecycleRulesRequest,
  ListLifecycleRulesResponse,
  UpdateLifecycleRuleRequest,
  LifecycleDryRunRequest,
  LifecycleDryRunResponse,
  ListLifecycleAuditLogsResponse,
} from '@/utils/types/lifecycle';

const url = (suffix = '') => ApiHelper.getTuttiAPIUrl(`/lifecycle${suffix}`);

export abstract class LifecycleService {
  static async listRules(params?: ListLifecycleRulesRequest): Promise<LifecycleRule[]> {
    const { data } = await apiClient.get<ListLifecycleRulesResponse>(url('/rules'), { params });

    return data.rules || [];
  }

  static async getRule(id: string): Promise<LifecycleRule> {
    const { data } = await apiClient.get<LifecycleRule>(url(`/rules/${encodeURIComponent(id)}`));

    return data;
  }

  static async createRule(req: CreateLifecycleRuleRequest): Promise<LifecycleRule> {
    const { data } = await apiClient.post<LifecycleRule>(url('/rules'), req);

    return data;
  }

  static async updateRule(req: UpdateLifecycleRuleRequest): Promise<LifecycleRule> {
    const { data } = await apiClient.put<LifecycleRule>(url('/rules'), req);

    return data;
  }

  static async deleteRule(id: string): Promise<void> {
    await apiClient.put(url('/rules/delete'), { id });
  }

  static async enableRule(id: string): Promise<void> {
    await apiClient.put(url('/rules/enable'), { id });
  }

  static async disableRule(id: string): Promise<void> {
    await apiClient.put(url('/rules/disable'), { id });
  }

  static async dryRun(req: LifecycleDryRunRequest): Promise<LifecycleDryRunResponse> {
    const { data } = await apiClient.post<LifecycleDryRunResponse>(url('/rules/dry-run'), req);

    return data;
  }

  static async listAuditLogs(ruleId: string, limit = 50, offset = 0): Promise<ListLifecycleAuditLogsResponse> {
    const { data } = await apiClient.get<ListLifecycleAuditLogsResponse>(url('/audit-logs'), {
      params: { ruleId, limit, offset },
    });

    return data;
  }
}
