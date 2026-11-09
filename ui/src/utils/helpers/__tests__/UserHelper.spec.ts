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

import { describe, expect, it } from 'vitest';
import { UserHelper } from '../UserHelper';

describe('UserHelper.validateName', () => {
  const existing = ['user1'];

  it('accepts valid names', () => {
    ['a', 'demo3', 'a.b_c-d', 'A'.repeat(64), '9lives'].forEach((name) => {
      expect(UserHelper.validateName(name, existing)).toBeNull();
    });
  });

  it('rejects invalid names', () => {
    expect(UserHelper.validateName('', existing)).toBe('required');
    expect(UserHelper.validateName('a'.repeat(65), existing)).toBe('tooLong');
    expect(UserHelper.validateName('_x', existing)).toBe('invalidStart');
    expect(UserHelper.validateName('-x', existing)).toBe('invalidStart');
    expect(UserHelper.validateName('a b', existing)).toBe('invalidChars');
    expect(UserHelper.validateName('a@b', existing)).toBe('invalidChars');
    expect(UserHelper.validateName('user1', existing)).toBe('exists');
  });
});

describe('UserHelper credentials step', () => {
  const rows = () => UserHelper.newCredentialRows(['main', 'follower']);

  it('copies the first row to all rows when "same credentials" is on', () => {
    const r = rows();

    r[0]!.accessKeyId = 'AK';
    r[0]!.secretAccessKey = 'SK';
    r[1]!.result = { ok: true, latencyMs: 3 };

    const out = UserHelper.applySameCredentials(r);

    expect(out[1]!.accessKeyId).toBe('AK');
    expect(out[1]!.secretAccessKey).toBe('SK');
    // values changed: old verification no longer applies
    expect(out[1]!.result).toBeNull();
  });

  it('keeps a result when the values did not change', () => {
    const r = rows();

    r.forEach((row) => {
      row.accessKeyId = 'AK';
      row.secretAccessKey = 'SK';
    });
    r[1]!.result = { ok: true };
    expect(UserHelper.applySameCredentials(r)[1]!.result).toEqual({ ok: true });
  });

  it('requires the main storage and all-or-nothing rows', () => {
    const r = rows();

    expect(UserHelper.validateCredentials(r, 'main')).toBe('mainRequired');
    r[0]!.accessKeyId = 'AK';
    r[0]!.secretAccessKey = 'SK';
    expect(UserHelper.validateCredentials(r, 'main')).toBeNull();
    expect(UserHelper.skippedStorages(r)).toEqual(['follower']);
    r[1]!.accessKeyId = 'AK';
    expect(UserHelper.validateCredentials(r, 'main')).toBe('incompleteRow');
    expect(UserHelper.toCredentials(r)).toEqual([
      { storage: 'main', accessKeyId: 'AK', secretAccessKey: 'SK' },
    ]);
  });

  it('maps verification results and backend failure messages to rows', () => {
    const results = UserHelper.mapVerification([
      { storage: 'main', ok: true, latencyMs: 14 },
      { storage: 'follower', ok: false, error: 'SignatureDoesNotMatch' },
    ]);
    const out = UserHelper.applyResults(rows(), results);

    expect(out[0]!.result).toEqual({
      ok: true,
      latencyMs: 14,
      error: undefined,
    });
    expect(out[1]!.result?.ok).toBe(false);
    expect(out[1]!.result?.error).toBe('SignatureDoesNotMatch');

    expect(
      UserHelper.parseVerifyFailures(
        'credential verification failed: storage "follower": SignatureDoesNotMatch; storage "main": timeout',
      ),
    ).toEqual({ follower: 'SignatureDoesNotMatch', main: 'timeout' });
  });

  it('parses bucket patterns', () => {
    expect(
      UserHelper.parseBucketPatterns(' ci-*, reports ,ci-*').patterns,
    ).toEqual(['ci-*', 'reports']);
    expect(UserHelper.parseBucketPatterns('a b, c').invalid).toEqual(['a b']);
    expect(UserHelper.parseBucketPatterns('').patterns).toEqual([]);
  });
});

describe('UserHelper.parseDeleteBlockers', () => {
  it('offers force when only active keys block', () => {
    const res = UserHelper.parseDeleteBlockers(
      'FailedPrecondition: s3 user "u" cannot be deleted: 1 active proxy access key(s) exist: disable them or set force=true to delete them',
    );

    expect(res.reasons).toHaveLength(1);
    expect(res.reasons[0]!.kind).toBe('activeKeys');
    expect(res.canForce).toBe(true);
  });

  it('does not offer force when anything else blocks', () => {
    const res = UserHelper.parseDeleteBlockers(
      'FailedPrecondition: s3 user "u" cannot be deleted: 1 replication(s) exist (main->f1): delete them first; 2 active proxy access key(s) exist: disable them or set force=true to delete them',
    );

    expect(res.reasons.map((r) => r.kind)).toEqual([
      'replications',
      'activeKeys',
    ]);
    expect(res.canForce).toBe(false);
  });

  it('classifies config credentials and handles empty input', () => {
    expect(
      UserHelper.parseDeleteBlockers(
        'cannot be deleted: credentials are defined in the worker config on storage(s) main and must be removed there',
      ).reasons[0]!.kind,
    ).toBe('config');
    expect(UserHelper.parseDeleteBlockers(null)).toEqual({
      reasons: [],
      canForce: false,
    });
  });
});

describe('UserHelper.isIncomplete', () => {
  it('uses complete and missing storages', () => {
    expect(UserHelper.isIncomplete({ name: 'a', complete: true })).toBe(false);
    expect(UserHelper.isIncomplete({ name: 'a', complete: false })).toBe(true);
    expect(
      UserHelper.isIncomplete({
        name: 'a',
        storages: [{ storage: 'x', configured: false }],
      }),
    ).toBe(true);
  });
});
