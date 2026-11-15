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

import axios from 'axios';

/*
 * Identifier for the standard Google RPC ErrorInfo detail type.
 * Since the API uses Protobuf, rich error metadata (like the specific 'reason' code)
 * is encapsulated in this message type within the error details array, see
 * https://cloud.google.com/apis/design/errors#error_info.
 */
const GOOGLE_RPC_ERROR_TYPE = 'type.googleapis.com/google.rpc.ErrorInfo';

export const PASSWORD_CHANGE_REQUIRED_REASON = 'PASSWORD_CHANGE_REQUIRED';

/** the backend prefixes ErrorInfo reasons with the gRPC code name ("FailedPrecondition: ...") */
const GRPC_CODE_PREFIX =
  /^(?:InvalidArg(?:ument)?|FailedPrecondition|AlreadyExists|NotFound|PermissionDenied|Unauthenticated|Internal|Unavailable|ResourceExhausted|Aborted|Unimplemented|NotImplemented|DeadlineExceeded|Cancell?ed|OutOfRange|Unknown|DataLoss):\s+/;

export abstract class ErrorHelper {
  /** a backend reason as a user may read it: without the gRPC code name in front */
  static stripCode<T extends string | null | undefined>(text: T): T {
    return (
      typeof text === 'string' ? text.replace(GRPC_CODE_PREFIX, '') : text
    ) as T;
  }

  /**
   * HTTP 403 {"code":7,"message":"password change required","details":[{"reason":
   * "PASSWORD_CHANGE_REQUIRED"}]}: the signed-in user must set a new password first.
   */
  static isPasswordChangeRequired(error: unknown): boolean {
    if (!axios.isAxiosError(error) || error.response?.status !== 403) {
      return false;
    }

    const data = error.response.data as
      | {
          code?: number;
          message?: string;
          details?: { reason?: string }[];
        }
      | undefined;

    if (
      Array.isArray(data?.details) &&
      data.details.some((d) => d?.reason === PASSWORD_CHANGE_REQUIRED_REASON)
    ) {
      return true;
    }

    return data?.code === 7 && data.message === 'password change required';
  }

  static getReason(error: unknown): string | null {
    if (!axios.isAxiosError(error)) return null;

    const details = error.response?.data.details;

    if (!Array.isArray(details)) return `${error.name} - ${error.message}`;

    const errorInfo = details.find((d) => d['@type'] === GOOGLE_RPC_ERROR_TYPE);

    return ErrorHelper.stripCode(errorInfo?.reason);
  }

  static getStatusCode(error: unknown): number | null {
    if (!axios.isAxiosError(error)) return null;

    return error.status ?? null;
  }

  static getValidationErrorMessage(
    field: {
      $errors: { $message: unknown }[];
    },
    unknownErrorMessage = '',
  ): string {
    const errors = field.$errors;

    if (!errors || errors.length === 0) return '';

    return String(errors[0]?.$message) || unknownErrorMessage;
  }
}
