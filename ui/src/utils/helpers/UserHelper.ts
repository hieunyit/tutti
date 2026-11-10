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

import type { RuntimeStorage } from '@/utils/types/runtimeStorage';
import type {
  S3User,
  S3UserCredentialMode,
  S3UserStorage,
  S3UserCredentialInput,
  S3UserVerification,
} from '@/utils/types/users';

export type UserNameError =
  | 'required'
  | 'tooLong'
  | 'invalidStart'
  | 'invalidChars'
  | 'exists';

export const USER_NAME_MAX = 64;

export type DeleteBlockerKind =
  | 'config'
  | 'replications'
  | 'routings'
  | 'failover'
  | 'activeKeys'
  | 'other';

export interface DeleteBlocker {
  kind: DeleteBlockerKind;
  text: string;
}

export interface DeleteBlockers {
  reasons: DeleteBlocker[];
  /** true when "force" resolves every blocker (only active keys are left) */
  canForce: boolean;
}

/** One row of the credentials step of the create wizard. */
export interface CredentialRow {
  storage: string;
  accessKeyId: string;
  secretAccessKey: string;
  result: CredentialCheck | null;
}

export interface CredentialCheck {
  ok: boolean;
  latencyMs?: number;
  error?: string;
}

export interface CredentialPart {
  kind: S3UserCredentialMode;
  storages: string[];
}

export type PolicyStepError = 'bucketsRequired' | 'invalidBuckets' | null;

export type CredentialsStepError = 'mainRequired' | 'incompleteRow' | null;

export abstract class UserHelper {
  /** Mode of one storage entry; older backends only send `configured`. */
  static modeOf(entry: S3UserStorage): S3UserCredentialMode {
    if (
      entry.mode === 'override' ||
      entry.mode === 'connection-account' ||
      entry.mode === 'missing'
    )
      return entry.mode;

    return entry.configured ? 'override' : 'missing';
  }

  /**
   * The "Credentials" column: storages grouped by mode, in the order
   * connection-account, override, missing.
   */
  static credentialParts(user: S3User): CredentialPart[] {
    const order: S3UserCredentialMode[] = [
      'connection-account',
      'override',
      'missing',
    ];

    return order
      .map((kind) => ({
        kind,
        storages: (user.storages ?? [])
          .filter((entry) => UserHelper.modeOf(entry) === kind)
          .map((entry) => entry.storage),
      }))
      .filter((part) => part.storages.length > 0);
  }

  /** Text of one part; `translate` is the i18n function: (key, storages). */
  static credentialText(
    part: CredentialPart,
    translate: (key: string, params: { storages: string }) => string,
  ): string {
    const key = {
      'connection-account': 'modeConnection',
      override: 'modeOverride',
      missing: 'modeMissing',
    }[part.kind];

    return translate(key, { storages: part.storages.join(', ') });
  }

  /** The proxy would sign for this user with the storage connection account. */
  static usesConnectionAccount(
    storage: Pick<RuntimeStorage, 'connectionAccount'>,
  ) {
    return storage.connectionAccount?.configured === true;
  }

  /**
   * The access policy is mandatory when any storage resolves through its
   * connection account (no override entered for it). Unknown storages count
   * as shared: the backend decides, the UI stays on the safe side.
   */
  static policyRequired(
    storages: Pick<RuntimeStorage, 'name' | 'connectionAccount'>[] | null,
    overridden: string[] = [],
  ): boolean {
    if (storages === null) return true;

    return storages.some(
      (storage) =>
        UserHelper.usesConnectionAccount(storage) &&
        !overridden.includes(storage.name),
    );
  }

  /** Storages without a connection account need an override for the user. */
  static legacyStorages(
    storages: Pick<RuntimeStorage, 'name' | 'connectionAccount'>[],
  ): string[] {
    return storages
      .filter((storage) => !UserHelper.usesConnectionAccount(storage))
      .map((storage) => storage.name);
  }

  /** Validates the bucket patterns; with a shared account they are mandatory. */
  static validatePolicy(input: string, required: boolean): PolicyStepError {
    const parsed = UserHelper.parseBucketPatterns(input);

    if (parsed.invalid.length) return 'invalidBuckets';

    return required && parsed.patterns.length === 0 ? 'bucketsRequired' : null;
  }

  /** Legacy storages (no connection account) need a complete override. */
  static validateOverrides(
    rows: CredentialRow[],
    legacy: string[],
  ): CredentialsStepError {
    const partial = rows.some(
      (row) =>
        !UserHelper.isRowFilled(row) &&
        (row.accessKeyId.trim() !== '' || row.secretAccessKey !== ''),
    );

    if (partial) return 'incompleteRow';

    return legacy.some((name) => {
      const row = rows.find((item) => item.storage === name);

      return !row || !UserHelper.isRowFilled(row);
    })
      ? 'mainRequired'
      : null;
  }

  /** Names of the storages the user has no credentials on. */
  static missingStorages(user: S3User): string[] {
    return (user.storages ?? [])
      .filter((entry) => UserHelper.modeOf(entry) === 'missing')
      .map((entry) => entry.storage);
  }

  static isIncomplete(user: S3User): boolean {
    return (
      user.complete === false || UserHelper.missingStorages(user).length > 0
    );
  }

