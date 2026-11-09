/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { apiClient } from '@/http';
import { ApiHelper } from '@/utils/helpers/ApiHelper';
import type {
  RuntimeStorage,
  RuntimeStorageCredential,
  RuntimeStorageList,
  RuntimeStorageMutation,
  RuntimeStorageTestResult,
} from '@/utils/types/runtimeStorage';

const url = (suffix = '') =>
  ApiHelper.getChorusAPIUrl(`/tutti/storages${suffix}`);

export abstract class RuntimeStorageService {
  static async list(signal?: AbortSignal): Promise<RuntimeStorageList> {
    return (await apiClient.get<RuntimeStorageList>(url(), { signal })).data;
  }

  static async create(input: RuntimeStorageMutation): Promise<RuntimeStorage> {
    return (await apiClient.post<RuntimeStorage>(url(), input)).data;
  }

  static async update(input: RuntimeStorageMutation): Promise<RuntimeStorage> {
    return (await apiClient.put<RuntimeStorage>(url(), input)).data;
  }

  static async remove(name: string): Promise<void> {
    await apiClient.put(url('/delete'), { name });
  }

  static async test(
    storage: RuntimeStorage,
    credential?: RuntimeStorageCredential,
  ): Promise<RuntimeStorageTestResult> {
    return (
      await apiClient.post<RuntimeStorageTestResult>(url('/test'), {
        storage,
        credential,
      })
    ).data;
  }
}
