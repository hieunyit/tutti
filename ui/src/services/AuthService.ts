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
  AuthLoginRequest,
  AuthLoginResponse,
  AuthMeResponse,
} from '@/utils/types/auth';

export abstract class AuthService {
  static async login(payload: AuthLoginRequest): Promise<AuthLoginResponse> {
    const { data } = await apiClient.post<AuthLoginResponse>(
      ApiHelper.getChorusAPIUrl('/auth/login'),
      payload,
    );

    return data;
  }

  static async logout(): Promise<void> {
    await apiClient.post(ApiHelper.getChorusAPIUrl('/auth/logout'));
  }

  static async me(): Promise<AuthMeResponse> {
    const { data } = await apiClient.get<AuthMeResponse>(
      ApiHelper.getChorusAPIUrl('/auth/me'),
    );

    return data;
  }

  static async changePassword(
    currentPassword: string,
    newPassword: string,
  ): Promise<void> {
    await apiClient.post(ApiHelper.getChorusAPIUrl('/auth/password'), {
      currentPassword,
      newPassword,
    });
  }
}