  /**
   * Rules of the backend: 1-64 characters, letters, digits, ".", "_", "-",
   * starting with a letter or digit; names of existing users are rejected.
   */
  static validateName(name: string, existing: string[]): UserNameError | null {
    if (name === '') return 'required';

    if (name.length > USER_NAME_MAX) return 'tooLong';

    if (!/^[A-Za-z0-9]/.test(name)) return 'invalidStart';

    if (!/^[A-Za-z0-9._-]+$/.test(name)) return 'invalidChars';

    if (existing.includes(name)) return 'exists';

    return null;
  }

  /** Splits the FailedPrecondition message of a refused deletion into reasons. */
  static parseDeleteBlockers(
    message: string | null | undefined,
  ): DeleteBlockers {
    const text = message ?? '';
    const marker = 'cannot be deleted:';
    const index = text.indexOf(marker);
    const body = (index >= 0 ? text.slice(index + marker.length) : text).trim();
    const reasons = body
      .split(';')
      .map((part) => part.trim())
      .filter((part) => part !== '')
      .map<DeleteBlocker>((part) => ({
        kind: UserHelper.blockerKind(part),
        text: part,
      }));

    return {
      reasons,
      canForce:
        reasons.length > 0 && reasons.every((r) => r.kind === 'activeKeys'),
    };
  }

  private static blockerKind(text: string): DeleteBlockerKind {
    const lower = text.toLowerCase();

    if (lower.includes('active proxy access key')) return 'activeKeys';

    if (lower.includes('defined in the worker config')) return 'config';

    if (lower.includes('replication')) return 'replications';

    if (lower.includes('routing')) return 'routings';

    if (lower.includes('failover')) return 'failover';

    return 'other';
  }

  /** Per-storage errors out of "credential verification failed: storage "x": msg; ...". */
  static parseVerifyFailures(
    message: string | null | undefined,
  ): Record<string, string> {
    const result: Record<string, string> = {};
    const text = message ?? '';
    const regex = /storage "([^"]+)":\s*([^;]*)/g;
    let match = regex.exec(text);

    while (match) {
      result[match[1]!] = (match[2] ?? '').trim();
      match = regex.exec(text);
    }

    return result;
  }

  /** Verification list of a create response keyed by storage. */
  static mapVerification(
    verification: S3UserVerification[] | undefined,
  ): Record<string, CredentialCheck> {
    const result: Record<string, CredentialCheck> = {};

    (verification ?? []).forEach((item) => {
      result[item.storage] = {
        ok: item.ok,
        latencyMs: item.latencyMs,
        error: item.error,
      };
    });

    return result;
  }

  static newCredentialRows(storages: string[]): CredentialRow[] {
    return storages.map((storage) => ({
      storage,
      accessKeyId: '',
      secretAccessKey: '',
      result: null,
    }));
  }

  /**
   * "Use the same credentials on all storages": every row takes the values of
   * the first row (the main storage). Verification results are dropped for
   * rows whose values changed.
   */
  static applySameCredentials(rows: CredentialRow[]): CredentialRow[] {
    const [first] = rows;

    if (!first) return rows;

    return rows.map((row, index) => {
      if (index === 0) return row;

      const changed =
        row.accessKeyId !== first.accessKeyId ||
        row.secretAccessKey !== first.secretAccessKey;

      return {
        ...row,
        accessKeyId: first.accessKeyId,
        secretAccessKey: first.secretAccessKey,
        result: changed ? null : row.result,
      };
    });
  }

  static isRowFilled(row: CredentialRow): boolean {
    return row.accessKeyId.trim() !== '' && row.secretAccessKey !== '';
  }

  /** Main storage is required, other rows are all-or-nothing. */
  static validateCredentials(
    rows: CredentialRow[],
    mainStorage: string,
  ): CredentialsStepError {
    const main = rows.find((row) => row.storage === mainStorage);

    if (!main || !UserHelper.isRowFilled(main)) return 'mainRequired';

    const partial = rows.some(
      (row) =>
        !UserHelper.isRowFilled(row) &&
        (row.accessKeyId.trim() !== '' || row.secretAccessKey !== ''),
    );

    return partial ? 'incompleteRow' : null;
  }

  static toCredentials(rows: CredentialRow[]): S3UserCredentialInput[] {
    return rows.filter(UserHelper.isRowFilled).map((row) => ({
      storage: row.storage,
      accessKeyId: row.accessKeyId.trim(),
      secretAccessKey: row.secretAccessKey,
    }));
  }

  /** Storages that are left out (no credentials entered). */
  static skippedStorages(rows: CredentialRow[]): string[] {
    return rows
      .filter((row) => !UserHelper.isRowFilled(row))
      .map((row) => row.storage);
  }

  /** Applies per-storage results (from verification) to the rows. */
  static applyResults(
    rows: CredentialRow[],
    results: Record<string, CredentialCheck>,
  ): CredentialRow[] {
    return rows.map((row) => ({
      ...row,
      result: results[row.storage] ?? row.result,
    }));
  }

  /** Comma separated glob patterns; whitespace inside a pattern is invalid. */
  static parseBucketPatterns(input: string): {
    patterns: string[];
    invalid: string[];
  } {
    const patterns = input
      .split(',')
      .map((part) => part.trim())
      .filter((part) => part !== '');
    const invalid = patterns.filter((pattern) => /\s/.test(pattern));

    return { patterns: [...new Set(patterns)], invalid };
  }
}
