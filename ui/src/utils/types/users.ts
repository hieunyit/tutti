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

/** S3 users (GET /tutti/users) and console users (/tutti/console-users). */

export type S3UserCredentialSource = 'config' | 'dynamic';

export interface S3UserStorage {
  storage: string;
  configured: boolean;
  source?: S3UserCredentialSource;
  accessKeyId?: string;
}

export interface S3UserKeyCounts {
  static?: number;
  dynamicActive?: number;
  dynamicDisabled?: number;
}

export interface S3UserPolicy {
  hasPolicy?: boolean;
  permission?: string;
  buckets?: string[];
}

export interface S3User {
  name: string;
  storages?: S3UserStorage[];
  /** credentials exist on every storage */
  complete?: boolean;
  accessKeys?: S3UserKeyCounts;
  policy?: S3UserPolicy;
}

export interface S3UserListResponse {
  users?: S3User[];
  mainStorage?: string;
  storages?: string[];
  dynamicCredentialsEnabled?: boolean;
}

export interface S3UserCredentialInput {
  storage: string;
  accessKeyId: string;
  secretAccessKey: string;
}

export interface S3UserPolicyInput {
  user: string;
  permission: string;
  buckets: string[];
  description?: string;
}

export interface S3UserCreateRequest {
  name: string;
  credentials: S3UserCredentialInput[];
  verify: boolean;
  createAccessKey: boolean;
  description?: string;
  policy?: S3UserPolicyInput;
}

export interface S3UserVerification {
  storage: string;
  ok: boolean;
  latencyMs?: number;
  error?: string;
}

export interface S3UserCreateResponse {
  user?: S3User;
  accessKey?: { accessKeyId: string; secretAccessKey?: string };
  verification?: S3UserVerification[];
}

export interface S3UserTestRequest {
  name: string;
  storage: string;
  accessKeyId?: string;
  secretAccessKey?: string;
}

export interface S3UserTestResponse {
  ok?: boolean;
  latencyMs?: number;
  error?: string;
}

export type ConsoleUserSource = 'config' | 'managed';

export interface ConsoleUser {
  username: string;
  role: 'admin' | 'viewer';
  source: ConsoleUserSource;
  disabled?: boolean;
  mustChangePassword?: boolean;
  createdAt?: string | null;
  updatedAt?: string | null;
  lastLoginAt?: string | null;
  passwordChangedAt?: string | null;
}

export interface ConsoleUserListResponse {
  users?: ConsoleUser[];
}

export interface ConsoleUserCreateRequest {
  username: string;
  role: 'admin' | 'viewer';
  password?: string;
  generatePassword?: boolean;
}

export interface ConsoleUserCreateResponse {
  user?: ConsoleUser;
  generatedPassword?: string;
}

export interface ConsoleUserPasswordRequest {
  username: string;
  password?: string;
  generatePassword?: boolean;
}

export interface ConsoleUserPasswordResponse {
  generatedPassword?: string;
}
